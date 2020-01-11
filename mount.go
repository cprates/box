package main

import (
	"fmt"
	"os"
	"path"
	"syscall"
)

func mount(source, target, fsType, fsPath string, flags uintptr, data string) (err error) {
	at := path.Join(fsPath, target)

	if err = os.MkdirAll(at, 0755); err != nil {
		err = fmt.Errorf("creating dir %q: %s", at, err)
		return
	}

	err = syscall.Mount(source, at, fsType, flags, data)
	if err != nil {
		err = fmt.Errorf("mounting target at %q: %s", at, err)
		return
	}

	return
}
