package runtime

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	log "github.com/sirupsen/logrus"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "bootstrap" {
		// TODO: do I really need this? Or When?
		//boxRuntime.GOMAXPROCS(1)
		//boxRuntime.LockOSThread()

		setupLog()
		log.Debug("Starting bootstrap process...")
	}
}

func setupLog() {
	// TODO: the logger output must be the logFd file

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
