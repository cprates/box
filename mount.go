package main

import (
	"fmt"
	"os"
	"path"
	"syscall"

	"golang.org/x/sys/unix"
)

func mountPoints(rootFs string) (err error) {
	// doing the mounts like this makes the code extremely repetitive. The right thing to do for
	// mounts would be get the mount points from the spec, but for now I want to make the steps
	// clear
	// ref: https://github.com/opencontainers/runc/blob/master/libcontainer/SPEC.md#filesystem
	var flags uintptr = unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID
	if err = mount("proc", "/proc", "proc", rootFs, flags, ""); err != nil {
		return
	}
	flags = unix.MS_NODEV | unix.MS_NOSUID
	if err = mount("tmpfs", "/tmp", "tmpfs", rootFs, flags, ""); err != nil {
		return
	}
	flags = unix.MS_NOEXEC | unix.MS_STRICTATIME
	if err = mount("tmpfs", "/dev", "tmpfs", rootFs, flags, "mode=755"); err != nil {
		return
	}
	flags = unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_RDONLY
	if err = mount("sysfs", "/sys", "sysfs", rootFs, flags, ""); err != nil {
		return
	}
	flags = unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID
	if err = mount("mqueue", "/dev/mqueue", "mqueue", rootFs, flags, ""); err != nil {
		return
	}
	flags = unix.MS_NOEXEC | unix.MS_NOSUID
	data := "newinstance,ptmxmode=0666,mode=620,gid=5"
	if err = mount("devpts", "/dev/pts", "devpts", rootFs, flags, data); err != nil {
		return
	}
	flags = unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID
	data = "mode=1777,size=65536k"
	if err = mount("tmpfs", "/dev/shm", "tmpfs", rootFs, flags, data); err != nil {
		return
	}

	return
}

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
