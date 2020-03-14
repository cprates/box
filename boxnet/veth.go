package boxnet

import (
	"net"
	"runtime"

	"github.com/vishvananda/netlink"
)

var _ Vether = (*veth)(nil)

type Vether interface {
	IFacer
	PeerDown() error
	PeerUp() error
	SetAddr(addr net.IPNet) error
	SetPeerAddr(addr net.IPNet) error
	SetPeerNsByPid(nspid int) error
}

type veth struct {
	link netlink.Veth
}

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
		return nil, err
	}

	err = iface.SetPeerNsByPid(nsPID)
	if err != nil {
		return nil, err
	}

	ip, netIP, err := net.ParseCIDR(conf.Ip)
	if err != nil {
		return nil, err
	}

	err = iface.SetAddr(net.IPNet{IP: ip, Mask: netIP.Mask})
	if err != nil {
		return nil, err
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
		return nil, err
	}

	err = iface.Up()
	if err != nil {
		return nil, err
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
