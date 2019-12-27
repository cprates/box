package runtime

import "os"

type ProcessIO struct {
	In  *os.File
	Out *os.File
	Err *os.File
}
