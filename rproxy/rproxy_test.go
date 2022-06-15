package rproxy

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Codility/redis-proxy/fakeredis"
	"github.com/Codility/redis-proxy/resp"
	"github.com/stvp/assert"
)

const BaseTestRedisPort = 7300

func TestProxy(t *testing.T) {
	srv := fakeredis.Start("fake", "tcp")
	defer srv.Stop()

	proxy, err := NewProxy(&TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
		},
	})
	assert.Nil(t, err)
	proxy.Start()
	assert.True(t, proxy.State().IsAlive())

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	resp := c.MustCall(resp.MsgFromStrings("get", "a"))
	assert.Equal(t, resp.String(), "$4\r\nfake\r\n")
	assert.Equal(t, srv.ReqCnt(), 1)

	err = proxy.Stop()
	assert.Nil(t, err)
}

func TestProxyUnix(t *testing.T) {
	srv := fakeredis.Start("fake", "tcp")
	defer srv.Stop()

	dir, err := ioutil.TempDir("/tmp", "rproxy")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	socketPath := dir + "/rproxy.sock"

	proxy, err := NewProxy(&TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: socketPath, Network: "unix"},
		},
	})
	assert.Nil(t, err)
	proxy.Start()
	assert.True(t, proxy.State().IsAlive())

	c := resp.MustDial("unix", socketPath, 0, false)
	resp := c.MustCall(resp.MsgFromStrings("get", "a"))
	assert.Equal(t, resp.String(), "$4\r\nfake\r\n")
	assert.Equal(t, srv.ReqCnt(), 1)

	c.Close()
	err = proxy.Stop()
	assert.Nil(t, err)

	_, err = os.Stat(socketPath)
	assert.True(t, os.IsNotExist(err))
}

func TestProxyTLS(t *testing.T) {
	srv := fakeredis.Start("fake", "tcp")
	defer srv.Stop()

	proxy, err := NewProxy(&TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{
				Addr:     "127.0.0.1:0",
				TLS:      true,
				CertFile: "../test_data/tls/server/cert.pem",
				KeyFile:  "../test_data/tls/server/key.pem",
			},
		},
	})
	assert.Nil(t, err)
	proxy.Start()
	assert.True(t, proxy.State().IsAlive())

	certPEM, err := ioutil.ReadFile("../test_data/tls/testca/cacert.pem")
	assert.Nil(t, err)

	roots := x509.NewCertPool()
	assert.True(t, roots.AppendCertsFromPEM(certPEM))

	addr := strings.Replace(proxy.ListenAddr().String(), "127.0.0.1", "localhost", -1)
	//	time.Sleep(time.Second)
	tlsc, err := tls.Dial("tcp", addr, &tls.Config{RootCAs: roots})
	assert.Nil(t, err)

	c := resp.NewConn(tlsc, 0, false)
	resp := c.MustCall(resp.MsgFromStrings("get", "a"))
	assert.Equal(t, resp.String(), "$4\r\nfake\r\n")
	assert.Equal(t, srv.ReqCnt(), 1)

	err = proxy.Stop()
	assert.Nil(t, err)
}

func TestProxyUplinkTLS(t *testing.T) {
	srv := fakeredis.StartTLS("fake", "tcp")
	defer srv.Stop()

	laddr := strings.Replace(srv.Addr().String(), "127.0.0.1", "localhost", -1)
	proxy := mustStartTestProxy(t, &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: laddr,
				TLS:        true,
				CACertFile: "../test_data/tls/testca/cacert.pem",
			},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
		},
	})
	defer proxy.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	resp, err := c.Call(resp.MsgFromStrings("get", "a"))
	assert.Nil(t, err)
	assert.Equal(t, resp.String(), "$4\r\nfake\r\n")
}

func TestProxyUplinkSkipVerify(t *testing.T) {
	srv := fakeredis.StartTLS("fake", "tcp")
	defer srv.Stop()

	laddr := strings.Replace(srv.Addr().String(), "127.0.0.1", "localhost", -1)
	proxy := mustStartTestProxy(t, &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: laddr,
				TLS:        true,
				SkipVerify: true,
			},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
		},
	})
	defer proxy.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	resp, err := c.Call(resp.MsgFromStrings("get", "a"))
	assert.Nil(t, err)
	assert.Equal(t, resp.String(), "$4\r\nfake\r\n")
}

func TestProxyUplinkUnix(t *testing.T) {
	srv := fakeredis.Start("fake", "unix")
	defer srv.Stop()

	proxy := mustStartTestProxy(t, &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String(), Network: "unix"},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
		}})
	defer proxy.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	resp, err := c.Call(resp.MsgFromStrings("get", "a"))
	assert.Nil(t, err)
	assert.Equal(t, resp.String(), "$4\r\nfake\r\n")
}

func TestProxySwitch(t *testing.T) {
	srv_0 := fakeredis.Start("srv-0", "tcp")
	defer srv_0.Stop()
	srv_1 := fakeredis.Start("srv-1", "tcp")
	defer srv_1.Stop()

	conf := &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv_0.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
		},
	}

	proxy := mustStartTestProxy(t, conf)
	defer proxy.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	assert.Equal(t, c.MustCall(resp.MsgFromStrings("get", "a")).String(), "$5\r\nsrv-0\r\n")

	conf.Replace(&Config{
		Uplink: AddrSpec{Addr: srv_1.Addr().String()},
		Listen: AddrSpec{Addr: "127.0.0.1:0"},
	})

	assert.Equal(t, c.MustCall(resp.MsgFromStrings("get", "a")).String(), "$5\r\nsrv-0\r\n")

	proxy.Reload()

	assert.Equal(t, c.MustCall(resp.MsgFromStrings("get", "a")).String(), "$5\r\nsrv-1\r\n")
}

func TestProxyRejectsBrokenConfigOnStart(t *testing.T) {
	// empty config is invalid
	proxy, err := NewProxy(&TestConfigLoader{conf: &Config{}})
	assert.Nil(t, proxy)
	assert.NotNil(t, err)
}

func TestProxyRejectsBrokenConfigOnSwitch(t *testing.T) {
	srv_0 := fakeredis.Start("srv-0", "tcp")
	defer srv_0.Stop()

	conf := &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv_0.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
		},
	}

	proxy := mustStartTestProxy(t, conf)
	defer proxy.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	assert.Equal(t, c.MustCall(resp.MsgFromStrings("get", "a")).String(), "$5\r\nsrv-0\r\n")

	conf.Replace(&Config{
		Uplink: AddrSpec{Addr: "127.0.0.1:0"}, // <- incorrect uplink address
		Listen: AddrSpec{Addr: "127.0.0.1:0"},
	})

	assert.Equal(t, c.MustCall(resp.MsgFromStrings("get", "a")).String(), "$5\r\nsrv-0\r\n")

	proxy.Reload()

	assert.Equal(t, c.MustCall(resp.MsgFromStrings("get", "a")).String(), "$5\r\nsrv-0\r\n")
}

func TestProxyAuthenticatesClient(t *testing.T) {
	srv := fakeredis.Start("srv", "tcp")
	defer srv.Stop()

	conf := &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0", Pass: "test-pass"},
		},
	}

	proxy := mustStartTestProxy(t, conf)
	defer proxy.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	assert.Equal(t,
		c.MustCall(resp.MsgFromStrings("get", "a")).String(),
		"-NOAUTH Authentication required.\r\n")
	assert.Equal(t,
		c.MustCall(resp.MsgFromStrings("auth", "wrong-pass")).String(),
		"-ERR invalid password\r\n")
	assert.Equal(t,
		c.MustCall(resp.MsgFromStrings("auth", "test-pass")).String(),
		"+OK\r\n")

	// None of the above have reached the actual server
	assert.Equal(t, srv.ReqCnt(), 0)

	// Also: check that the proxy filters out further AUTH commands
	assert.Equal(t,
		c.MustCall(resp.MsgFromStrings("auth", "test-pass")).String(),
		"+OK\r\n")
	assert.Equal(t, srv.ReqCnt(), 0)
}

func TestOpenProxyBlocksAuthCommands(t *testing.T) {
	srv := fakeredis.Start("srv", "tcp")
	defer srv.Stop()

	conf := &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
		},
	}

	proxy := mustStartTestProxy(t, conf)
	defer proxy.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	assert.Equal(t,
		c.MustCall(resp.MsgFromStrings("auth", "test-pass")).String(),
		"-ERR Client sent AUTH, but no password is set\r\n")
	assert.Equal(t, srv.ReqCnt(), 0)
}

func mustStartRedisServer(port int, args ...string) *exec.Cmd {
	fullArgs := append([]string{"--port", strconv.Itoa(port)}, args...)
	p := exec.Command("redis-server", fullArgs...)
	p.Stdout = os.Stdout
	p.Stderr = os.Stderr
	if err := p.Start(); err != nil {
		panic(err)
	}

	for {
		c, err := resp.Dial("tcp", fmt.Sprintf("localhost:%d", port), 0, false)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return p
}

func TestProxyCanAuthenticateWithRedis(t *testing.T) {
	redis := mustStartRedisServer(
		BaseTestRedisPort,
		"--requirepass", "test-pass")
	defer redis.Process.Kill()

	redisUrl := fmt.Sprintf("localhost:%d", BaseTestRedisPort)
	conf := &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: redisUrl, Pass: "test-pass"},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
		},
	}

	proxy := mustStartTestProxy(t, conf)
	defer proxy.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	assert.Equal(t,
		c.MustCall(resp.MsgFromStrings("SET", "A", "test")).String(),
		"+OK\r\n")
}

func TestProxyKeepsTrackOfSelectedDB(t *testing.T) {
	srv_0 := fakeredis.Start("srv-0", "tcp")
	defer srv_0.Stop()
	srv_1 := fakeredis.Start("srv-1", "tcp")
	defer srv_1.Stop()

	conf := NewTestConfigLoader(srv_0.Addr().String())
	proxy := mustStartTestProxy(t, conf)
	defer proxy.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	c.MustCall(resp.MsgFromStrings("SELECT", "1"))

	// 1. That SELECT message must make it to the server
	assert.Equal(t, srv_0.ReqCnt(), 1)
	assert.True(t, srv_0.LastRequest().Equal(resp.MsgFromStrings("SELECT", "1")))

	// 2. Proxy must resend that message after reconnecting, before first request
	conf.Replace(&Config{
		Uplink: AddrSpec{Addr: srv_1.Addr().String()},
		Listen: AddrSpec{Addr: "127.0.0.1:0"},
	})
	assert.Nil(t, proxy.Reload())
	c.MustCall(resp.MsgFromStrings("SET", "k", "v"))

	assert.Equal(t, srv_1.ReqCnt(), 2)
	assert.True(t, srv_1.Requests()[0].Equal(resp.MsgFromStrings("SELECT", "1")))
}

func TestProxyKillsConnectionOnBrokenCommands(t *testing.T) {
	srv := fakeredis.Start("srv", "tcp")
	defer srv.Stop()

	proxy := mustStartTestProxy(t, NewTestConfigLoader(srv.Addr().String()))
	defer proxy.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	resp := c.MustCall(resp.MsgFromStrings("SELECT", "X"))
	assert.True(t,
		bytes.Equal(resp.Data(),
			[]byte("-ERR Command parse error (redis-proxy)\r\n")))

	_, err := c.ReadMsg()
	assert.Equal(t, err, io.EOF)
}

func startFakeredisAndProxy(t *testing.T) (*fakeredis.FakeRedisServer, *Proxy) {
	srv := fakeredis.Start("srv", "tcp")

	proxy := mustStartTestProxy(t, &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0", Pass: "test-pass"},
		},
	})

	return srv, proxy
}

func TestProxyPause(t *testing.T) {
	srv, proxy := startFakeredisAndProxy(t)
	defer srv.Stop()
	defer proxy.Stop()

	// in ProxyRunning: requests are executed immediately
	r0 := NewTestRequest(proxy, func() {})
	go r0.Do()
	waitUntil(t, func() bool { return r0.done })

	// in ProxyPaused: requests are queued
	r1 := NewTestRequest(proxy, func() {})
	proxy.Pause() // --------------- pause starts
	go r1.Do()
	waitUntil(t, func() bool { return proxy.GetInfo().WaitingRequests == 1 })

	time.Sleep(250 * time.Millisecond)
	assert.Equal(t, proxy.GetInfo().WaitingRequests, 1)
	assert.False(t, r1.done)

	// back to ProxyRunning: queued requests get executed
	proxy.Unpause() // --------------- pause ends
	waitUntil(t, func() bool { return proxy.GetInfo().WaitingRequests == 0 })
	waitUntil(t, func() bool { return r1.done })
}

func TestProxyAllowsParallelRequests(t *testing.T) {
	srv, proxy := startFakeredisAndProxy(t)
	defer srv.Stop()
	defer proxy.Stop()

	finish := make(chan struct{})
	executing := 0

	go NewTestRequest(proxy, func() { executing += 1; <-finish; executing -= 1 }).Do()
	go NewTestRequest(proxy, func() { executing += 1; <-finish; executing -= 1 }).Do()

	waitUntil(t, func() bool { return executing == 2 })
	finish <- struct{}{}
	finish <- struct{}{}

	waitUntil(t, func() bool { return executing == 0 })
}

func TestProxyAllowsCancellingPause(t *testing.T) {
	srv, proxy := startFakeredisAndProxy(t)
	defer srv.Stop()
	defer proxy.Stop()

	finish := make(chan struct{})
	r := NewTestRequest(proxy, func() { <-finish })
	go r.Do()

	waitUntil(t, func() bool { return r.started })

	proxy.Pause()
	assert.Equal(t, proxy.GetInfo().State, ProxyPausing)

	proxy.Unpause()
	assert.Equal(t, proxy.GetInfo().State, ProxyRunning)

	finish <- struct{}{}
	waitUntil(t, func() bool { return r.done })
}

func TestProxyPauseDuringActiveRequests(t *testing.T) {
	srv, proxy := startFakeredisAndProxy(t)
	defer srv.Stop()
	defer proxy.Stop()

	finish := make(chan struct{})

	reqStartedBeforePauseWorking := false
	reqStartedBeforePause := NewTestRequest(proxy, func() {
		reqStartedBeforePauseWorking = true
		<-finish
		reqStartedBeforePauseWorking = false
	})

	go reqStartedBeforePause.Do()
	waitUntil(t, func() bool { return reqStartedBeforePauseWorking })
	assert.Equal(t, proxy.GetInfo().ActiveRequests, 1)

	proxy.Pause()

	reqStartedDuringPauseWorking := false
	reqStartedDuringPause := NewTestRequest(proxy, func() {
		reqStartedDuringPauseWorking = true
		<-finish
		reqStartedDuringPauseWorking = false
	})
	go reqStartedDuringPause.Do()
	waitUntil(t, func() bool { return proxy.GetInfo().WaitingRequests == 1 })

	assert.Equal(t, proxy.GetInfo().ActiveRequests, 1)
	assert.Equal(t, proxy.GetInfo().State, ProxyPausing)
	assert.True(t, reqStartedBeforePauseWorking)

	finish <- struct{}{}
	waitUntil(t, func() bool { return proxy.GetInfo().State == ProxyPaused })
	assert.Equal(t, proxy.GetInfo().ActiveRequests, 0)
	assert.Equal(t, proxy.GetInfo().WaitingRequests, 1)
	assert.False(t, reqStartedBeforePauseWorking)
	assert.False(t, reqStartedDuringPauseWorking)

	proxy.Unpause()
	waitUntil(t, func() bool { return proxy.GetInfo().ActiveRequests == 1 })
	assert.Equal(t, proxy.GetInfo().WaitingRequests, 0)
	waitUntil(t, func() bool { return reqStartedDuringPauseWorking })
}

func TestProxyCloseClientOnEOF(t *testing.T) {
	redis := mustStartRedisServer(
		BaseTestRedisPort,
		"--requirepass", "test-pass")
	defer redis.Process.Kill()

	redisURL := fmt.Sprintf("localhost:%d", BaseTestRedisPort)
	conf := &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: redisURL, Pass: "test-pass"},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
		},
	}
	proxy := mustStartTestProxy(t, conf)
	defer proxy.Stop()

	conn := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	assert.Equal(t, conn.MustCall(resp.MsgFromStrings("get", "a")).String(), "$-1\r\n")
	redis.Process.Kill()
	response, err := conn.Call(resp.MsgFromStrings("get", "a"))
	assert.NotNil(t, err)
	assert.Nil(t, response)
}
