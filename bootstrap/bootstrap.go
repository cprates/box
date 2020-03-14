package bootstrap

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/cprates/box/boxnet"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
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
	NetConfig      *boxnet.NetConf `json:"NetConfig,omitempty"`
}

func options(cfg Config) (opts []Option) {
	opts = append(
		opts,
		DefaultMounts(cfg.RootFs)...,
	)

	opts = append(
		opts,
		DefaultNodeDevs(cfg.RootFs)...,
	)

	return
}

func setupEnv(cfg Config) (err error) {
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
	//  Still need localtime
	if err = setHostname(cfg.Hostname, path.Join(cfg.RootFs, "/etc/hostname")); err != nil {
		return fmt.Errorf("setting hostname: %s", err)
	}

	resolvF, err := os.OpenFile(path.Join(
		cfg.RootFs, "/etc/resolv.conf"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0665,
	)
	if err != nil {
		return err
	}
	defer resolvF.Close()
	hostsF, err := os.OpenFile(path.Join(
		cfg.RootFs, "/etc/hosts"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0665,
	)
	if err != nil {
		return err
	}
	defer hostsF.Close()
	if cfg.NetConfig != nil {
		if err = setLoopbackUp(cfg.NetConfig.LoopbackName); err != nil {
			return fmt.Errorf("setting loopback interface up: %s", err)
		}

		if err = setDNS(resolvF, cfg.NetConfig.DNS); err != nil {
			return fmt.Errorf("setting dns configs: %s", err)
		}

		if err = setHosts(hostsF, cfg.NetConfig.DNS); err != nil {
			return fmt.Errorf("setting hosts: %s", err)
		}
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

func setDNS(f io.Writer, cfg boxnet.DNSConf) error {
	if cfg.Domain != "" {
		if _, err := fmt.Fprintf(f, "domain %s\n", cfg.Domain); err != nil {
			return err
		}
	}

	if len(cfg.Search) > 0 {
		if _, err := fmt.Fprintf(f, "search %s\n", strings.Join(cfg.Search, " ")); err != nil {
			return err
		}
	}

	for _, server := range cfg.Nameservers {
		if _, err := fmt.Fprintf(f, "nameserver %s\n", server); err != nil {
			return err
		}
	}

	return nil
}

func setHosts(f io.Writer, cfg boxnet.DNSConf) error {
	// for now just configure localhost

	ipv4 := "127.0.0.1 localhost"
	ipv6 := "::1 localhost"
	if cfg.Domain != "" {
		ipv4 += " localhost." + cfg.Domain
		ipv6 += " localhost." + cfg.Domain
	}

	_, err := fmt.Fprintf(f, strings.Join([]string{ipv4, ipv6}, "\n")+"\n")
	return err
}

func setLoopbackUp(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("unable to find interface %q: %s", name, err)
	}

	if err = netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("unable to set interface %q up: %s", name, err)
	}

	return nil
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
