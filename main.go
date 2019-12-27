package main

import (
	goos "os"

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

	log.Infof("Running %+v", goos.Args)

	log.SetLevel(log.DebugLevel)

	switch goos.Args[idxAction] {
	case "create":
		config, err := spec.Load("config.json")
		if err != nil {
			log.Fatalln("Failed to load spec:", err)
		}

		wd, _ := goos.Getwd()
		r := runtime.New("box1", wd, config)
		if err := r.Create(); err != nil {
			log.Error("Failed to create box:", err)
			goos.Exit(-1)
		}
	case "start":
		config, err := spec.Load("config.json")
		if err != nil {
			log.Fatalln("Failed to load spec:", err)
		}

		wd, _ := goos.Getwd()
		r := runtime.New("box1", wd, config)
		if err != nil {
			log.Error("Failed to start box:", err)
		}
		if err := r.Start(); err != nil {
			log.Error("Failed to start box:", err)
			goos.Exit(-1)
		}
	case "bootstrap":
		log.Println("Bootstrapping box...")
		if err := runtime.Bootstrap(
			goos.Getenv("BOX_BOOTSTRAP_CONFIG_FD"),
			goos.Getenv("BOX_BOOTSTRAP_LOG_FD"),
		); err != nil {
			goos.Exit(1)
		}
		panic("should never reach this far!")
	default:
		panic("help")
	}
}
