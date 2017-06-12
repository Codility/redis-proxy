package rproxy

import (
	"testing"

	"github.com/stvp/assert"
)

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

	proxy.controller.Stop()
	waitUntil(t, func() bool { return !proxy.Alive() })
}
