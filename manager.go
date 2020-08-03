package box

import (
	"errors"
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/cprates/box/spec"
	"github.com/cprates/box/system"
)

// Interface defines the interface through which we can manage Boxes.
type Interface interface {
	Create(name string, io ProcessIO, spec *spec.Spec, opts ...BoxOption) (box Box, err error)
	Run(name string, io ProcessIO, spec *spec.Spec, opts ...BoxOption) (err error)
	Load(name string, io ProcessIO) (box Box, err error)
	Destroy(name string) (err error)
}

type manager struct {
	workdir string
	lock    sync.Mutex
}

const execFifoFilename = "exec.fifo"

const stdioFdCount = 3

var ErrBoxExists = errors.New("box exists")

var _ Interface = (*manager)(nil)

// New returns a new ready to use Box manager which will use the given workdir to store and load
// Boxes. The given workdir must be an absolute path.
func New(workdir string) Interface {
	return &manager{
		workdir: workdir,
		lock:    sync.Mutex{},
	}
}

// Load loads an existing box with the given name from the configured workdir.
// TODO: io doesn't seem to belong here... Should come from the state file.
func (m *manager) Load(name string, io ProcessIO) (box Box, err error) {
	state, err := m.loadStateFromName(name)
	if err != nil {
		err = fmt.Errorf("while loading state: %s", err)
		return nil, err
	}

	box = &boxInternal{
		state:        *state,
		childProcess: process{io: io},
		config: config{
			Name:           state.BoxConfig.Name,
			Hostname:       state.BoxConfig.Hostname,
			RootFs:         state.BoxConfig.RootFs,
			EntryPoint:     state.BoxConfig.EntryPoint,
			EntryPointArgs: state.BoxConfig.EntryPointArgs,
			ExecFifoPath:   state.BoxConfig.ExecFifoPath,
			StateFilePath:  state.BoxConfig.StateFilePath,
		},
	}

	return
}

// Create creates a new Box with the given name and spec, which will use the given io
// to communicate with the exterior world. The name must be unique for each workdir.
func (m *manager) Create(
	name string,
	io ProcessIO,
	spec *spec.Spec,
	opts ...BoxOption,
) (
	box Box,
	err error,
) {
	m.lock.Lock()
	defer m.lock.Unlock()
	boxDir := path.Join(m.workdir, name)

	_, err = os.Stat(boxDir)
	if err == nil {
		err = ErrBoxExists
		return
	}
	if !os.IsNotExist(err) {
		return
	}

	err = os.MkdirAll(boxDir, 0766)
	if err != nil {
		err = fmt.Errorf("while creating dir %q: %s", boxDir, err)
		return
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(boxDir)
		}
	}()

	b := newBox()
	err = b.create(name, boxDir, io, spec, opts...)
	if err != nil {
		err = fmt.Errorf("while creating box %q: %s", boxDir, err)
		return
	}

	box = b
	return
}

// Run creates and starts a new box with name and io with the given spec, blocking until
// the box is terminated.
func (m *manager) Run(
	name string,
	io ProcessIO,
	spec *spec.Spec,
	opts ...BoxOption,
) (
	err error,
) {
	m.lock.Lock()
	defer m.lock.Unlock()

	boxDir := path.Join(m.workdir, name)

	_, err = os.Stat(boxDir)
	if err == nil {
		err = ErrBoxExists
		return
	}
	if !os.IsNotExist(err) {
		return
	}

	err = os.MkdirAll(boxDir, 0766)
	if err != nil {
		err = fmt.Errorf("while creating dir %q: %s", boxDir, err)
		return
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(boxDir)
		}
	}()

	b := newBox()
	err = b.run(name, boxDir, io, spec, opts...)
	if err != nil {
		err = fmt.Errorf("while creating box %q: %s", boxDir, err)
		return
	}

	return
}

func (m *manager) Destroy(name string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	state, err := m.loadStateFromName(name)
	if err != nil {
		return fmt.Errorf("unable to load state: %s", err)
	}

	stat, err := system.Stat(state.BoxPID)
	if err != nil || stat.StartTime != state.ProcessStartClockTicks {
		boxWd := path.Join(m.workdir, state.BoxConfig.Name)
		err = os.RemoveAll(boxWd)
		if err != nil {
			return fmt.Errorf("cleaning up box dir: %s", err)
		}

		return nil
	}

	p, err := os.FindProcess(state.BoxPID)
	if err != nil {
		// this shouldn't happen in linux according to the doc
		return fmt.Errorf("couldn't find process to kill with PID %d: %s", state.BoxPID, err)
	}

	err = p.Kill()
	if err != nil {
		return fmt.Errorf("unable to kill process with PID %d: %s", state.BoxPID, err)
	}

	// TODO: add a timeout
	<-awaitProcessExit(state.BoxPID, make(chan struct{}))

	boxWd := path.Join(m.workdir, state.BoxConfig.Name)
	err = os.RemoveAll(boxWd)
	if err != nil {
		return fmt.Errorf("cleaning up box dir after killing process: %s", err)
	}

	return nil
}
