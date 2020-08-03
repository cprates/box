package boxnet

import (
	"fmt"
	"net"
	"runtime"

	"github.com/vishvananda/netlink"
)

type Vether interface {
	IFacer
	PeerDown() error
	PeerUp() error
	SetAddr(addr net.IPNet) error
	SetPeerAddr(addr net.IPNet) error
	SetPeerNsByPid(nspid int) error
	SetRoutes(routes []Route) error
}

type veth struct {
	link    netlink.Veth
	peerIdx int
}

var _ Vether = (*veth)(nil)

func NewVeth(name, peerName string) (Vether, error) {
	la := netlink.NewLinkAttrs()
	la.Name = name

	link := netlink.Veth{
		LinkAttrs: la,
		PeerName:  peerName,
	}

	err := netlink.LinkAdd(&link)
	if err != nil {
		return nil, err
	}

	return veth{link: link}, nil
}

func VethFromConfig(conf VethConf, nsPID int) (Vether, error) {
	iface, err := NewVeth(conf.Name, conf.PeerName)
	if err != nil {
		return nil, fmt.Errorf("unable to create new veth: %s", err)
	}

	peerLink, err := netlink.LinkByName(conf.PeerName)
	if err != nil {
		return nil, fmt.Errorf("unable to get link by name %q: %s", conf.PeerName, err)
	}
	pl := iface.(veth)
	pl.peerIdx = peerLink.Attrs().Index

	err = iface.SetPeerNsByPid(nsPID)
	if err != nil {
		// if the link gets created and we fail to move it to the correct NS, delete the link.
		// This only needs to be done manually here, because as soon as the peer iface is attached
		// to the correct NS, the veth is deleted by the kernel when the process dies
		_ = netlink.LinkDel(peerLink)
		return nil, fmt.Errorf("unable to move peer iface to ns %d: %s", nsPID, err)
	}

	ip, netIP, err := net.ParseCIDR(conf.Ip)
	if err != nil {
		return nil, fmt.Errorf("unable to parse configured IP %q: %s", conf.Ip, err)
	}

	err = iface.SetAddr(net.IPNet{IP: ip, Mask: netIP.Mask})
	if err != nil {
		return nil, fmt.Errorf("unable to set peer iface addr: %s", err)
	}

	peerIP, peerNetIP, err := net.ParseCIDR(conf.PeerIp)
	err = ExecuteOnNs(
		nsPID,
		func() {
			if e := iface.SetPeerAddr(net.IPNet{IP: peerIP, Mask: peerNetIP.Mask}); e != nil {
				panic(e)
			}

		},
	)
	if err != nil {
		return nil, fmt.Errorf("unable to set peer address: %s", err)
	}

	err = iface.Up()
	if err != nil {
		return nil, fmt.Errorf("unable to set iface up: %s", err)
	}

	err = ExecuteOnNs(
		nsPID,
		func() {
			if e := iface.PeerUp(); e != nil {
				panic(e)
			}

		},
	)
	if err != nil {
		return nil, fmt.Errorf("unable to set peer iface up: %s", err)
	}

	return iface, nil
}

func AttachVeth(cfg VethConf, nsPID int) (Vether, error) {
	iface, err := VethFromConfig(cfg, nsPID)
	if err != nil {
		return nil, fmt.Errorf("setting up iface %q: %s", cfg.Name, err)
	}

	if err = iface.Up(); err != nil {
		return nil, fmt.Errorf("setting iface up %q: %s", cfg.Name, err)
	}

	errC := make(chan error, 1)
	err = ExecuteOnNs(nsPID, func() {
		if err = iface.PeerUp(); err != nil {
			errC <- fmt.Errorf("setting peer interface up %q: %s", cfg.PeerName, err)
			return
		}
		errC <- nil
	})
	if err != nil {
		return nil, fmt.Errorf(
			"entering box NS to set peer interface up %q: %s", cfg.PeerName, err,
		)
	}
	if err = <-errC; err != nil {
		return nil, err
	}

	err = ExecuteOnNs(nsPID,
		func() {
			err = iface.SetRoutes(cfg.Routes)
			if err != nil {
				errC <- fmt.Errorf("configuring routes for iface %q: %s", cfg.PeerName, err)
				return
			}
			errC <- nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"entering box NS to setup peer routes %q: %s", cfg.PeerName, err,
		)
	}
	if err = <-errC; err != nil {
		return nil, err
	}

	return iface, nil
}

func (v veth) Down() error {
	return netlink.LinkSetDown(&v.link)
}

func (v veth) Up() error {
	return netlink.LinkSetUp(&v.link)
}

func (v veth) SetMaster(master netlink.Link) error {
	return netlink.LinkSetMaster(&v.link, master)
}

func (v veth) PeerDown() error {
	peerLink, err := netlink.LinkByName(v.link.PeerName)
	if err != nil {
		return err
	}

	return netlink.LinkSetDown(peerLink)
}

func (v veth) PeerUp() error {
	peerLink, err := netlink.LinkByName(v.link.PeerName)
	if err != nil {
		return err
	}

	return netlink.LinkSetUp(peerLink)
}

func (v veth) Type() string {
	return v.link.Type()
}

func (v veth) SetAddr(addr net.IPNet) error {
	a, err := netlink.ParseAddr(addr.String())
	if err != nil {
		return err
	}

	return netlink.AddrAdd(&v.link, a)
}

func (v veth) SetPeerAddr(addr net.IPNet) error {
	a, err := netlink.ParseAddr(addr.String())
	if err != nil {
		return err
	}

	peerLink, err := netlink.LinkByName(v.link.PeerName)
	if err != nil {
		return err
	}

	return netlink.AddrAdd(peerLink, a)
}

func (v veth) SetPeerNsByPid(nspid int) (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	peerLink, err := netlink.LinkByName(v.link.PeerName)
	if err != nil {
		return
	}

	err = netlink.LinkSetNsPid(peerLink, nspid)
	if err != nil {
		return
	}

	return nil
}

func (v veth) SetRoutes(routes []Route) error {
	for _, route := range routes {
		_, dst, err := net.ParseCIDR(route.Subnet)
		if err != nil {
			return fmt.Errorf("parsing route subnet %+v: %s", route.Subnet, err)
		}
		gw := net.ParseIP(route.Gateway)

		err = netlink.RouteAdd(
			&netlink.Route{
				LinkIndex: v.peerIdx,
				Dst:       dst,
				Gw:        gw,
			},
		)
		if err != nil {
			return err
		}
	}

	return nil
}
