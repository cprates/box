package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	log "github.com/sirupsen/logrus"
)

func (b *boxRuntime) Start(pid int) error {
	// TODO: currently there is no way to know if the child process has died for sure.
	//  runC uses the start time of the process to make sure it is the right process
	//  but for now I'm not storing the box state yet
	// first check if the waiting child is still alive
	//stat, err := system.Stat(pid)
	//if err != nil {
	//	return fmt.Errorf("container is stopped")
	//}
	//if stat.StartTime != c.initProcessStartTime ||
	//	stat.State == system.Zombie ||
	//	stat.State == system.Dead {
	//	return fmt.Errorf("container is stopped")
	//}

	// TODO: must be thread safe

	b.childProcess = process{
		pid:    pid,
		config: b.childProcess.config,
	}

	return b.exec()
}

func (b *boxRuntime) start() (err error) {
	cmd := exec.Command("/proc/self/exe", "bootstrap")
	// TODO: set IO correctly
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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
		"BOX_BOOTSTRAP_LOG_FD=" + strconv.Itoa(int(os.Stdout.Fd())), // TODO: handle logging properly
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
	b.childProcess.state = StateCreated

	// TODO: delete after storing this in a state file
	log.Printf("Box instance PID:::::: %d\n\n", cmd.Process.Pid)

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
