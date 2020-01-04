package main

import (
	"fmt"
	"os"
	"path"

	"github.com/cprates/box/spec"
)

type Cartoner interface {
	CreateBox(name string, io ProcessIO, spec *spec.Spec) (box Boxer, err error)
	LoadBox(name string, io ProcessIO) (box Boxer, err error)
	//DestroyBox() (err error)
}

type carton struct {
	workdir string
}

const execFifoFilename = "exec.fifo"

const stdioFdCount = 3

var _ Cartoner = (*carton)(nil)

func New(workdir string) Cartoner {
	return &carton{
		workdir: workdir,
	}
}

// TODO: doc. io doesn't seem to belong here... Should come from the state file.
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

// TODO: doc.
func (c *carton) CreateBox(name string, io ProcessIO, spec *spec.Spec) (box Boxer, err error) {
	boxDir := path.Join(c.workdir, name)
	err = os.MkdirAll(boxDir, 0766)
	if err != nil {
		err = fmt.Errorf("box: while creating dir %q: %s", boxDir, err)
		return
	}

	b := newCartonBox()
	err = b.create(name, boxDir, io, spec)
	if err != nil {
		err = fmt.Errorf("box: while creating box %q: %s", boxDir, err)
		return
	}

	box = b
	return
}
