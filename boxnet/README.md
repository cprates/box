This is more like a wrapper on ```github.com/vishvananda/netlink``` to add a set of extra
functionalities and supports Linux only. 

#### Supported interfaces
* veth

The example bellow creates a new veth pair with the given address, move the peer interface to the
given namespace and finally set both interfaces up.

#### Config file

```
{
  "loopback_name": "lo",
  "interfaces": [
    {
      "type": "veth",
      "name": "eth1",
      "peer_name": "eth2",
      "ip": "10.0.0.1/30",
      "peer_ip":  "10.0.0.2/30",
      "routes": [
        {
          "subnet": "0.0.0.0/0",
          "gateway": "10.0.0.1"
        }
      ]
    }
  ],
  "dns": {
    "nameservers": [
      "10.0.0.1"
    ],
    "domain": "lambda1",
    "search": [
      "lambda1",
      "lambda.local"
    ]
  }
}

```

#### How to run

```go run main.go $NSPID```

```go
package main

import (
	"net"
	"os"
	"strconv"

	"github.com/cprates/box/boxnet"
)

func main() {
	pidStr := os.Args[1]
	iface, err := boxnet.NewVeth("eth1", "eth2")
	if err != nil {
		panic(err)
	}
	pid, _ := strconv.Atoi(pidStr)
	err = iface.SetPeerNsByPid(pid)
	if err != nil {
		panic(err)
	}

	ip := net.IPv4(10, 0, 0, 1)
	mask := net.IPv4Mask(255, 255, 255, 252)
	err = iface.SetAddr(net.IPNet{IP: ip, Mask: mask})
	if err != nil {
		panic(err)
	}

	peerIP := net.IPv4(10, 0, 0, 2)
	err = boxnet.ExecuteOnNs(
		pid,
		func() {
			if e := iface.SetPeerAddr(net.IPNet{IP: peerIP, Mask: mask}); e != nil {
				panic(e)
			}

		},
	)
	if err != nil {
		panic(err)
	}

	err = iface.Up()
	if err != nil {
		panic(err)
	}

	err = boxnet.ExecuteOnNs(
		pid,
		func() {
			if e := iface.PeerUp(); e != nil {
				panic(e)
			}

		},
	)
	if err != nil {
		panic(err)
	}

}
```

