package main

import (
	"github.com/cprates/box/boxnet"
	"net"
	"os"
	"strconv"

	"github.com/cprates/box/bootstrap"
	"github.com/cprates/box/spec"

	log "github.com/sirupsen/logrus"
)

const (
	_ = iota
	idxAction
)

// make && sudo ./box create box1
// make && sudo ./box start box1
func main() {

	log.Infof("Running %+v", os.Args)

	log.SetLevel(log.DebugLevel)

	switch os.Args[idxAction] {
	case "create":
		spec, err := spec.Load("config.json")
		if err != nil {
			log.Fatalln("Failed to load spec:", err)
		}

		wd, _ := os.Getwd()
		io := ProcessIO{
			In:  os.Stdin,
			Out: os.Stdout,
			Err: os.Stderr,
		}

		c := New(wd)
		_, err = c.CreateBox(os.Args[2], io, spec, nil)
		if err != nil {
			log.Error("Failed to create box:", err)
			os.Exit(-1)
		}
	case "start":
		wd, _ := os.Getwd()
		io := ProcessIO{
			In:  os.Stdin,
			Out: os.Stdout,
			Err: os.Stderr,
		}

		c := New(wd)
		box, err := c.LoadBox(os.Args[2], io)
		if err != nil {
			log.Fatalln("Failed to load box:", err)
		}
		if err := box.Start(); err != nil {
			log.Fatalln("Failed to start box:", err)
		}
	case "run":
		spec, err := spec.Load("config.json")
		if err != nil {
			log.Fatalln("Failed to load spec:", err)
		}

		netConf, err := boxnet.Load("netconf.json")
		if err != nil {
			log.Fatalln("Failed to load spec:", err)
		}

		wd, _ := os.Getwd()
		io := ProcessIO{
			In:  os.Stdin,
			Out: os.Stdout,
			Err: os.Stderr,
		}

		c := New(wd)
		if err := c.RunBox(os.Args[2], io, spec, WithNetwork(netConf)); err != nil {
			log.Fatalln("Failed to run box:", err)
		}
	case "bootstrap":
		log.Println("Bootstrapping box...")
		if err := bootstrap.Boot(
			os.Getenv("BOX_BOOTSTRAP_CONFIG_FD"),
			os.Getenv("BOX_BOOTSTRAP_LOG_FD"),
		); err != nil {
			os.Exit(1)
		}
		panic("should never reach this far!")
	case "iface":
		pidStr := os.Args[idxAction+1]
		iface, err := boxnet.NewVeth("eth1", "eth2")
		if err != nil {
			panic(err)
		}
		pid, _ := strconv.Atoi(pidStr)
		err = iface.SetPeerNsByPid(pid)
		if err != nil {
			panic(err)
		}

		ip := net.IPv4(10, 0, 0, 1)
		mask := net.IPv4Mask(255, 255, 255, 252)
		err = iface.SetAddr(net.IPNet{IP: ip, Mask: mask})
		if err != nil {
			panic(err)
		}

		peerIP := net.IPv4(10, 0, 0, 2)
		err = boxnet.ExecuteOnNs(
			pid,
			func() {
				if e := iface.SetPeerAddr(net.IPNet{IP: peerIP, Mask: mask}); e != nil {
					panic(e)
				}
			},
		)
		if err != nil {
			panic(err)
		}

		err = iface.Up()
		if err != nil {
			panic(err)
		}

		err = boxnet.ExecuteOnNs(
			pid,
			func() {
				if e := iface.PeerUp(); e != nil {
					panic(e)
				}

			},
		)
		if err != nil {
			panic(err)
		}

	default:
		panic("help")
	}
}
