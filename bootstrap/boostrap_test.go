package bootstrap

import (
	"bytes"
	"github.com/cprates/box/boxnet"
	"testing"
)

func TestSetDNS(t *testing.T) {
	buf := bytes.Buffer{}

	cfg := boxnet.DNSConf{
		Domain:      "domain1",
		Search:      []string{"search1", "search2"},
		Nameservers: []string{"server1", "server2"},
	}
	err := setDNS(&buf, cfg)
	if err != nil {
		t.Error(err)
	}

	l, err := buf.ReadString('\n')
	if err != nil {
		t.Error(err)
	}
	expects := "domain domain1\n"
	if l != expects {
		t.Errorf("domain check failed. Expects %q, got %q", expects, l)
	}

	l, err = buf.ReadString('\n')
	if err != nil {
		t.Error(err)
	}
	expects = "search search1 search2\n"
	if l != expects {
		t.Errorf("search check failed. Expects %q, got %q", expects, l)
	}

	l, err = buf.ReadString('\n')
	if err != nil {
		t.Error(err)
	}
	l2, err := buf.ReadString('\n')
	if err != nil {
		t.Error(err)
	}
	expects = "nameserver server1\nnameserver server2\n"
	if l+l2 != expects {
		t.Errorf("nameserver check failed. Expects %q, got %q", expects, l+l2)
	}
}

func TestSetHostsWithDomain(t *testing.T) {
	buf := bytes.Buffer{}

	cfg := boxnet.DNSConf{
		Domain: "domain1",
	}
	err := setHosts(&buf, cfg)
	if err != nil {
		t.Error(err)
	}

	l, err := buf.ReadString('\n')
	if err != nil {
		t.Error(err)
	}
	expects := "127.0.0.1 localhost localhost." + cfg.Domain + "\n"
	if l != expects {
		t.Errorf("ipv4 host check failed. Expects %q, got %q", expects, l)
	}

	l, err = buf.ReadString('\n')
	if err != nil {
		t.Error(err)
	}
	expects = "::1 localhost localhost." + cfg.Domain + "\n"
	if l != expects {
		t.Errorf("ipv6 host check failed. Expects %q, got %q", expects, l)
	}
}

func TestSetHostsWithoutDomain(t *testing.T) {
	buf := bytes.Buffer{}

	err := setHosts(&buf, boxnet.DNSConf{})
	if err != nil {
		t.Error(err)
	}

	l, err := buf.ReadString('\n')
	if err != nil {
		t.Error(err)
	}
	expects := "127.0.0.1 localhost\n"
	if l != expects {
		t.Errorf("ipv4 host check failed. Expects %q, got %q", expects, l)
	}

	l, err = buf.ReadString('\n')
	if err != nil {
		t.Error(err)
	}
	expects = "::1 localhost\n"
	if l != expects {
		t.Errorf("ipv6 host check failed. Expects %q, got %q", expects, l)
	}
}
