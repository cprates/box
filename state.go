package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const stateFilename = "state.json"

type state struct {
	BoxPID                 int
	Created                bool
	ProcessStartClockTicks uint64
	BoxConfig              config
}

func (c *cartonBox) saveState() (err error) {
	f, err := os.Create(c.config.StateFilePath)
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(c.state)
	return
}

func (c *cartonBox) loadState() (err error) {
	f, err := os.Open(c.config.StateFilePath)
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&c.state)
	return
}

func (c *carton) loadStateFromName(name string) (s *state, err error) {
	f, err := os.Open(filepath.Join(c.workdir, name, stateFilename))
	if err != nil {
		return
	}
	defer f.Close()

	s = &state{}
	err = json.NewDecoder(f).Decode(s)
	if err != nil {
		s = nil
	}

	return
}
