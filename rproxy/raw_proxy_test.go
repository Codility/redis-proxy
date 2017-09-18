package rproxy

import (
	"testing"

	"github.com/Codility/redis-proxy/fakeredis"
	"github.com/Codility/redis-proxy/resp"
	"github.com/stvp/assert"
)

func TestRawProxy(t *testing.T) {
	srv := fakeredis.Start("fake", "tcp")
	defer srv.Stop()

	proxy, err := NewProxy(&TestConfigLoader{
		conf: &Config{
			Uplink:          AddrSpec{Addr: srv.Addr().String()},
			Listen:          AddrSpec{Addr: "127.0.0.1:0"},
			ListenUnmanaged: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:           AddrSpec{Addr: "127.0.0.1:0"},
		},
	})
	assert.Nil(t, err)
	proxy.Start()
	assert.True(t, proxy.State().IsAlive())

	c := resp.MustDial("tcp", proxy.ListenUnmanagedAddr().String(), 0, false)
	resp := c.MustCall(resp.MsgFromStrings("get", "a"))
	assert.Equal(t, resp.String(), "$4\r\nfake\r\n")
	assert.Equal(t, srv.ReqCnt(), 1)

	proxy.Stop()
	waitUntil(t, func() bool { return !proxy.State().IsAlive() })
}
