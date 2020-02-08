package main

import (
	"os"

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
		_, err = c.CreateBox(os.Args[2], io, spec)
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

		wd, _ := os.Getwd()
		io := ProcessIO{
			In:  os.Stdin,
			Out: os.Stdout,
			Err: os.Stderr,
		}

		c := New(wd)
		if err := c.RunBox(os.Args[2], io, spec); err != nil {
			log.Fatalln("Failed to run box:", err)
		}
	case "bootstrap":
		log.Println("Bootstrapping box...")
		if err := Bootstrap(
			os.Getenv("BOX_BOOTSTRAP_CONFIG_FD"),
			os.Getenv("BOX_BOOTSTRAP_LOG_FD"),
		); err != nil {
			os.Exit(1)
		}
		panic("should never reach this far!")
	default:
		panic("help")
	}
}
