package rproxy

import (
	"testing"
	"time"

	"github.com/Codility/redis-proxy/fakeredis"
	"github.com/Codility/redis-proxy/resp"
	"github.com/stvp/assert"
)

func TestRawProxy(t *testing.T) {
	srv := fakeredis.Start("fake", "tcp")
	defer srv.Stop()

	proxy, err := NewProxy(&TestConfigLoader{
		conf: &Config{
			Uplink:    AddrSpec{Addr: srv.Addr().String()},
			Listen:    AddrSpec{Addr: "127.0.0.1:0"},
			ListenRaw: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:     AddrSpec{Addr: "127.0.0.1:0"},
		},
	})
	assert.Nil(t, err)
	proxy.Start()
	assert.True(t, proxy.State().IsAlive())
	assert.Equal(t, proxy.GetInfo().RawConnections, 0)

	c := resp.MustDial("tcp", proxy.ListenRawAddr().String(), 0, false)
	resp := c.MustCall(resp.MsgFromStrings("get", "a"))
	assert.Equal(t, resp.String(), "$4\r\nfake\r\n")
	assert.Equal(t, srv.ReqCnt(), 1)
	assert.Equal(t, proxy.GetInfo().RawConnections, 1)

	proxy.Stop()
	waitUntil(t, func() bool { return !proxy.State().IsAlive() })
}

func TestRawProxy_TerminateBeforeConnectionFullyStarts(t *testing.T) {
	srv := fakeredis.Start("fake", "tcp")
	defer srv.Stop()

	proxy, err := NewProxy(&TestConfigLoader{
		conf: &Config{
			Uplink:    AddrSpec{Addr: srv.Addr().String()},
			Listen:    AddrSpec{Addr: "127.0.0.1:0"},
			ListenRaw: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:     AddrSpec{Addr: "127.0.0.1:0"},
		},
	})
	assert.Nil(t, err)
	proxy.Start()
	assert.True(t, proxy.State().IsAlive())

	c := resp.MustDial("tcp", proxy.ListenRawAddr().String(), 0, false)
	assert.True(t, isConnOpen(c))
	assert.Equal(t, proxy.GetInfo().RawConnections, 1)

	proxy.rawProxy.TerminateAll()

	deadline := time.Now().Add(time.Second)
	for isConnOpen(c) {
		if time.Now().After(deadline) {
			t.Fatal("Expected client to shut down")
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.Equal(t, proxy.GetInfo().RawConnections, 0)

	proxy.Stop()
	waitUntil(t, func() bool { return !proxy.State().IsAlive() })
}

func isConnOpen(conn *resp.Conn) bool {
	_, err := conn.Call(resp.MsgFromStrings("ping"))
	return err == nil
}
