package bootstrap

import (
	"fmt"
	"os"
	"path"
	"syscall"

	"golang.org/x/sys/unix"
)

// DefaultMounts returns a list of the default device nodes for a container as specified at
// https://github.com/opencontainers/runc/blob/master/libcontainer/SPEC.md#filesystem
func DefaultMounts(rootFs string) []Option {
	return []Option{
		ProcMount(rootFs),
		TmpMount(rootFs),
		DevMount(rootFs),
		SysMount(rootFs),
		MqueueMount(rootFs),
		PtsMount(rootFs),
		ShmMount(rootFs),
	}
}

func ProcMount(rootFs string) Option {
	return func() error {
		var flags uintptr = unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID
		return mount("proc", "/proc", "proc", rootFs, flags, "")
	}
}

func TmpMount(rootFs string) Option {
	return func() error {
		flags := unix.MS_NODEV | unix.MS_NOSUID
		return mount("tmpfs", "/tmp", "tmpfs", rootFs, uintptr(flags), "")
	}
}

func DevMount(rootFs string) Option {
	return func() error {
		flags := unix.MS_NOEXEC | unix.MS_STRICTATIME
		return mount("tmpfs", "/dev", "tmpfs", rootFs, uintptr(flags), "mode=755")
	}
}

func SysMount(rootFs string) Option {
	return func() error {
		flags := unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_RDONLY
		return mount("sysfs", "/sys", "sysfs", rootFs, uintptr(flags), "")
	}
}

func MqueueMount(rootFs string) Option {
	return func() error {
		flags := unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID
		return mount("mqueue", "/dev/mqueue", "mqueue", rootFs, uintptr(flags), "")
	}
}

func PtsMount(rootFs string) Option {
	return func() error {
		flags := unix.MS_NOEXEC | unix.MS_NOSUID
		data := "newinstance,ptmxmode=0666,mode=620,gid=5"
		return mount("devpts", "/dev/pts", "devpts", rootFs, uintptr(flags), data)
	}
}

func ShmMount(rootFs string) Option {
	return func() error {
		flags := unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID
		data := "mode=1777,size=65536k"
		return mount("tmpfs", "/dev/shm", "tmpfs", rootFs, uintptr(flags), data)
	}
}

func mount(source, target, fsType, rootFs string, flags uintptr, data string) (err error) {
	at := path.Join(rootFs, target)

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
