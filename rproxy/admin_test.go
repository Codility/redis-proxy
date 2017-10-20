package rproxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/Codility/redis-proxy/fakeredis"
	"github.com/stvp/assert"
)

func TestProxyAdminNonTLS(t *testing.T) {
	srv := fakeredis.Start("srv", "tcp")
	defer srv.Stop()

	conf := &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	}
	proxy := mustStartTestProxy(t, conf)
	defer proxy.Stop()

	res, err := http.Get(fmt.Sprintf("http://%s/", proxy.AdminAddr().String()))
	assert.Nil(t, err)
	assert.Equal(t, res.StatusCode, 200)
}

func TestProxyAdminTLS(t *testing.T) {
	srv := fakeredis.Start("srv", "tcp")
	defer srv.Stop()

	conf := &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin: AddrSpec{
				Addr:     "127.0.0.1:0",
				TLS:      true,
				CertFile: "../test_data/tls/server/cert.pem",
				KeyFile:  "../test_data/tls/server/key.pem",
			},
		},
	}
	proxy := mustStartTestProxy(t, conf)
	defer proxy.Stop()

	certPEM, err := ioutil.ReadFile("../test_data/tls/testca/cacert.pem")
	assert.Nil(t, err)

	roots := x509.NewCertPool()
	assert.True(t, roots.AppendCertsFromPEM(certPEM))

	addr := strings.Replace(proxy.AdminAddr().String(), "127.0.0.1", "localhost", -1)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: roots,
			},
		},
	}
	res, err := client.Get(fmt.Sprintf("https://%s/", addr))
	assert.Nil(t, err)
	assert.Equal(t, res.StatusCode, 200)
}

func TestProxyAdminStatusJSON(t *testing.T) {
	// Old-style status.json page

	srv := fakeredis.Start("srv", "tcp")
	defer srv.Stop()

	conf := &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	}
	proxy := mustStartTestProxy(t, conf)
	defer proxy.Stop()

	res, err := http.Get(fmt.Sprintf("http://%s/status.json", proxy.AdminAddr().String()))

	var status map[string]interface{}
	if err = json.NewDecoder(res.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}

	expect := func(path string, expVal interface{}) {
		parts := strings.Split(path, "/")
		val := interface{}(status)
		for _, k := range parts {
			if val == nil {
				t.Fatalf("Path: %s, hit <nil> when trying to access %s", path, k)
			}
			val = val.(map[string]interface{})[k]
		}
		if val != expVal {
			t.Fatalf("Path: %s expected %v, got %v", path, expVal, val)
		}
	}

	expect("ActiveRequests", 0.0)
	expect("WaitingRequests", 0.0)
	expect("State", 2.0)
	expect("RawConnections", 0.0)

	expect("Config/uplink/addr", srv.Addr().String())
	expect("Config/uplink/pass", "[removed]")
	expect("Config/uplink/tls", false)
	expect("Config/uplink/network", "")
	expect("Config/uplink/certfile", "")
	expect("Config/uplink/keyfile", "")
	expect("Config/uplink/cacertfile", "")

	expect("Config/listen/addr", "127.0.0.1:0")
	expect("Config/listen/pass", "[removed]")
	expect("Config/listen/tls", false)
	expect("Config/listen/network", "")
	expect("Config/listen/certfile", "")
	expect("Config/listen/keyfile", "")
	expect("Config/listen/cacertfile", "")

	expect("Config/listen_raw/addr", "")
	expect("Config/listen_raw/pass", "[removed]")
	expect("Config/listen_raw/tls", false)
	expect("Config/listen_raw/network", "")
	expect("Config/listen_raw/certfile", "")
	expect("Config/listen_raw/keyfile", "")
	expect("Config/listen_raw/cacertfile", "")

	expect("Config/admin/addr", "127.0.0.1:0")
	expect("Config/admin/pass", "[removed]")
	expect("Config/admin/tls", false)
	expect("Config/admin/network", "")
	expect("Config/admin/certfile", "")
	expect("Config/admin/keyfile", "")
	expect("Config/admin/cacertfile", "")
}

func TestProxyAdminInfoJSON(t *testing.T) {
	// Old-style status.json page

	srv := fakeredis.Start("srv", "tcp")
	defer srv.Stop()

	conf := &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	}
	proxy := mustStartTestProxy(t, conf)
	defer proxy.Stop()

	res, err := http.Get(fmt.Sprintf("http://%s/info.json", proxy.AdminAddr().String()))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatal(res.StatusCode)
	}

	var status map[string]interface{}
	if err = json.NewDecoder(res.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}

	expect := func(path string, expVal interface{}) {
		parts := strings.Split(path, "/")
		val := interface{}(status)
		for _, k := range parts {
			if val == nil {
				t.Fatalf("Path: %s, hit <nil> when trying to access %s", path, k)
			}
			val = val.(map[string]interface{})[k]
		}
		if val != expVal {
			t.Fatalf("Path: %s expected %v, got %v", path, expVal, val)
		}
	}

	expect("active_requests", 0.0)
	expect("waiting_requests", 0.0)
	expect("state", 2.0)
	expect("state_str", "running")
	expect("raw_connections", 0.0)

	expect("config/uplink/addr", srv.Addr().String())
	expect("config/uplink/pass", "[removed]")
	expect("config/uplink/tls", false)
	expect("config/uplink/network", "")
	expect("config/uplink/certfile", "")
	expect("config/uplink/keyfile", "")
	expect("config/uplink/cacertfile", "")

	expect("config/listen/addr", "127.0.0.1:0")
	expect("config/listen/pass", "[removed]")
	expect("config/listen/tls", false)
	expect("config/listen/network", "")
	expect("config/listen/certfile", "")
	expect("config/listen/keyfile", "")
	expect("config/listen/cacertfile", "")

	expect("config/listen_raw/addr", "")
	expect("config/listen_raw/pass", "[removed]")
	expect("config/listen_raw/tls", false)
	expect("config/listen_raw/network", "")
	expect("config/listen_raw/certfile", "")
	expect("config/listen_raw/keyfile", "")
	expect("config/listen_raw/cacertfile", "")

	expect("config/admin/addr", "127.0.0.1:0")
	expect("config/admin/pass", "[removed]")
	expect("config/admin/tls", false)
	expect("config/admin/network", "")
	expect("config/admin/certfile", "")
	expect("config/admin/keyfile", "")
	expect("config/admin/cacertfile", "")
}
