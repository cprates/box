package box

import "github.com/cprates/box/boxnet"

type BoxOption func(*cartonBox)

func WithNetwork(netConf *boxnet.NetConf) BoxOption {
	return func(c *cartonBox) {
		c.config.NetConfig = netConf
	}
}
