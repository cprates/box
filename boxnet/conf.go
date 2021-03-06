package boxnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// NetConf holds config for interfaces and DNS resolvers.
type NetConf struct {
	Model        map[string]interface{}   `json:"model,omitempty"`
	LoopbackName string                   `json:"loopback_name,omitempty"`
	Interfaces   []map[string]interface{} `json:"interfaces,omitempty"`
	DNS          DNSConf                  `json:"dns,omitempty"`
}

type Model struct {
	Type string `json:"type"`
}

type ModelBridge struct {
	BrName string `json:"bridge_name"`
}

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

var ErrTypeNotDefined = errors.New("interface type not defined")
var ErrModelNotDefined = errors.New("network model not defined")

func LoadFromFile(path string) (*NetConf, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	conf := &NetConf{}
	err = json.NewDecoder(f).Decode(conf)

	return conf, err
}

func Load(rd io.Reader) (*NetConf, error) {
	conf := &NetConf{}
	return conf, json.NewDecoder(rd).Decode(conf)
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

func ModelFromConfig(conf map[string]interface{}) (string, error) {
	t, ok := conf["type"]
	if !ok {
		return "", ErrModelNotDefined
	}

	tStr, ok := t.(string)
	if !ok {
		return "", fmt.Errorf("invalid model: %+v", conf)
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
