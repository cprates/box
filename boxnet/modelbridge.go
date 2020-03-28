package boxnet

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

type Bridger interface {
}

type bridgeModel struct {
	BrName string
	NsPID  int
	IFaces []IFacer
}

var _ Bridger = (*bridgeModel)(nil)

func NewBridgeModel(brName string, nsPID int, ifsConfig []map[string]interface{}) (Bridger, error) {
	brLink, err := netlink.LinkByName(brName)
	if err != nil {
		return nil, fmt.Errorf("unable to get bridge interface %q: %s", brName, err)
	}

	var ifaces []IFacer
	for _, rawConf := range ifsConfig {
		t, err := TypeFromConfig(rawConf)
		if err != nil {
			return nil, fmt.Errorf("unable to get iface type: %s", err)
		}

		var iFace IFacer
		switch t {
		case "veth":
			cfg := VethConf{}
			err := ConfigFromRawConfig(rawConf, &cfg)
			if err != nil {
				return nil, fmt.Errorf("parsing iface config: %+v ** %s", rawConf, err)
			}

			iFace, err = AttachVeth(cfg, nsPID)
			err = iFace.SetMaster(brLink)
			if err != nil {
				return nil, fmt.Errorf("unable to set master to %q: %s", cfg.Name, err)
			}
		default:
			return nil, fmt.Errorf("unsupported iface type: %s", t)
		}

		ifaces = append(ifaces, iFace)
	}

	return &bridgeModel{
		BrName: brName,
		NsPID:  nsPID,
		IFaces: ifaces,
	}, nil
}
