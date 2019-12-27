package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/cprates/box/system"
)

func (b *boxRuntime) Start() error {
	// TODO: must be thread safe

	err := b.loadState()
	if err != nil {
		return err
	}

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

	b.childProcess = process{
		pid:    b.state.BoxPID,
		config: b.childProcess.config,
	}

	return b.exec()
}

func (b *boxRuntime) start() (err error) {
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
	if err = json.NewEncoder(configWPipe).Encode(&b.childProcess.config); err != nil {
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
		BoxPID:        b.childProcess.pid,
		Created:       b.childProcess.created,
		ProcessConfig: b.childProcess.config,
	}

	stat, err := system.Stat(cmd.Process.Pid)
	if err == nil {
		b.state.ProcessStartClockTicks = stat.StartTime
		err = b.saveState()
		if err == nil {
			return
		}
	}

	// we were unable to save the box's state so, kill the brand new child process

	if err = cmd.Process.Kill(); err != nil {
		err = fmt.Errorf("while killing child process: %s", err)
		return
	}

	c := make(chan error)
	go func() {
		c <- cmd.Wait()
	}()

	select {
	case err = <-c:
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
