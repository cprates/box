package main

import (
	"fmt"
	"os"
	"path"

	"github.com/cprates/box/spec"
)

// Cartoner defines the interface through which we can manage Boxes.
type Cartoner interface {
	CreateBox(name string, io ProcessIO, spec *spec.Spec, opts ...BoxOption) (box Boxer, err error)
	RunBox(name string, io ProcessIO, spec *spec.Spec, opts ...BoxOption) (err error)
	LoadBox(name string, io ProcessIO) (box Boxer, err error)
	//DestroyBox() (err error)
}

type carton struct {
	workdir string
}

const execFifoFilename = "exec.fifo"

const stdioFdCount = 3

var _ Cartoner = (*carton)(nil)

// New returns a new ready to use Boxes manager which will use the given workdir to store and load
// Boxes. The given workdir should be an absolute path.
func New(workdir string) Cartoner {
	return &carton{
		workdir: workdir,
	}
}

// LoadBox loads an existing box with the given name from the configured workdir.
// TODO: io doesn't seem to belong here... Should come from the state file.
func (c *carton) LoadBox(name string, io ProcessIO) (box Boxer, err error) {
	state, err := c.loadStateFromName(name)
	if err != nil {
		err = fmt.Errorf("box: while loading state: %s", err)
		return nil, err
	}

	box = &cartonBox{
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

// CreateBox creates a new Box with the given name and spec, which will use the given io
// to communicate with the exterior world. The name must be unique for each workdir.
func (c *carton) CreateBox(
	name string,
	io ProcessIO,
	spec *spec.Spec,
	opts ...BoxOption,
) (
	box Boxer,
	err error,
) {
	boxDir := path.Join(c.workdir, name)
	err = os.MkdirAll(boxDir, 0766)
	if err != nil {
		err = fmt.Errorf("box: while creating dir %q: %s", boxDir, err)
		return
	}

	b := newCartonBox()
	err = b.create(name, boxDir, io, spec, opts...)
	if err != nil {
		err = fmt.Errorf("box: while creating box %q: %s", boxDir, err)
		return
	}

	box = b
	return
}

// RunBox creates and starts a new box with name and io with the given spec, blocking until
// the box is terminated.
func (c *carton) RunBox(
	name string,
	io ProcessIO,
	spec *spec.Spec,
	opts ...BoxOption,
) (
	err error,
) {
	boxDir := path.Join(c.workdir, name)
	err = os.MkdirAll(boxDir, 0766)
	if err != nil {
		err = fmt.Errorf("box: while creating dir %q: %s", boxDir, err)
		return
	}

	b := newCartonBox()
	err = b.Run(name, boxDir, io, spec, opts...)
	if err != nil {
		err = fmt.Errorf("box: while creating box %q: %s", boxDir, err)
		return
	}

	return
}
