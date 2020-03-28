This is more like a wrapper on ```github.com/vishvananda/netlink``` to add a set of extra
functionalities and supports Linux only. 

#### Supported interface types
* veth

#### Network Models
To activate a model add a model object to the config. When a module config is present, all
interfaces are related to the module. Only one module is allowed per config.:

```
"model": {
   "type": "bridge"
}
```

#####Supported models
* Bridge: TODO

#### Examples
TODO

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
