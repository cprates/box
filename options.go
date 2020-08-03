package box

import "github.com/cprates/box/boxnet"

type BoxOption func(*boxInternal)

// WithNetwork sets the netconf on a box.
func WithNetwork(netConf *boxnet.NetConf) BoxOption {
	return func(c *boxInternal) {
		c.config.NetConfig = netConf
	}
}
