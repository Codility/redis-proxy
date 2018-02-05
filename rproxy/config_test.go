package rproxy

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Codility/redis-proxy/fakeredis"
)

func TestConfigValidation(t *testing.T) {
	assertValid := func(c *Config) {
		errList := c.Prepare()
		if !errList.Ok() {
			t.Fatalf("Expected config to be valid: %s, %s",
				errList.Errors(), c.AsJSON())
		}
	}
	assertInvalid := func(c *Config, errors []string) {
		errList := c.Prepare()
		if errList.Ok() {
			t.Fatalf("Expected config to be invalid: %s",
				c.AsJSON())
		}

		expected := map[string]bool{}
		for _, e := range errors {
			expected[e] = true
		}
		got := map[string]bool{}
		for _, e := range errList.errors {
			got[e] = true
		}
		if !reflect.DeepEqual(expected, got) {
			expB, _ := json.MarshalIndent(expected, "", "  ")
			gotB, _ := json.MarshalIndent(got, "", "  ")
			t.Fatalf("Expected and received error lists are different:\n%v\n%v",
				string(expB), string(gotB))
		}
	}

	// prepare servers to connect to.
	srv_nonTLS := fakeredis.Start("fake", "tcp")
	defer srv_nonTLS.Stop()
	nonTLSAddr := srv_nonTLS.Addr().String()

	proxy_TLS := mustStartTestProxy(t, &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv_nonTLS.Addr().String()},
			Listen: AddrSpec{
				Addr:     "127.0.0.1:0",
				TLS:      true,
				CertFile: "../test_data/tls/server/cert.pem",
				KeyFile:  "../test_data/tls/server/key.pem",
			},
			Admin: AddrSpec{Addr: "127.0.0.1:0"},
		},
	})
	defer proxy_TLS.Stop()
	TLSAddr := strings.Replace(proxy_TLS.ListenAddr().String(), "127.0.0.1", "localhost", -1)

	// non-TLS configurations

	assertValid(&Config{
		Uplink: AddrSpec{Addr: nonTLSAddr},
		Listen: AddrSpec{Addr: "127.0.0.1:0"},
		Admin:  AddrSpec{Addr: "127.0.0.1:0"},
	})
	assertValid(&Config{
		Uplink:    AddrSpec{Addr: nonTLSAddr},
		Listen:    AddrSpec{Addr: "127.0.0.1:0"},
		ListenRaw: AddrSpec{Addr: "127.0.0.1:0"},
		Admin:     AddrSpec{Addr: "127.0.0.1:0"},
	})
	assertInvalid(&Config{},
		[]string{
			"Missing listen address",
			"Missing uplink address",
		})
	assertInvalid(&Config{
		Uplink:    AddrSpec{Addr: "127.0.0.1:0"},
		Listen:    AddrSpec{Addr: "127.0.0.1:0"},
		ListenRaw: AddrSpec{Addr: "127.0.0.1:0", Pass: "somepass"},
		Admin:     AddrSpec{Addr: "127.0.0.1:0"},
	}, []string{
		"could not connect to uplink: 127.0.0.1:0 (non-TLS)",
		"listen_raw does not support in-proxy authentication",
	})

	// TLS configurations

	assertValid(&Config{
		Uplink: AddrSpec{Addr: TLSAddr, TLS: true, CACertFile: "../test_data/tls/testca/cacert.pem"},
		Listen: AddrSpec{Addr: "127.0.0.1:0",
			TLS:      true,
			CertFile: "../test_data/tls/server/cert.pem",
			KeyFile:  "../test_data/tls/server/key.pem",
		},
		Admin: AddrSpec{Addr: "127.0.0.1:0",
			TLS:      true,
			CertFile: "../test_data/tls/server/cert.pem",
			KeyFile:  "../test_data/tls/server/key.pem",
		},
	})
	assertValid(&Config{
		Uplink: AddrSpec{Addr: TLSAddr, TLS: true, SkipVerify: true},
		Listen: AddrSpec{Addr: "127.0.0.1:0",
			TLS:      true,
			CertFile: "../test_data/tls/server/cert.pem",
			KeyFile:  "../test_data/tls/server/key.pem",
		},
		Admin: AddrSpec{Addr: "127.0.0.1:0",
			TLS:      true,
			CertFile: "../test_data/tls/server/cert.pem",
			KeyFile:  "../test_data/tls/server/key.pem",
		},
	})

	assertInvalid(&Config{
		Uplink: AddrSpec{Addr: nonTLSAddr,
			TLS:        true,
			CACertFile: "../test_data/tls/testca/cacert.pem",
		},
		Listen: AddrSpec{Addr: "127.0.0.1:0",
			TLS:      true,
			CertFile: "../test_data/tls/server/cert.pem",
			KeyFile:  "../test_data/tls/server/key.pem",
		},
		Admin: AddrSpec{Addr: "127.0.0.1:0",
			TLS:      true,
			CertFile: "../test_data/tls/server/cert.pem",
			KeyFile:  "../test_data/tls/server/key.pem",
		},
	}, []string{
		"could not connect to uplink: " + nonTLSAddr + " (TLS)",
	})
	assertInvalid(&Config{
		Uplink: AddrSpec{Addr: "127.0.0.1:0", TLS: true},
		Listen: AddrSpec{Addr: "127.0.0.1:0", TLS: true},
		Admin:  AddrSpec{Addr: "127.0.0.1:0", TLS: true},
	}, []string{
		"admin.tls requires certfile",
		"admin.tls requires keyfile",
		"listen.tls requires certfile",
		"listen.tls requires keyfile",
		"uplink.tls requires cacertfile or skipverify",
	})
	assertInvalid(&Config{
		Uplink: AddrSpec{Addr: nonTLSAddr,
			TLS:        true,
			CACertFile: "no-such-cacertfile",
		},
		Listen: AddrSpec{Addr: "127.0.0.1:0",
			TLS:      true,
			CertFile: "no-such-certfile",
			KeyFile:  "no-such-keyfile",
		},
		Admin: AddrSpec{Addr: "127.0.0.1:0",
			TLS:      true,
			CertFile: "no-such-certfile",
			KeyFile:  "no-such-keyfile",
		},
	}, []string{
		"could not load admin.certfile: no-such-certfile",
		"could not load admin.keyfile: no-such-keyfile",
		"could not load listen.certfile: no-such-certfile",
		"could not load listen.keyfile: no-such-keyfile",
		"could not load uplink.cacertfile: no-such-cacertfile",
	})
}

func TestAddrSpecEquality(t *testing.T) {

	assertEqual := func(a, b AddrSpec) {
		if a != b {
			t.Fatal("Expected AddrSpects to be equal: ", a.AsJSON(), b.AsJSON())
		}
	}

	assertNotEqual := func(a, b AddrSpec) {
		if a == b {
			t.Fatal("Expected AddrSpects *not* to be equal: ", a.AsJSON(), b.AsJSON())
		}
	}

	assertEqual(AddrSpec{Addr: "a", TLS: true, CACertFile: "ca"},
		AddrSpec{Addr: "a", TLS: true, CACertFile: "ca"})
	assertNotEqual(AddrSpec{Addr: "a", TLS: true, CACertFile: "ca"},
		AddrSpec{Addr: "a", TLS: true, CACertFile: "ca-changed"})
	assertNotEqual(AddrSpec{Addr: "a", TLS: true, CACertFile: "ca"},
		AddrSpec{Addr: "a"})

	assertEqual(AddrSpec{Addr: "a", Pass: "p"},
		AddrSpec{Addr: "a", Pass: "p"})
	assertNotEqual(AddrSpec{Addr: "a", Pass: "p"},
		AddrSpec{Addr: "a", Pass: "p-changed"})
}
