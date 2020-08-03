`boxnet` is basically a wrapper around ```github.com/vishvananda/netlink``` to extend it by adding
the concept of `models`. Its current state is a bit of a mess mainly because I started writing it
without a clear idea of how it should look like. Basically a `model` represents a composite network
setup and aims to make it easy to configure common network setups. In the future may be the
network functionality should be decoupled from the runtime like Docker currently does.

### Supported interface types
* veth: it's a normal veth pair

### Network Models
To activate a model add a model object to the config. When a module config is present, all
interfaces are related to the module. Only one module is allowed per config/box:

```
"model": {
   "type": "model_name",
   "param1": "val1",
   ...
}
```

#### Supported models
* Bridge: connects a `box` to an external network by creating and attaching a `veth` master to a
  given bridge interface and moving the peer to the box NS. Example:

```
"model": {
   "type": "bridge",
   "bridge_name": "docker0"
}
```

#### Config file

Simple config without model
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

With model *bridge*
```
{
  "loopback_name": "lo",
  "model": {
    "type": "bridge",
    "bridge_name": "docker0"
  },
  "interfaces": [
    {
      "type": "veth",
      "name": "eth1",
      "peer_name": "eth2",
      "ip": "0.0.0.0/0",
      "peer_ip":  "172.17.0.6/28",
      "routes": [
        {
          "subnet": "0.0.0.0/0",
          "gateway": "172.17.0.1"
        }
      ]
    }
  ],
  "dns": {
    "nameservers": [
      "8.8.8.8"
    ],
    "domain": "lambda1",
    "search": [
      "lambda1",
      "lambda.local"
    ]
  }
}
```
