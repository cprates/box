package runtime

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
	ProcessConfig          config
}

func (b *boxRuntime) saveState() (err error) {
	f, err := os.Create(filepath.Join(b.workdir, b.childProcess.config.Name, stateFilename))
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(b.state)
	return
}

func (b *boxRuntime) loadState() (err error) {
	f, err := os.Open(filepath.Join(b.workdir, b.childProcess.config.Name, stateFilename))
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&b.state)
	return
}

func (b *boxRuntime) loadStateFromName(name string) (err error) {
	f, err := os.Open(filepath.Join(b.workdir, name, stateFilename))
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&b.state)
	return
}
