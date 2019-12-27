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
}

func (r *boxRuntime) saveState() (err error) {
	f, err := os.Create(filepath.Join(r.workdir, r.childProcess.config.Name, stateFilename))
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(r.state)
	return
}

func (r *boxRuntime) loadState() (err error) {
	f, err := os.Open(filepath.Join(r.workdir, r.childProcess.config.Name, stateFilename))
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&r.state)
	return
}
