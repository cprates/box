package spec

// Reference: https://github.com/opencontainers/runtime-spec/blob/master/config.md

import (
	"encoding/json"
	"fmt"
	"os"
)

type Spec struct {
	Version  string   `json:"ociVersion"`
	Process  *Process `json:"process"`
	Root     *Root    `json:"root"`
	Hostname string   `json:"hostname,omitempty"`
}

type Process struct {
	Terminal bool     `json:"terminal,omitempty"`
	Args     []string `json:"args,omitempty"`
	Env      []string `json:"env,omitempty"`
	Cwd      string   `json:"cwd"`
	// TODO
	//Rlimits []POSIXRlimit `json:"rlimits,omitempty" platform:"linux,solaris"`
}

type Root struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly,omitempty"`
}

func Load(path string) (spec *Spec, err error) {
	f, err := os.Open(path)
	if err != nil {
		err = fmt.Errorf("unable to open spec file: %s", err)
		return
	}
	defer f.Close()

	if err = json.NewDecoder(f).Decode(&spec); err != nil {
		return nil, err
	}

	if err = spec.Valid(); err != nil {
		err = fmt.Errorf("given file contains invalid spec: %s", err)
		return nil, err
	}

	return spec, nil
}
