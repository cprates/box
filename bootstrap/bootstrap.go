package bootstrap

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

type Option func() error

type Config struct {
	Name           string
	Hostname       string
	RootFs         string
	EnvVars        []string
	Cwd            string
	EntryPoint     string
	EntryPointArgs []string
}

func options(cfg Config) []Option {
	return DefaultNodeDevs(cfg.RootFs)
}

func setupEnv(cfg Config) (err error) {
	if err = mountPoints(cfg.RootFs); err != nil {
		return
	}

	for _, opt := range options(cfg) {
		if e := opt(); e != nil {
			err = fmt.Errorf("unable to setup environment: %s", e)
			log.Error(err)
			return
		}
	}

	if err = createDevSymlinks(cfg.RootFs); err != nil {
		return
	}

	// TODO
	//  https://github.com/opencontainers/runc/blob/master/libcontainer/SPEC.md#runtime-and-init-process
	if err = setHostname(cfg.Hostname, path.Join(cfg.RootFs, "/etc/hostname")); err != nil {
		return fmt.Errorf("setting hostname: %s", err)
	}

	if err = syscall.Mknod(path.Join(cfg.RootFs, "/dev/null"), 1, 3); err != nil {
		if !os.IsExist(err) {
			return
		}
		err = nil
	}

	if err = createDevSymlinks(cfg.RootFs); err != nil {
		return
	}

	os.Clearenv()
	if err = setEnvVars(cfg.EnvVars); err != nil {
		return
	}

	if err = syscall.Chroot(cfg.RootFs); err != nil {
		return
	}

	if err = os.Chdir(cfg.Cwd); err != nil {
		return
	}

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
func Boot(configFd, logFd string) (err error) {

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

	cfg := Config{}
	if err = json.NewDecoder(configPipe).Decode(&cfg); err != nil {
		err = fmt.Errorf("reading config: %s\n", err)
		log.Error(err)
		return
	}

	// capture the env var before setting up the env since all env vars are deleted
	// in order to set up the box's env
	fifoFd := os.Getenv("BOX_FIFO_FD")

	err = setupEnv(cfg)
	if err != nil {
		err = fmt.Errorf("unable to setup environment: %s", err)
		log.Error(err)
		return
	}
	defer func() {
		cleanup()
	}()

	log.Debugf("Bootstrapping box %s: %s %v \n", cfg.Name, cfg.EntryPoint, cfg.EntryPointArgs)

	if fifoFd != "" {
		fd, e := strconv.Atoi(fifoFd)
		if e != nil {
			err = fmt.Errorf("unable to convert fifo fd: %s", e)
			log.Error(err)
			return
		}

		if err = syncParent(fd); err != nil {
			return
		}
	}

	err = syscall.Exec(
		cfg.EntryPoint,
		append([]string{path.Base(cfg.EntryPoint)}, cfg.EntryPointArgs...),
		os.Environ(),
	)
	log.Errorf("bootstrap: executing entry point: %s", err)

	return
}

func syncParent(fifoFd int) (err error) {
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
