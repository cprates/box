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
	"syscall"
	"time"

	"github.com/cprates/box/spec"
	"github.com/cprates/box/system"

	"golang.org/x/sys/unix"
)

type Boxer interface {
	create(name, workdir string, io ProcessIO, spec *spec.Spec) (err error)

	Start() (err error)
	//Run() (err error)
	//Exec() (err error)
}

type cartonBox struct {
	state        state
	childProcess process
	config       config
}

type process struct {
	created bool
	pid     int
	io      ProcessIO
}

type config struct {
	Name           string
	Hostname       string
	RootFs         string
	EntryPoint     string
	EntryPointArgs []string // entryPoint args
	ExecFifoPath   string
	StateFilePath  string
}

type openResult struct {
	file *os.File
	err  error
}

var _ Boxer = (*cartonBox)(nil)

func newCartonBox() Boxer {
	return &cartonBox{}
}

// here the workdir is from the box's point of view
func (c *cartonBox) create(name, workdir string, io ProcessIO, spec *spec.Spec) (err error) {
	// the hostname in the spec is optional so, if it isn't set, set it to the given name
	hostname := spec.Hostname
	if hostname == "" {
		hostname = name
	}

	c.childProcess = process{io: io}
	c.config = config{
		Name:           name,
		Hostname:       hostname,
		RootFs:         spec.Root.Path,
		EntryPoint:     spec.Process.Args[0],
		EntryPointArgs: append(spec.Process.Args[:0:0], spec.Process.Args...)[1:],
		ExecFifoPath:   filepath.Join(workdir, execFifoFilename),
		StateFilePath:  filepath.Join(workdir, stateFilename),
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

// TODO: doc.
func (c *cartonBox) Start() error {
	// TODO: must be thread safe

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
		err = c.saveState()
		if err == nil {
			return
		}
	}

	// we were unable to save the box's state so, kill the brand new child process

	if err = cmd.Process.Kill(); err != nil {
		err = fmt.Errorf("while killing child process: %s", err)
		return
	}

	errC := make(chan error)
	go func() {
		errC <- cmd.Wait()
	}()

	select {
	case err = <-errC:
		if err != nil {
			err = fmt.Errorf("while waiting for child process to die: %s", err)
			return
		}
	case <-time.After(500 * time.Millisecond):
		err = errors.New("child process didn't return in time after being killed")
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
	fifoFd, err := unix.Open(c.config.ExecFifoPath, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, os.NewFile(uintptr(fifoFd), c.config.ExecFifoPath))
	cmd.Env = append(cmd.Env, fmt.Sprintf("BOX_FIFO_FD=%d", stdioFdCount+len(cmd.ExtraFiles)-1))
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
