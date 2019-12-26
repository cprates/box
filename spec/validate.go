package spec

import (
	"errors"
	"path/filepath"
	"strings"
)

func (s Spec) Valid() error {
	verErr := errors.New("spec version not supported")
	v := strings.Split(s.Version, ".")
	if s.Version == "" {
		return errors.New("spec version must be specified")
	}
	if len(v) != 3 {
		return verErr
	}
	if v[0] != "1" || v[1] != "0" || v[2] != "1" {
		return verErr
	}

	if err := s.Process.Valid(); err != nil {
		return err
	}

	if err := s.Root.Valid(); err != nil {
		return err
	}

	return nil
}

func (p Process) Valid() error {
	if p.Cwd == "" {
		return errors.New("cwd property must not be empty")
	}
	if !filepath.IsAbs(p.Cwd) {
		return errors.New("cwd must be an absolute path")
	}
	if len(p.Args) == 0 {
		return errors.New("args list must not be empty")
	}

	return nil
}

func (r Root) Valid() error {
	if r.Readonly {
		return errors.New("read-only root not supported")
	}

	return nil
}
