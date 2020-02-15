package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// DefaultNodeDevs returns a list of the default device nodes for a container as specified at
// https://github.com/opencontainers/runtime-spec/blob/master/config-linux.md#default-devices
// other refs:
//       https://github.com/opencontainers/runc/blob/master/libcontainer/SPEC.md#runtime-and-init-process
//       https://github.com/opencontainers/runtime-spec/blob/master/config-linux.md#devices
//       https://www.kernel.org/doc/Documentation/admin-guide/devices.txt
func DefaultNodeDevs(rootFs string) []Option {
	return []Option{
		NullDev(rootFs),
		ZeroDev(rootFs),
		FullDev(rootFs),
		RandomDev(rootFs),
		URandomDev(rootFs),
		TtyDev(rootFs),
		PtmxDev(rootFs),
	}
}

func NullDev(rootFs string) Option {
	return func() error {
		return createDeviceNode(1, 3, "/dev/null", 'c', 666, 0, 0, rootFs)
	}
}

func ZeroDev(rootFs string) Option {
	return func() error {
		return createDeviceNode(1, 5, "/dev/zero", 'c', 666, 0, 0, rootFs)
	}
}

func FullDev(rootFs string) Option {
	return func() error {
		return createDeviceNode(1, 7, "/dev/full", 'c', 666, 0, 0, rootFs)
	}
}

func RandomDev(rootFs string) Option {
	return func() error {
		return createDeviceNode(1, 8, "/dev/random", 'c', 666, 0, 0, rootFs)
	}
}

func URandomDev(rootFs string) Option {
	return func() error {
		return createDeviceNode(1, 9, "/dev/urandom", 'c', 666, 0, 0, rootFs)
	}
}

func TtyDev(rootFs string) Option {
	return func() error {
		return createDeviceNode(5, 0, "/dev/tty", 'c', 666, 0, 5, rootFs)
	}
}

func PtmxDev(rootFs string) Option {
	return func() error {
		ptmx := filepath.Join(rootFs, "/dev/ptmx")
		if err := os.Remove(ptmx); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unable to remove existing symlink dev ptmx at %q: %s", ptmx, err)
		}
		if err := os.Symlink("pts/ptmx", ptmx); err != nil {
			return fmt.Errorf("creating symlink dev ptmx %s", err)
		}

		return nil
	}
}

func createDeviceNode(
	major, minor uint32,
	target string,
	nodeType byte,
	fileMode uint32,
	uid, gid int,
	rootFs string,
) (err error) {
	absPath := filepath.Join(rootFs, target)
	if err = os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return
	}

	mode := fileMode
	switch nodeType {
	case 'c', 'u':
		mode |= unix.S_IFCHR
	case 'b':
		mode |= unix.S_IFBLK
	case 'p':
		mode |= unix.S_IFIFO
	default:
		return fmt.Errorf("%c is not a valid device type for device %s", nodeType, target)
	}

	oldMask := unix.Umask(0000)
	defer unix.Umask(oldMask)

	dev := unix.Mkdev(major, minor) & 0xffffffff
	if err = unix.Mknod(absPath, mode, int(dev)); err != nil {
		if os.IsExist(err) {
			// if it already exists, that's not a problem
			return nil
		}
		return fmt.Errorf("unable to create device node %q: %s", absPath, err)
	}

	return unix.Chown(absPath, uid, gid)
}

// copied from runc
func createDevSymlinks(rootFs string) (err error) {
	var links = [][2]string{
		{"/proc/self/fd", "/dev/fd"},
		{"/proc/self/fd/0", "/dev/stdin"},
		{"/proc/self/fd/1", "/dev/stdout"},
		{"/proc/self/fd/2", "/dev/stderr"},
	}
	// kcore support can be toggled with CONFIG_PROC_KCORE; only create a symlink
	// in /dev if it exists in /proc.
	if _, err := os.Stat("/proc/kcore"); err == nil {
		links = append(links, [2]string{"/proc/kcore", "/dev/core"})
	}
	for _, link := range links {
		var (
			src = link[0]
			dst = filepath.Join(rootFs, link[1])
		)
		if err := os.Symlink(src, dst); err != nil && !os.IsExist(err) {
			return fmt.Errorf("creating symlink %s %s %s", src, dst, err)
		}
	}
	return nil
}
