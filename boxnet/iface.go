package boxnet

import (
	"github.com/vishvananda/netns"
	"runtime"
)

type IFacer interface {
	Down() error
	Up() error
	Type() string
}

func ExecuteOnNs(pidns int, f func()) (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := netns.Get()
	if err != nil {
		return
	}
	defer func() {
		if e := origns.Close(); err == nil && e != nil {
			err = e
			return
		}
	}()

	targetNs, err := netns.GetFromPid(pidns)
	if err != nil {
		return
	}

	err = netns.Set(targetNs)
	if err != nil {
		return
	}
	defer func() {
		if e := targetNs.Close(); err == nil && e != nil {
			err = e
			return
		}
	}()

	f()

	err = netns.Set(origns)
	return
}
