package runtime

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/cprates/box/system"
)

func (b *boxRuntime) exec() error {
	path := filepath.Join(b.workdir, b.childProcess.config.Name, execFifoFilename)

	fifoOpen := make(chan struct{})
	select {
	case <-awaitProcessExit(b.childProcess.pid, fifoOpen):
		return errors.New("box process is already dead")
	case result := <-awaitFifoOpen(path):
		close(fifoOpen)
		if result.err != nil {
			return result.err
		}
		f := result.file
		defer f.Close()
		if err := readFromExecFifo(f); err != nil {
			return err
		}
		return nil
	}
}

func readFromExecFifo(execFifo io.Reader) error {
	data, err := ioutil.ReadAll(execFifo)
	if err != nil {
		return err
	}
	if len(data) <= 0 {
		return fmt.Errorf("cannot start an already running box")
	}
	return nil
}

func awaitProcessExit(pid int, exit <-chan struct{}) <-chan struct{} {
	isDead := make(chan struct{})
	go func() {
		for {
			select {
			case <-exit:
				return
			case <-time.After(time.Millisecond * 100):
				stat, err := system.Stat(pid)
				if err != nil || stat.State == system.Zombie {
					close(isDead)
					return
				}
			}
		}
	}()
	return isDead
}

func awaitFifoOpen(path string) <-chan openResult {
	fifoOpened := make(chan openResult)
	go func() {
		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err != nil {
			fifoOpened <- openResult{err: fmt.Errorf("open exec fifo for reading: %s", err)}
			return
		}
		fifoOpened <- openResult{file: f}
	}()
	return fifoOpened
}

type openResult struct {
	file *os.File
	err  error
}
