package rproxy

import (
	"testing"
	"time"

	"github.com/Codility/redis-proxy/fakeredis"
	"github.com/stvp/assert"
)

func startFakeredisAndProxy(t *testing.T) (*fakeredis.FakeRedisServer, *Proxy) {
	srv := fakeredis.Start("srv", "tcp")

	proxy := mustStartTestProxy(t, &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: srv.Addr().String()},
			Listen: AddrSpec{Addr: "127.0.0.1:0", Pass: "test-pass"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
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
	proxy.PauseAndWait() // --------------- pause starts
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

func TestProxyReloadWaitsForPause(t *testing.T) {
	srv, proxy := startFakeredisAndProxy(t)
	defer srv.Stop()
	defer proxy.Stop()

	finish := make(chan struct{})
	executing := 0

	go NewTestRequest(proxy, func() { executing += 1; <-finish; executing -= 1 }).Do()
	waitUntil(t, func() bool { return executing == 1 })

	proxy.Reload()
	assert.Equal(t, proxy.GetInfo().State, ProxyReloading)
	// assert.Equal(t, ch.ReloadConfigCallCnt, 0)

	finish <- struct{}{}

	waitUntil(t, func() bool { return executing == 0 })
	assert.Equal(t, proxy.GetInfo().State, ProxyRunning)
	//	assert.Equal(t, ch.ReloadConfigCallCnt, 1)
}
