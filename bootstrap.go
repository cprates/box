package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"syscall"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// TODO: delete
func must(err error) {
	if err != nil {
		panic(err)
	}
}

func setupEnv(conf *config) (err error) {
	if err = mountPoints(conf.RootFs); err != nil {
		return
	}

	if err = createDeviceNodes(conf.RootFs); err != nil {
		return
	}

	if err = createDevSymlinks(conf.RootFs); err != nil {
		return
	}

	// TODO
	//  https://github.com/opencontainers/runc/blob/master/libcontainer/SPEC.md#runtime-and-init-process
	if err = setHostname(conf.Hostname, path.Join(conf.RootFs, "/etc/hostname")); err != nil {
		return fmt.Errorf("setting hostname: %s", err)
	}

	if err := syscall.Mknod(path.Join(conf.RootFs, "/dev/null"), 1, 3); err != nil {
		if !os.IsExist(err) {
			must(err)
		}
	}

	// TODO: os.Clearenv() and set the bare minimum env vars

	must(syscall.Chroot(conf.RootFs))
	must(os.Chdir("/"))

	return
}

func setHostname(hostname, path string) (err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0665)
	if err != nil {
		return
	}
	defer f.Close()

	n, err := fmt.Fprintln(f, hostname)
	if err != nil {
		return
	}
	if n != len(hostname)+1 {
		err = fmt.Errorf("unfinished write to file, got %d expect %d", n, len(hostname))
		return
	}

	if err = syscall.Sethostname([]byte(hostname)); err != nil {
		return
	}

	return
}

func cleanup() {
	//must(os.Remove("/dev/null"))
	//must(syscall.Unmount("/proc", 0))
}

func pipe(sFd, name string) (f *os.File, err error) {
	d, err := strconv.Atoi(sFd)
	if err != nil {
		err = fmt.Errorf("parsing fd: %s", err)
		return
	}
	fd := uintptr(d)

	f = os.NewFile(fd, name)
	if f == nil {
		err = fmt.Errorf("opening file with fd %q: %s", fd, err)
		return
	}

	return
}

// TODO: failures must be properly handled while bootstrapping
func Bootstrap(configFd, logFd string) (err error) {

	logPipe, err := pipe(logFd, "logPipe")
	if err != nil {
		err = fmt.Errorf("creating log pipe: %s\n", err)
		return
	}
	defer func() {
		_ = logPipe.Close()
	}()
	log.SetOutput(logPipe)

	configPipe, err := pipe(configFd, "configPipe")
	if err != nil {
		err = fmt.Errorf("creating config pipe: %s\n", err)
		log.Error(err)
		return
	}
	defer func() {
		_ = configPipe.Close()
	}()

	conf := config{}
	if err = json.NewDecoder(configPipe).Decode(&conf); err != nil {
		err = fmt.Errorf("reading config: %s\n", err)
		log.Error(err)
		return
	}

	err = setupEnv(&conf)
	if err != nil {
		err = fmt.Errorf("unable to setup environment: %s", err)
		log.Error(err)
		return
	}
	defer func() {
		cleanup()
	}()

	log.Debugf("Bootstrapping box %s: %s %v \n", conf.Name, conf.EntryPoint, conf.EntryPointArgs)

	boxFifoFd := os.Getenv("BOX_FIFO_FD")
	if boxFifoFd != "" {
		if err = syncParent(); err != nil {
			return
		}
	}

	err = syscall.Exec(
		conf.EntryPoint,
		append([]string{path.Base(conf.EntryPoint)}, conf.EntryPointArgs...),
		os.Environ(),
	)
	log.Errorf("bootstrap: executing entry point: %s", err)

	return
}

func syncParent() (err error) {
	fifoFd, err := strconv.Atoi(os.Getenv("BOX_FIFO_FD"))
	if err != nil {
		err = fmt.Errorf("unable to get fifo fd: %s", err)
		log.Error(err)
		return
	}

	fd, err := unix.Open(fmt.Sprintf("/proc/self/fd/%d", fifoFd), unix.O_WRONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		err = fmt.Errorf("open exec fifo: %s", err)
		log.Error(err)
		return
	}

	if _, err = unix.Write(fd, []byte("0")); err != nil {
		err = fmt.Errorf("write 0 exec fifo: %s", err)
		log.Error(err)
		return
	}

	_ = unix.Close(fifoFd)
	return
}
