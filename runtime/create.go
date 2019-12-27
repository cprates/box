package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/cprates/box/spec"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const execFifoFilename = "exec.fifo"

const stdioFdCount = 3

type boxRuntime struct {
	workdir      string
	state        state
	childProcess process
}

type process struct {
	created bool
	pid     int
	config  config
}

type config struct {
	Name           string
	Hostname       string
	RootFs         string
	EntryPoint     string
	EntryPointArgs []string // entryPoint args
}

type Runtimer interface {
	Create() (err error)
	Start() (err error)
	//Run() (err error)
	//Exec() (err error)
}

func New(name, workdir string, spec *spec.Spec) Runtimer {
	hostname := spec.Hostname
	if hostname == "" {
		hostname = name
	}

	p := process{
		config: config{
			Name:           name,
			Hostname:       hostname,
			RootFs:         spec.Root.Path,
			EntryPoint:     spec.Process.Args[0],
			EntryPointArgs: append(spec.Process.Args[:0:0], spec.Process.Args...)[1:],
		},
	}

	return &boxRuntime{
		workdir:      workdir,
		childProcess: p,
	}
}

func (b *boxRuntime) Create() (err error) {
	log.Debugf("Creating Box %v \n", b.childProcess.config.Name)

	dir := path.Join(b.workdir, b.childProcess.config.Name)
	err = os.MkdirAll(dir, 0766)
	if err != nil {
		err = fmt.Errorf("while creating dir %q: %s", dir, err)
		return
	}

	if err = b.createExecFifo(); err != nil {
		err = fmt.Errorf("box: creating exec fifo: %s", err)
		return
	}

	// this is a much simpler implementation compared to runC so, at this point it just needs
	// to re-execute itself setting all the namespaces, except user ns.
	err = b.start()
	if err != nil {
		b.deleteExecFifo()
		err = fmt.Errorf("box: creating container: %s", err)
		return
	}

	return
}

func (b *boxRuntime) createExecFifo() error {
	fifoName := filepath.Join(b.workdir, b.childProcess.config.Name, execFifoFilename)
	if _, err := os.Stat(fifoName); err == nil {
		return fmt.Errorf("exec fifo %s already exists", fifoName)
	}
	oldMask := unix.Umask(0000)
	if err := unix.Mkfifo(fifoName, 0622); err != nil {
		unix.Umask(oldMask)
		err = fmt.Errorf("unable to create fifo at %q: %s", fifoName, err)
		return err
	}
	unix.Umask(oldMask)

	// currently it does not support user namespaces so, uid and gid are always set to 0
	return os.Chown(fifoName, 0, 0)
}

func (b *boxRuntime) deleteExecFifo() {
	fifoName := filepath.Join(b.workdir, b.childProcess.config.Name, execFifoFilename)
	os.Remove(fifoName)
}

// includeExecFifo opens the box's execfifo as a pathfd, so that the
// box cannot access the statedir (and the FIFO itself remains
// un-opened). It then adds the FifoFd to the given exec.Cmd as an inherited
// fd, with BOX_FIFO_FD set to its fd number.
func (b *boxRuntime) includeExecFifo(cmd *exec.Cmd) error {
	fifoName := filepath.Join(b.workdir, b.childProcess.config.Name, execFifoFilename)
	fifoFd, err := unix.Open(fifoName, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, os.NewFile(uintptr(fifoFd), fifoName))
	cmd.Env = append(cmd.Env, fmt.Sprintf("BOX_FIFO_FD=%d", stdioFdCount+len(cmd.ExtraFiles)-1))
	return nil
}
