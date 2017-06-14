package rproxy

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stvp/assert"
	"gitlab.codility.net/marcink/redis-proxy/fakeredis"
	"gitlab.codility.net/marcink/redis-proxy/resp"
)

const BaseTestRedisPort = 7300

func TestProxy(t *testing.T) {
	srv := fakeredis.Start("fake")
	defer srv.Stop()

	proxy, err := NewProxy(&TestConfig{
		conf: &ProxyConfig{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	})
	assert.Nil(t, err)
	assert.False(t, proxy.Alive())

	go proxy.Run()
	waitUntil(t, func() bool { return proxy.Alive() })

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	resp := c.MustCall(resp.MsgFromStrings("get", "a"))
	assert.Equal(t, resp.String(), "$4\r\nfake\r\n")
	assert.Equal(t, srv.ReqCnt(), 1)

	proxy.controller.Stop()
	waitUntil(t, func() bool { return !proxy.Alive() })
}

func TestProxySwitch(t *testing.T) {
	srv_0 := fakeredis.Start("srv-0")
	defer srv_0.Stop()
	srv_1 := fakeredis.Start("srv-1")
	defer srv_1.Stop()

	conf := &TestConfig{
		conf: &ProxyConfig{
			Uplink: AddrSpec{Addr: srv_0.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	}

	proxy := mustStartTestProxy(t, conf)
	defer proxy.controller.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	assert.Equal(t, c.MustCall(resp.MsgFromStrings("get", "a")).String(), "$5\r\nsrv-0\r\n")

	conf.Replace(&ProxyConfig{
		Uplink: AddrSpec{Addr: srv_1.Addr().String()},
		Listen: AddrSpec{Addr: "127.0.0.1:0"},
		Admin:  AddrSpec{Addr: "127.0.0.1:0"},
	})

	assert.Equal(t, c.MustCall(resp.MsgFromStrings("get", "a")).String(), "$5\r\nsrv-0\r\n")

	proxy.controller.ReloadAndWait()

	assert.Equal(t, c.MustCall(resp.MsgFromStrings("get", "a")).String(), "$5\r\nsrv-1\r\n")
}

func TestProxyAuthenticatesClient(t *testing.T) {
	srv := fakeredis.Start("srv")
	defer srv.Stop()

	conf := &TestConfig{
		conf: &ProxyConfig{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0", Pass: "test-pass"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	}

	proxy := mustStartTestProxy(t, conf)
	defer proxy.controller.Stop()

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
	srv := fakeredis.Start("srv")
	defer srv.Stop()

	conf := &TestConfig{
		conf: &ProxyConfig{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	}

	proxy := mustStartTestProxy(t, conf)
	defer proxy.controller.Stop()

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
	conf := &TestConfig{
		conf: &ProxyConfig{
			Uplink: AddrSpec{Addr: redisUrl, Pass: "test-pass"},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	}

	proxy := mustStartTestProxy(t, conf)
	defer proxy.controller.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	assert.Equal(t,
		c.MustCall(resp.MsgFromStrings("SET", "A", "test")).String(),
		"+OK\r\n")
}

func TestProxyKeepsTrackOfSelectedDB(t *testing.T) {
	srv_0 := fakeredis.Start("srv-0")
	defer srv_0.Stop()
	srv_1 := fakeredis.Start("srv-1")
	defer srv_1.Stop()

	conf := NewTestConfig(srv_0.Addr().String())
	proxy := mustStartTestProxy(t, conf)
	defer proxy.controller.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	c.MustCall(resp.MsgFromStrings("SELECT", "1"))

	// 1. That SELECT message must make it to the server
	assert.Equal(t, srv_0.ReqCnt(), 1)
	assert.True(t, srv_0.LastRequest().Equal(resp.MsgFromStrings("SELECT", "1")))

	// 2. Proxy must resend that message after reconnecting, before first request
	conf.Replace(&ProxyConfig{
		Uplink: AddrSpec{Addr: srv_1.Addr().String()},
		Listen: AddrSpec{Addr: "127.0.0.1:0"},
		Admin:  AddrSpec{Addr: "127.0.0.1:0"},
	})
	proxy.controller.ReloadAndWait()
	c.MustCall(resp.MsgFromStrings("SET", "k", "v"))

	assert.Equal(t, srv_1.ReqCnt(), 2)
	assert.True(t, srv_1.Requests()[0].Equal(resp.MsgFromStrings("SELECT", "1")))
}

func TestProxyKillsConnectionOnBrokenCommands(t *testing.T) {
	srv := fakeredis.Start("srv")
	defer srv.Stop()

	proxy := mustStartTestProxy(t, NewTestConfig(srv.Addr().String()))
	defer proxy.controller.Stop()

	c := resp.MustDial("tcp", proxy.ListenAddr().String(), 0, false)
	resp := c.MustCall(resp.MsgFromStrings("SELECT", "X"))
	assert.True(t,
		bytes.Equal(resp.Data(),
			[]byte("-ERR Command parse error (redis-proxy)\r\n")))

	_, err := c.ReadMsg()
	assert.Equal(t, err, io.EOF)
}
