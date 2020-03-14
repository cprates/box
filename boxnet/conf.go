package boxnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// NetConf holds config for interfaces and DNS resolvers.
type NetConf struct {
	LoopbackName string                   `json:"loopback_name,omitempty"`
	Interfaces   []map[string]interface{} `json:"interfaces,omitempty"`
	DNS          DNSConf                  `json:"dns,omitempty"`
}

var ErrTypeNotDefined = errors.New("interface type not defined")

// VethConf holds a config of a single veth pair. Ip and PeerIp holds a CIDR format IP.
type VethConf struct {
	Type     string  `json:"type"`
	Name     string  `json:"name"`
	PeerName string  `json:"peer_name"`
	Ip       string  `json:"ip"`
	PeerIp   string  `json:"peer_ip"`
	Routes   []Route `json:"routes,omitempty"`
}

// Route config where Subnet must be in CIDR format.
type Route struct {
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

// DNSConf resolvers config.
type DNSConf struct {
	Nameservers []string `json:"nameservers,omitempty"`
	Domain      string   `json:"domain,omitempty"`
	Search      []string `json:"search,omitempty"`
}

func Load(path string) (*NetConf, error) {
	f, err := os.Open(path)
	if err != nil {
		err = fmt.Errorf("unable to open config file: %s", err)
		return nil, err
	}
	defer f.Close()

	conf := &NetConf{}
	err = json.NewDecoder(f).Decode(conf)

	return conf, err
}

func TypeFromConfig(conf map[string]interface{}) (string, error) {
	t, ok := conf["type"]
	if !ok {
		return "", ErrTypeNotDefined
	}

	tStr, ok := t.(string)
	if !ok {
		return "", fmt.Errorf("invalid type: %+v", conf)
	}

	return tStr, nil
}

func ConfigFromRawConfig(rawConf map[string]interface{}, dst interface{}) error {
	jsonConf, err := json.Marshal(rawConf)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonConf, dst)
}
