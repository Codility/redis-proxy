package rproxy

import (
	"testing"

	"github.com/stvp/assert"
)

const BASE_TEST_REDIS_PORT = 7300

func TestProxy(t *testing.T) {
	srv := StartFakeRedisServer("fake")
	assert.Equal(t, srv.ReqCnt(), 0)

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

	c := MustRespDial("tcp", proxy.ListenAddr().String(), 0, false)
	resp := c.MustCall(RespMsgFromStrings("get", "a"))
	assert.Equal(t, resp.String(), "$4\r\nfake\r\n")
	assert.Equal(t, srv.ReqCnt(), 1)

	proxy.controller.Stop()
	waitUntil(t, func() bool { return !proxy.Alive() })
}

func TestProxySwitch(t *testing.T) {
	srv_0 := StartFakeRedisServer("srv-0")
	srv_1 := StartFakeRedisServer("srv-1")

	conf := &TestConfig{
		conf: &ProxyConfig{
			Uplink: AddrSpec{Addr: srv_0.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	}

	proxy := mustStartTestProxy(t, conf)
	defer proxy.controller.Stop()

	c := MustRespDial("tcp", proxy.ListenAddr().String(), 0, false)
	assert.Equal(t, c.MustCall(RespMsgFromStrings("get", "a")).String(), "$5\r\nsrv-0\r\n")

	conf.Replace(&ProxyConfig{
		Uplink: AddrSpec{Addr: srv_1.Addr().String()},
		Listen: AddrSpec{Addr: "127.0.0.1:0"},
		Admin:  AddrSpec{Addr: "127.0.0.1:0"},
	})

	assert.Equal(t, c.MustCall(RespMsgFromStrings("get", "a")).String(), "$5\r\nsrv-0\r\n")

	proxy.controller.ReloadAndWait()

	assert.Equal(t, c.MustCall(RespMsgFromStrings("get", "a")).String(), "$5\r\nsrv-1\r\n")
}

func TestProxyAuthenticatesClient(t *testing.T) {
	srv := StartFakeRedisServer("srv")
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

	c := MustRespDial("tcp", proxy.ListenAddr().String(), 0, false)
	assert.Equal(t,
		c.MustCall(RespMsgFromStrings("get", "a")).String(),
		"-NOAUTH Authentication required.\r\n")
	assert.Equal(t,
		c.MustCall(RespMsgFromStrings("auth", "wrong-pass")).String(),
		"-ERR invalid password\r\n")
	assert.Equal(t,
		c.MustCall(RespMsgFromStrings("auth", "test-pass")).String(),
		"+OK\r\n")

	// None of the above have reached the actual server
	assert.Equal(t, srv.ReqCnt(), 0)

	// Also: check that the proxy filters out further AUTH commands
	assert.Equal(t,
		c.MustCall(RespMsgFromStrings("auth", "test-pass")).String(),
		"+OK\r\n")
	assert.Equal(t, srv.ReqCnt(), 0)
}

func TestOpenProxyBlocksAuthCommands(t *testing.T) {
	srv := StartFakeRedisServer("srv")
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

	c := MustRespDial("tcp", proxy.ListenAddr().String(), 0, false)
	assert.Equal(t,
		c.MustCall(RespMsgFromStrings("auth", "test-pass")).String(),
		"-ERR Client sent AUTH, but no password is set\r\n")
	assert.Equal(t, srv.ReqCnt(), 0)
}
