package rproxy

import (
	"fmt"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stvp/assert"
	"gitlab.codility.net/marcink/redis-proxy/fakeredis"
)

func TestConfigValidation(t *testing.T) {
	assertValid := func(c *Config) {
		errList := c.Prepare()
		assert.Equal(t, []string{}, errList.Errors(),
			fmt.Sprintf("%s\n%s", errList.Errors(), debug.Stack()))
	}
	assertInvalid := func(c *Config, errors []string) {
		errList := c.Prepare()
		assert.Equal(t, errors, errList.Errors(), string(debug.Stack()))
	}

	// prepare servers to connect to.
	srv_nonTLS := fakeredis.Start("fake")
	defer srv_nonTLS.Stop()
	nonTLSAddr := srv_nonTLS.Addr().String()

	proxy_TLS := mustStartTestProxy(t, &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv_nonTLS.Addr().String()},
			Listen: AddrSpec{
				Addr: "127.0.0.1:0",
				TLS: &TLSSpec{
					CertFile: "../test_data/tls/server/cert.pem",
					KeyFile:  "../test_data/tls/server/key.pem",
				}},
			Admin: AddrSpec{Addr: "127.0.0.1:0"},
		},
	})
	defer proxy_TLS.controller.Stop()
	TLSAddr := strings.Replace(proxy_TLS.ListenAddr().String(), "127.0.0.1", "localhost", -1)

	// non-TLS configurations

	assertValid(&Config{
		Uplink: AddrSpec{Addr: nonTLSAddr},
		Listen: AddrSpec{Addr: "127.0.0.1:0"},
		Admin:  AddrSpec{Addr: "127.0.0.1:0"},
	})
	assertInvalid(&Config{},
		[]string{
			"Missing admin address",
			"Missing listen address",
			"Missing uplink address",
		})
	assertInvalid(&Config{
		Uplink: AddrSpec{Addr: "127.0.0.1:0"},
		Listen: AddrSpec{Addr: "127.0.0.1:0"},
		Admin:  AddrSpec{Addr: "127.0.0.1:0"},
	}, []string{
		"could not connect to uplink: 127.0.0.1:0 (non-TLS)",
	})

	// TLS configurations

	assertValid(&Config{
		Uplink: AddrSpec{Addr: TLSAddr, TLS: &TLSSpec{
			CACertFile: "../test_data/tls/testca/cacert.pem",
		}},
		Listen: AddrSpec{Addr: "127.0.0.1:0", TLS: &TLSSpec{
			CertFile: "../test_data/tls/server/cert.pem",
			KeyFile:  "../test_data/tls/server/key.pem",
		}},
		Admin: AddrSpec{Addr: "127.0.0.1:0", TLS: &TLSSpec{
			CertFile: "../test_data/tls/server/cert.pem",
			KeyFile:  "../test_data/tls/server/key.pem",
		}},
	})
	assertInvalid(&Config{
		Uplink: AddrSpec{Addr: nonTLSAddr, TLS: &TLSSpec{
			CACertFile: "../test_data/tls/testca/cacert.pem",
		}},
		Listen: AddrSpec{Addr: "127.0.0.1:0", TLS: &TLSSpec{
			CertFile: "../test_data/tls/server/cert.pem",
			KeyFile:  "../test_data/tls/server/key.pem",
		}},
		Admin: AddrSpec{Addr: "127.0.0.1:0", TLS: &TLSSpec{
			CertFile: "../test_data/tls/server/cert.pem",
			KeyFile:  "../test_data/tls/server/key.pem",
		}},
	}, []string{
		"could not connect to uplink: " + nonTLSAddr + " (TLS)",
	})
	assertInvalid(&Config{
		Uplink: AddrSpec{Addr: "127.0.0.1:0", TLS: &TLSSpec{}},
		Listen: AddrSpec{Addr: "127.0.0.1:0", TLS: &TLSSpec{}},
		Admin:  AddrSpec{Addr: "127.0.0.1:0", TLS: &TLSSpec{}},
	}, []string{
		"admin.tls requires certfile",
		"admin.tls requires keyfile",
		"listen.tls requires certfile",
		"listen.tls requires keyfile",
		"uplink.tls requires cacertfile",
	})
	assertInvalid(&Config{
		Uplink: AddrSpec{Addr: nonTLSAddr, TLS: &TLSSpec{
			CACertFile: "no-such-cacertfile",
		}},
		Listen: AddrSpec{Addr: "127.0.0.1:0", TLS: &TLSSpec{
			CertFile: "no-such-certfile",
			KeyFile:  "no-such-keyfile",
		}},
		Admin: AddrSpec{Addr: "127.0.0.1:0", TLS: &TLSSpec{
			CertFile: "no-such-certfile",
			KeyFile:  "no-such-keyfile",
		}},
	}, []string{
		"could not load admin.tls.certfile: no-such-certfile",
		"could not load admin.tls.keyfile: no-such-keyfile",
		"could not load listen.tls.certfile: no-such-certfile",
		"could not load listen.tls.keyfile: no-such-keyfile",
		"could not load uplink.tls.cacertfile: no-such-cacertfile",
	})
}

func TestAddrSpecEquality(t *testing.T) {

	assertEqual := func(a, b AddrSpec) {
		if !a.Equal(&b) {
			t.Fatal("Expected AddrSpects to be equal: ", a.AsJSON(), b.AsJSON())
		}
	}

	assertNotEqual := func(a, b AddrSpec) {
		if a.Equal(&b) {
			t.Fatal("Expected AddrSpects *not* to be equal: ", a.AsJSON(), b.AsJSON())
		}
	}

	assertEqual(AddrSpec{Addr: "a", TLS: &TLSSpec{CACertFile: "ca"}},
		AddrSpec{Addr: "a", TLS: &TLSSpec{CACertFile: "ca"}})
	assertNotEqual(AddrSpec{Addr: "a", TLS: &TLSSpec{CACertFile: "ca"}},
		AddrSpec{Addr: "a", TLS: &TLSSpec{CACertFile: "ca-changed"}})
	assertNotEqual(AddrSpec{Addr: "a", TLS: &TLSSpec{CACertFile: "ca"}},
		AddrSpec{Addr: "a"})

	assertEqual(AddrSpec{Addr: "a", Pass: "p"},
		AddrSpec{Addr: "a", Pass: "p"})
	assertNotEqual(AddrSpec{Addr: "a", Pass: "p"},
		AddrSpec{Addr: "a", Pass: "p-changed"})
}
