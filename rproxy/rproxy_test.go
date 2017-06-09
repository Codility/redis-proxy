package rproxy

import (
	"testing"

	"github.com/stvp/assert"
)

func TestProxy(t *testing.T) {
	srv := StartFakeRedisServer()
	assert.Equal(t, srv.ReqCnt(), 0)

	proxy, err := NewProxy(&ConstConfig{
		conf: &ProxyConfig{
			UplinkAddr: srv.Addr().String(),
			ListenOn:   "127.0.0.1:0",
			AdminOn:    "127.0.0.1:0",
		},
	})
	assert.Nil(t, err)
	assert.False(t, proxy.Alive())

	go proxy.Run()
	waitUntil(t, func() bool { return proxy.Alive() })

	c, err := RespDial("tcp", proxy.ListenAddr().String(), 0, false)
	if err != nil {
		panic(err)
	}
	_, err = c.WriteMsg(&RespMsg{[]byte("*2\r\n$3\r\nget\r\n$1\r\na\r\n")})
	if err != nil {
		panic(err)
	}
	resp, err := c.ReadMsg()
	if err != nil {
		panic(err)
	}
	assert.Equal(t, resp.String(), "$4\r\nfake\r\n")

	proxy.controller.Stop()
	waitUntil(t, func() bool { return !proxy.Alive() })
}
