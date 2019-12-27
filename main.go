package main

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/cprates/box/runtime"
	"github.com/cprates/box/spec"
)

const (
	_ = iota
	idxAction
)

// go build ./main.go  && sudo ./main create
// go build ./main.go  && sudo ./main start
func main() {

	log.Infof("Running %+v", os.Args)

	log.SetLevel(log.DebugLevel)

	switch os.Args[idxAction] {
	case "create":
		config, err := spec.Load("config.json")
		if err != nil {
			log.Fatalln("Failed to load spec:", err)
		}

		wd, _ := os.Getwd()
		io := runtime.ProcessIO{
			In:  os.Stdin,
			Out: os.Stdout,
			Err: os.Stderr,
		}
		r := runtime.New("box1", wd, io, config)
		if err := r.Create(); err != nil {
			log.Error("Failed to create box:", err)
			os.Exit(-1)
		}
	case "start":
		// TODO: start should only need the container ID.. All the needed config should be stored
		//  in the state file
		config, err := spec.Load("config.json")
		if err != nil {
			log.Fatalln("Failed to load spec:", err)
		}

		wd, _ := os.Getwd()
		io := runtime.ProcessIO{
			In:  os.Stdin,
			Out: os.Stdout,
			Err: os.Stderr,
		}
		r := runtime.New("box1", wd, io, config)
		if err != nil {
			log.Error("Failed to start box:", err)
		}
		if err := r.Start(); err != nil {
			log.Error("Failed to start box:", err)
			os.Exit(-1)
		}
	case "bootstrap":
		log.Println("Bootstrapping box...")
		if err := runtime.Bootstrap(
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
