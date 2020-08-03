package box

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

// Box defines the interface to interact with a Box.
type Box interface {
	// Creates a new box with the given mame and spec, storing the state in workdir, using the
	// given io channels to communicate with the outside world
	create(name, workdir string, io ProcessIO, spec *spec.Spec, opts ...BoxOption) (err error)
	// Starts an existing Box returning immediately after the Box is running
	Start() (err error)
	// run creates and starts a new box with name at workdir with the given spec, blocking until
	// the box is terminated.
	run(name, workdir string, io ProcessIO, spec *spec.Spec, opts ...BoxOption) (err error)
}

type boxInternal struct {
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

var _ Box = (*boxInternal)(nil)

func newCartonBox() Box {
	return &boxInternal{
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
func (b *boxInternal) create(
	name, workdir string,
	io ProcessIO,
	spec *spec.Spec,
	opts ...BoxOption,
) (
	err error,
) {
	b.childProcess = process{io: io}
	b.config = config{
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
		opt(b)
	}

	if err = b.createExecFifo(); err != nil {
		err = fmt.Errorf("creating exec fifo: %s", err)
		return
	}

	// this is a much simpler implementation compared to runC so, at this point it just needs
	// to re-execute itself setting all namespaces at once, except user ns.
	err = b.start()
	if err != nil {
		b.deleteExecFifo()
		err = fmt.Errorf("creating container: %s", err)
		return
	}

	return
}

// Run creates and starts a new box with name at workdir with the given spec, blocking until
// the box is terminated.
func (b *boxInternal) run(
	name, workdir string,
	io ProcessIO,
	spec *spec.Spec,
	opts ...BoxOption,
) (
	err error,
) {
	b.childProcess = process{io: io}
	b.config = config{
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
		opt(b)
	}

	err = b.start()
	if err != nil {
		return
	}

	<-awaitProcessExit(b.childProcess.pid, make(chan struct{}))

	err = os.RemoveAll(workdir)
	if err != nil {
		err = fmt.Errorf("cleaning up workdir: %s", err)
		return
	}

	return
}

// Start a previously create Box, returning immediately after the box is started.
func (b *boxInternal) Start() error {
	b.lock.Lock()
	defer b.lock.Unlock()

	// first check if the waiting child is still alive
	stat, err := system.Stat(b.state.BoxPID)
	if err != nil {
		return fmt.Errorf("box is stopped")
	}

	if stat.StartTime != b.state.ProcessStartClockTicks ||
		stat.State == system.Zombie ||
		stat.State == system.Dead {
		return fmt.Errorf("box is stopped")
	}

	b.childProcess.pid = b.state.BoxPID

	return b.exec()
}

func (b *boxInternal) start() (err error) {
	cmd := exec.Command("/proc/self/exe", "bootstrap")
	cmd.Stdin = b.childProcess.io.In
	cmd.Stdout = b.childProcess.io.Out
	cmd.Stderr = b.childProcess.io.Err
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
		"BOX_BOOTSTRAP_LOG_FD=" + strconv.Itoa(int(b.childProcess.io.Out.Fd())),
		"BOX_DEBUG=" + os.Getenv("BOX_DEBUG"),
	}

	// send box config
	if err = json.NewEncoder(configWPipe).Encode(&b.config); err != nil {
		err = fmt.Errorf("sending config to child: %s", err)
		return
	}

	if err = b.includeExecFifo(cmd); err != nil {
		err = fmt.Errorf("including fifo fd: %s", err)
		return
	}

	if err = cmd.Start(); err != nil {
		err = fmt.Errorf("starting child: %s", err)
		return
	}

	b.childProcess.pid = cmd.Process.Pid
	b.childProcess.created = true
	b.state = state{
		BoxPID:    b.childProcess.pid,
		Created:   b.childProcess.created,
		BoxConfig: b.config,
	}

	stat, err := system.Stat(cmd.Process.Pid)
	if err == nil {
		b.state.ProcessStartClockTicks = stat.StartTime

		if b.config.NetConfig != nil {
			if err = b.setupNetFromConfig(); err != nil {
				return
			}
		}

		if err = b.saveState(); err == nil {
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

func (b *boxInternal) exec() error {
	fifoOpen := make(chan struct{})
	select {
	case <-awaitProcessExit(b.childProcess.pid, fifoOpen):
		return errors.New("box process is already dead")
	case result := <-awaitFifoOpen(b.config.ExecFifoPath):
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

func (b *boxInternal) createExecFifo() error {
	if _, err := os.Stat(b.config.ExecFifoPath); err == nil {
		return fmt.Errorf("exec fifo %s already exists", b.config.ExecFifoPath)
	}
	oldMask := unix.Umask(0000)
	if err := unix.Mkfifo(b.config.ExecFifoPath, 0622); err != nil {
		unix.Umask(oldMask)
		err = fmt.Errorf("unable to create fifo at %q: %s", b.config.ExecFifoPath, err)
		return err
	}
	unix.Umask(oldMask)

	// currently it does not support user namespaces so, uid and gid are always set to 0
	return os.Chown(b.config.ExecFifoPath, 0, 0)
}

func (b *boxInternal) deleteExecFifo() {
	_ = os.Remove(b.config.ExecFifoPath)
}

// includeExecFifo opens the box's execfifo as a pathfd, so that the
// box cannot access the statedir (and the FIFO itself remains
// un-opened). It then adds the FifoFd to the given exec.Cmd as an inherited
// fd, with BOX_FIFO_FD set to its fd number.
func (b *boxInternal) includeExecFifo(cmd *exec.Cmd) error {
	if b.config.ExecFifoPath == "" {
		return nil
	}

	fifoFd, err := unix.Open(b.config.ExecFifoPath, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, os.NewFile(uintptr(fifoFd), b.config.ExecFifoPath))
	cmd.Env = append(cmd.Env, fmt.Sprintf("BOX_FIFO_FD=%d", stdioFdCount+len(cmd.ExtraFiles)-1))
	return nil
}

func (b *boxInternal) setupNetFromConfig() error {
	// search for module
	netConfig := b.config.NetConfig
	if netConfig.Model != nil {
		m, err := boxnet.ModelFromConfig(netConfig.Model)
		if err != nil {
			return fmt.Errorf("getting network model type: %s", err)
		}

		switch m {
		case "bridge":
			modelConfig := boxnet.ModelBridge{}
			err := boxnet.ConfigFromRawConfig(netConfig.Model, &modelConfig)
			if err != nil {
				return fmt.Errorf("parsing model config: %+v ** %s", netConfig.Model, err)
			}
			_, err = boxnet.NewBridgeModel(modelConfig.BrName, b.childProcess.pid, netConfig.Interfaces)
			if err != nil {
				return fmt.Errorf("creating network model: %s", err)
			}
		default:
			return fmt.Errorf("unknown model type %q", m)
		}

		return nil
	}

	// if a model is not configured, handle interfaces on their own
	for _, rawConf := range netConfig.Interfaces {
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

			_, err = boxnet.AttachVeth(cfg, b.childProcess.pid)
			if err != nil {
				return fmt.Errorf("unable to attach veth %q: %s", cfg.Name, t)
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
