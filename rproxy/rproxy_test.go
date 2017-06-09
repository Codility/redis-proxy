package rproxy

import (
	"testing"

	"github.com/stvp/assert"
)

func TestProxy(t *testing.T) {
	srv := StartFakeRedisServer()
	assert.Equal(t, srv.ReqCnt(), 0)

	srv.Addr()
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

	proxy.controller.Stop()
	waitUntil(t, func() bool { return !proxy.Alive() })
}
