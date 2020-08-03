package box

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

func (b *boxInternal) saveState() (err error) {
	f, err := os.Create(b.config.StateFilePath)
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(b.state)
	return
}

func (b *boxInternal) loadState() (err error) {
	f, err := os.Open(b.config.StateFilePath)
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&b.state)
	return
}

func (m *manager) loadStateFromName(name string) (s *state, err error) {
	f, err := os.Open(filepath.Join(m.workdir, name, stateFilename))
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
