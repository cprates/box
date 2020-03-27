package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/cprates/box/boxnet"
	"github.com/cprates/box/spec"
	"github.com/cprates/box/system"

	"golang.org/x/sys/unix"
)

// Boxer defines the interface to interact with a Box.
type Boxer interface {
	// Creates a new box with the given mame and spec, storing the state in workdir, using the
	// given io channels to communicate with the outside world
	create(name, workdir string, io ProcessIO, spec *spec.Spec, opts ...BoxOption) (err error)
	// Starts an existing Box returning immediately after the Box is running
	Start() (err error)
	// run creates and starts a new box with name at workdir with the given spec, blocking until
	// the box is terminated.
	run(name, workdir string, io ProcessIO, spec *spec.Spec, opts ...BoxOption) (err error)
}

type cartonBox struct {
	state        state
	childProcess process
	config
	lock sync.Mutex
}

type process struct {
	created bool
	pid     int
	io      ProcessIO
}

// ProcessIO is used to pass to the runtime the communication channels.
type ProcessIO struct {
	In  *os.File
	Out *os.File
	Err *os.File
}

type config struct {
	Name           string
	Hostname       string
	RootFs         string
	Cwd            string
	EntryPoint     string
	EntryPointArgs []string // entryPoint args
	EnvVars        []string
	ExecFifoPath   string
	StateFilePath  string
	NetConfig      *boxnet.NetConf `json:"NetConfig,omitempty"`
}

type openResult struct {
	file *os.File
	err  error
}

var _ Boxer = (*cartonBox)(nil)

func newCartonBox() Boxer {
	return &cartonBox{
		lock: sync.Mutex{},
	}
}

func boxHostname(name, hostname string) string {
	// the hostname in the spec is optional so, if it isn't set, set it to the given name
	if hostname != "" {
		return hostname
	}

	return name
}

func boxCwd(specCwd string) string {
	if specCwd != "" {
		return specCwd
	}

	return "/"
}

// here the workdir is from the box's point of view
func (c *cartonBox) create(
	name, workdir string,
	io ProcessIO,
	spec *spec.Spec,
	opts ...BoxOption,
) (
	err error,
) {
	c.childProcess = process{io: io}
	c.config = config{
		Name:           name,
		Hostname:       boxHostname(name, spec.Hostname),
		RootFs:         spec.Root.Path,
		Cwd:            boxCwd(spec.Process.Cwd),
		EntryPoint:     spec.Process.Args[0],
		EntryPointArgs: append(spec.Process.Args[:0:0], spec.Process.Args...)[1:],
		EnvVars:        spec.Process.Env,
		ExecFifoPath:   filepath.Join(workdir, execFifoFilename),
		StateFilePath:  filepath.Join(workdir, stateFilename),
	}

	for _, opt := range opts {
		opt(c)
	}

	if err = c.createExecFifo(); err != nil {
		err = fmt.Errorf("creating exec fifo: %s", err)
		return
	}

	// this is a much simpler implementation compared to runC so, at this point it just needs
	// to re-execute itself setting all namespaces at once, except user ns.
	err = c.start()
	if err != nil {
		c.deleteExecFifo()
		err = fmt.Errorf("creating container: %s", err)
		return
	}

	return
}

// Run creates and starts a new box with name at workdir with the given spec, blocking until
// the box is terminated.
func (c *cartonBox) run(
	name, workdir string,
	io ProcessIO,
	spec *spec.Spec,
	opts ...BoxOption,
) (
	err error,
) {
	c.childProcess = process{io: io}
	c.config = config{
		Name:           name,
		Hostname:       boxHostname(name, spec.Hostname),
		RootFs:         spec.Root.Path,
		Cwd:            boxCwd(spec.Process.Cwd),
		EntryPoint:     spec.Process.Args[0],
		EntryPointArgs: append(spec.Process.Args[:0:0], spec.Process.Args...)[1:],
		EnvVars:        spec.Process.Env,
		StateFilePath:  filepath.Join(workdir, stateFilename),
	}

	for _, opt := range opts {
		opt(c)
	}

	err = c.start()
	if err != nil {
		return
	}

	<-awaitProcessExit(c.childProcess.pid, make(chan struct{}))

	err = os.RemoveAll(workdir)
	if err != nil {
		err = fmt.Errorf("cleaning up workdir: %s", err)
		return
	}

	return
}

// Start a previously create Box, returning immediately after the box is started.
func (c *cartonBox) Start() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// first check if the waiting child is still alive
	stat, err := system.Stat(c.state.BoxPID)
	if err != nil {
		return fmt.Errorf("box: box is stopped")
	}

	if stat.StartTime != c.state.ProcessStartClockTicks ||
		stat.State == system.Zombie ||
		stat.State == system.Dead {
		return fmt.Errorf("box: box is stopped")
	}

	c.childProcess.pid = c.state.BoxPID

	return c.exec()
}

func (c *cartonBox) start() (err error) {
	cmd := exec.Command("/proc/self/exe", "bootstrap")
	cmd.Stdin = c.childProcess.io.In
	cmd.Stdout = c.childProcess.io.Out
	cmd.Stderr = c.childProcess.io.Err
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET,
		//syscall.CLONE_NEWUSER,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	configRPipe, configWPipe, err := os.Pipe()
	if err != nil {
		err = fmt.Errorf("creating configPipe: %s", err)
		return
	}
	defer configWPipe.Close()
	defer configRPipe.Close()

	cmd.ExtraFiles = append(cmd.ExtraFiles, os.NewFile(configRPipe.Fd(), "configPipe"))
	configFd := stdioFdCount + len(cmd.ExtraFiles) - 1

	cmd.Env = []string{
		"BOX_BOOTSTRAP_CONFIG_FD=" + strconv.Itoa(configFd),
		"BOX_BOOTSTRAP_LOG_FD=" + strconv.Itoa(int(c.childProcess.io.Out.Fd())),
		"BOX_DEBUG=" + os.Getenv("BOX_DEBUG"),
	}

	// send box config
	if err = json.NewEncoder(configWPipe).Encode(&c.config); err != nil {
		err = fmt.Errorf("sending config to child: %s", err)
		return
	}

	if err = c.includeExecFifo(cmd); err != nil {
		err = fmt.Errorf("including fifo fd: %s", err)
		return
	}

	if err = cmd.Start(); err != nil {
		err = fmt.Errorf("starting child: %s", err)
		return
	}

	c.childProcess.pid = cmd.Process.Pid
	c.childProcess.created = true
	c.state = state{
		BoxPID:    c.childProcess.pid,
		Created:   c.childProcess.created,
		BoxConfig: c.config,
	}

	stat, err := system.Stat(cmd.Process.Pid)
	if err == nil {
		c.state.ProcessStartClockTicks = stat.StartTime

		if c.config.NetConfig != nil {
			if err = c.setupNetFromConfig(); err != nil {
				return
			}
		}

		if err = c.saveState(); err == nil {
			return
		}
	}

	// we were unable to save the box's state so, kill the brand new child process
	err = fmt.Errorf("unable to save state: %s", err)

	if e := cmd.Process.Kill(); e != nil {
		e = fmt.Errorf("%s, also failed to kill child process: %s", err, e)
		return
	}

	errC := make(chan error)
	go func() {
		errC <- cmd.Wait()
	}()

	select {
	case e := <-errC:
		if e != nil {
			e = fmt.Errorf("%s, also failed while waiting for child process to die: %s", err, e)
			return
		}
	case <-time.After(500 * time.Millisecond):
		err = fmt.Errorf("%s, also child process didn't return in time after being killed", err)
		return
	}

	// TODO
	//go func() {
	//	// TODO: Not sure but the doc string state that cmd.Wait releases resources, but this
	//	//   will never return because this process will finish before the child
	//	if err = cmd.Wait(); err != nil {
	//		err = fmt.Errorf("box: bootstrapping box instance: %s", err)
	//		return
	//	}
	//}()

	return
}

func (c *cartonBox) exec() error {
	fifoOpen := make(chan struct{})
	select {
	case <-awaitProcessExit(c.childProcess.pid, fifoOpen):
		return errors.New("box process is already dead")
	case result := <-awaitFifoOpen(c.config.ExecFifoPath):
		close(fifoOpen)
		if result.err != nil {
			return result.err
		}
		f := result.file
		defer f.Close()
		if err := readFromExecFifo(f); err != nil {
			return err
		}
		return nil
	}
}

func (c *cartonBox) createExecFifo() error {
	if _, err := os.Stat(c.config.ExecFifoPath); err == nil {
		return fmt.Errorf("exec fifo %s already exists", c.config.ExecFifoPath)
	}
	oldMask := unix.Umask(0000)
	if err := unix.Mkfifo(c.config.ExecFifoPath, 0622); err != nil {
		unix.Umask(oldMask)
		err = fmt.Errorf("unable to create fifo at %q: %s", c.config.ExecFifoPath, err)
		return err
	}
	unix.Umask(oldMask)

	// currently it does not support user namespaces so, uid and gid are always set to 0
	return os.Chown(c.config.ExecFifoPath, 0, 0)
}

func (c *cartonBox) deleteExecFifo() {
	_ = os.Remove(c.config.ExecFifoPath)
}

// includeExecFifo opens the box's execfifo as a pathfd, so that the
// box cannot access the statedir (and the FIFO itself remains
// un-opened). It then adds the FifoFd to the given exec.Cmd as an inherited
// fd, with BOX_FIFO_FD set to its fd number.
func (c *cartonBox) includeExecFifo(cmd *exec.Cmd) error {
	if c.config.ExecFifoPath == "" {
		return nil
	}

	fifoFd, err := unix.Open(c.config.ExecFifoPath, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, os.NewFile(uintptr(fifoFd), c.config.ExecFifoPath))
	cmd.Env = append(cmd.Env, fmt.Sprintf("BOX_FIFO_FD=%d", stdioFdCount+len(cmd.ExtraFiles)-1))
	return nil
}

func (c *cartonBox) setupNetFromConfig() error {
	for _, rawConf := range c.config.NetConfig.Interfaces {
		t, err := boxnet.TypeFromConfig(rawConf)
		if err != nil {
			return fmt.Errorf("getting iface type: %s", err)
		}

		switch t {
		case "veth":
			cfg := boxnet.VethConf{}
			err := boxnet.ConfigFromRawConfig(rawConf, &cfg)
			if err != nil {
				return fmt.Errorf("parsing iface config: %+v ** %s", rawConf, err)
			}

			iface, err := boxnet.VethFromConfig(cfg, c.childProcess.pid)
			if err != nil {
				return fmt.Errorf("setting up iface %q: %s", cfg.Name, err)
			}

			if err = iface.Up(); err != nil {
				return fmt.Errorf("setting iface up %q: %s", cfg.Name, err)
			}

			errC := make(chan error, 1)
			err = boxnet.ExecuteOnNs(c.childProcess.pid, func() {
				if err = iface.PeerUp(); err != nil {
					errC <- fmt.Errorf("setting peer interface up %q: %s", cfg.PeerName, err)
					return
				}
				errC <- nil
			})
			if err != nil {
				return fmt.Errorf(
					"entering box NS to set peer interface up %q: %s", cfg.PeerName, err,
				)
			}
			if err = <-errC; err != nil {
				return err
			}

			err = boxnet.ExecuteOnNs(c.childProcess.pid,
				func() {
					err = iface.SetRoutes(cfg.Routes)
					if err != nil {
						errC <- fmt.Errorf("configuring routes for iface %q: %s", cfg.PeerName, err)
						return
					}
					errC <- nil
				},
			)
			if err != nil {
				return fmt.Errorf(
					"entering box NS to setup peer routes %q: %s", cfg.PeerName, err,
				)
			}
			if err = <-errC; err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected iface type: %s", t)
		}

	}

	return nil
}

func readFromExecFifo(execFifo io.Reader) error {
	data, err := ioutil.ReadAll(execFifo)
	if err != nil {
		return err
	}
	if len(data) <= 0 {
		return fmt.Errorf("cannot start an already running box")
	}
	return nil
}

func awaitProcessExit(pid int, exit <-chan struct{}) <-chan struct{} {
	isDead := make(chan struct{})
	go func() {
		for {
			select {
			case <-exit:
				return
			case <-time.After(time.Millisecond * 100):
				stat, err := system.Stat(pid)
				if err != nil || stat.State == system.Zombie {
					close(isDead)
					return
				}
			}
		}
	}()
	return isDead
}

func awaitFifoOpen(path string) <-chan openResult {
	fifoOpened := make(chan openResult)
	go func() {
		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err != nil {
			fifoOpened <- openResult{err: fmt.Errorf("open exec fifo for reading: %s", err)}
			return
		}
		fifoOpened <- openResult{file: f}
	}()
	return fifoOpened
}
