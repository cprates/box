package main

import (
	goos "os"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/cprates/box/runtime"
)

const (
	_ = iota
	idxAction
)

// go build ./main.go  && sudo ./main create
// go build ./main.go  && sudo ./main start {PID}
func main() {

	log.Infof("Running %+v", goos.Args)

	log.SetLevel(log.DebugLevel)

	switch goos.Args[idxAction] {
	case "create":
		wd, _ := goos.Getwd()
		r := runtime.New(wd)
		if err := r.Create(); err != nil {
			log.Error("Failed to create box:", err)
			goos.Exit(-1)
		}
	case "start":
		wd, _ := goos.Getwd()
		r := runtime.New(wd)
		pid, err := strconv.Atoi(goos.Args[2])
		if err != nil {
			log.Error("Failed to start box:", err)
		}
		if err := r.Start(pid); err != nil {
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
