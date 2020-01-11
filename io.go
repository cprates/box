package main

import "os"

// ProcessIO is used to pass to the runtime the communication channels.
type ProcessIO struct {
	In  *os.File
	Out *os.File
	Err *os.File
}
