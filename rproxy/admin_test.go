package rproxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stvp/assert"
	"github.com/codility/redis-proxy/fakeredis"
)

func TestProxyAdminNonTLS(t *testing.T) {
	srv := fakeredis.Start("srv")
	defer srv.Stop()

	conf := &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	}
	proxy := mustStartTestProxy(t, conf)
	defer proxy.controller.Stop()

	res, err := http.Get(fmt.Sprintf("http://%s/", proxy.AdminAddr().String()))
	assert.Nil(t, err)
	assert.Equal(t, res.StatusCode, 200)
}

func TestProxyAdminTLS(t *testing.T) {
	srv := fakeredis.Start("srv")
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
	defer proxy.controller.Stop()

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
