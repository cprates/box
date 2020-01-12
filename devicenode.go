package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func createDeviceNodes(rootFs string) (err error) {
	// refs: https://github.com/opencontainers/runc/blob/master/libcontainer/SPEC.md#runtime-and-init-process
	//       https://github.com/opencontainers/runtime-spec/blob/master/config-linux.md#devices
	//       https://www.kernel.org/doc/Documentation/admin-guide/devices.txt

	// just as mount points, the right thing to do for would be get the devices from the spec,
	// but for now I want to make the steps clear

	if err = createDeviceNode(1, 3, "/dev/null", 'c', 666, 0, 0, rootFs); err != nil {
		return
	}
	if err = createDeviceNode(1, 5, "/dev/zero", 'c', 666, 0, 0, rootFs); err != nil {
		return
	}
	if err = createDeviceNode(1, 7, "/dev/full", 'c', 666, 0, 0, rootFs); err != nil {
		return
	}
	if err = createDeviceNode(1, 8, "/dev/random", 'c', 666, 0, 0, rootFs); err != nil {
		return
	}
	if err = createDeviceNode(1, 9, "/dev/urandom", 'c', 666, 0, 0, rootFs); err != nil {
		return
	}
	if err = createDeviceNode(5, 0, "/dev/tty", 'c', 666, 0, 5, rootFs); err != nil {
		return
	}

	ptmx := filepath.Join(rootFs, "/dev/ptmx")
	if err := os.Remove(ptmx); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to remove existing symlink dev ptmx at %q: %s", ptmx, err)
	}
	if err := os.Symlink("pts/ptmx", ptmx); err != nil {
		return fmt.Errorf("creating symlink dev ptmx %s", err)
	}

	// TODO: lets leave console for later when we support terminal flag

	return
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
