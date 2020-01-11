package spec

import (
	"encoding/json"
	"fmt"
	"os"
)

// Spec is the base configuration for the container.
// Check https://github.com/opencontainers/runtime-spec/blob/master/config.md for more details.
// If you cannot find here a field  documented in the link above, is because it is not supported.
type Spec struct {
	// Version of the Open Container Initiative Runtime Specification with which the bundle complies
	Version string `json:"ociVersion"`
	// Process configures the container process.
	Process *Process `json:"process,omitempty"`
	// Root configures the container's root filesystem.
	Root *Root `json:"root,omitempty"`
	// Hostname configures the container's hostname.
	Hostname string `json:"hostname,omitempty"`
}

// Process contains information to start a specific application inside the container.
type Process struct {
	// Terminal creates an interactive terminal for the container
	Terminal bool `json:"terminal,omitempty"`
	// Args specifies the binary and arguments for the application to execute
	Args []string `json:"args,omitempty"`
	// Env populates the process environment for the process
	Env []string `json:"env,omitempty"`
	// Cwd is the current working directory for the process and must be
	// relative to the container's root
	Cwd string `json:"cwd"`
	// TODO
	//Rlimits []POSIXRlimit `json:"rlimits,omitempty" platform:"linux,solaris"`
}

// Root contains information about the container's root filesystem on the host
type Root struct {
	// Path is the absolute path to the container's root filesystem
	Path string `json:"path"`
	// TODO: not fully implemented yet
	// Readonly makes the root filesystem for the container readonly before the process is executed
	Readonly bool `json:"readonly,omitempty"`
}

// Load a Box spec from the given path.
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
