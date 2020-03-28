package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/cprates/box"
	"github.com/cprates/box/bootstrap"
	"github.com/cprates/box/boxnet"
	"github.com/cprates/box/spec"

	log "github.com/sirupsen/logrus"
)

const (
	actionIdx = iota
	boxNameIdx
)

var defaultIO = box.ProcessIO{
	In:  os.Stdin,
	Out: os.Stdout,
	Err: os.Stderr,
}

var (
	configFile  string
	netconfFile string
	workdir     string
)

func init() {
	wd, _ := os.Getwd()

	flag.StringVar(&configFile, "spec", "config.json", "Path to the spec file")
	flag.StringVar(&netconfFile, "netconf", "netconf.json", "Path to the file with network config")
	flag.StringVar(&workdir, "workdir", wd, "working dir where to store created boxes")

	log.StandardLogger().SetNoLock()
	if os.Getenv("BOX_DEBUG") == "1" {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.SetReportCaller(true)
	log.SetFormatter(
		&log.TextFormatter{
			DisableLevelTruncation: true,
			FullTimestamp:          true,
			CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
				_, fileName := filepath.Split(frame.File)
				file = " " + fileName + ":" + strconv.Itoa(frame.Line) + " #"
				return
			},
		},
	)
}

func printHelp() {
	fmt.Println("Usage: box [-flags] {create|start|run|destroy} boxname\nFlags:")
	flag.PrintDefaults()
}

func main() {
	flag.Parse()

	if len(flag.Args()) < 2 && flag.Args()[actionIdx] != "bootstrap" {
		printHelp()
		os.Exit(1)
	}

	switch flag.Args()[actionIdx] {
	case "create":
		sp, err := spec.Load(configFile)
		if err != nil {
			log.Fatalln("Failed to load spec:", err)
		}

		c := box.New(workdir)
		_, err = c.CreateBox(flag.Args()[boxNameIdx], defaultIO, sp)
		if err != nil {
			log.Fatalln("Failed to create box: ", err)
		}
	case "start":
		c := box.New(workdir)
		b, err := c.LoadBox(flag.Args()[boxNameIdx], defaultIO)
		if err != nil {
			log.Fatalln("Failed to load box:", err)
		}
		if err := b.Start(); err != nil {
			log.Fatalln("Failed to start box:", err)
		}
	case "run":
		sp, err := spec.Load(configFile)
		if err != nil {
			log.Fatalln("Failed to load spec:", err)
		}

		netConf, err := boxnet.Load(netconfFile)
		if err != nil {
			log.Fatalln("Failed to load netconf:", err)
		}

		c := box.New(workdir)
		err = c.RunBox(flag.Args()[boxNameIdx], defaultIO, sp, box.WithNetwork(netConf))
		if err != nil {
			log.Fatalln("Failed to run box:", err)
		}
	case "destroy":
		c := box.New(workdir)
		err := c.DestroyBox(flag.Args()[boxNameIdx])
		if err != nil {
			log.Fatalln("Failed to destroy box:", err)
		}
	case "bootstrap":
		log.Debugln("Bootstrapping box...")
		if err := bootstrap.Boot(
			os.Getenv("BOX_BOOTSTRAP_CONFIG_FD"),
			os.Getenv("BOX_BOOTSTRAP_LOG_FD"),
		); err != nil {
			os.Exit(1)
		}
		panic("should never reach this far!")
	default:
		printHelp()
		os.Exit(1)
	}
}
