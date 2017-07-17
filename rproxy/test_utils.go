package rproxy

import (
	"runtime/debug"
	"testing"
	"time"

	"github.com/Codility/redis-proxy/resp"
	"github.com/stvp/assert"
)

////////////////////////////////////////
// TestConfigHolder

type TestConfigHolder struct {
	config              *Config
	GetConfigCallCnt    int
	ReloadConfigCallCnt int
}

func (ch *TestConfigHolder) GetConfig() *Config {
	ch.GetConfigCallCnt += 1
	return ch.config
}

func (ch *TestConfigHolder) ReloadConfig() {
	ch.ReloadConfigCallCnt += 1
}

////////////////////////////////////////
// TestRequest

type TestRequest struct {
	proxy *Proxy
	done  bool
	block func()
}

func NewTestRequest(proxy *Proxy, block func()) *TestRequest {
	return &TestRequest{proxy: proxy, block: block}
}

func (r *TestRequest) Do() {
	r.proxy.CallUplink(func() (*resp.Msg, error) {
		r.block()
		return nil, nil
	})
	r.done = true
}

////////////////////////////////////////
// Other plumbing

func mustStartTestProxy(t *testing.T, conf *TestConfigLoader) *Proxy {
	proxy, err := NewProxy(conf)
	assert.Nil(t, err)
	proxy.Start()
	assert.True(t, proxy.State().IsAlive())
	return proxy
}

func waitUntil(t *testing.T, expr func() bool) {
	const duration = time.Second

	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if expr() {
			return
		}
		time.Sleep(250 * time.Microsecond)
	}
	debug.PrintStack()
	t.Fatalf("Expression still false after %v", duration)
}
