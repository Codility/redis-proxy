package rproxy

import (
	"runtime/debug"
	"testing"
	"time"

	"github.com/stvp/assert"
)

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

func TestControllerStartStop(t *testing.T) {
	contr := NewProxyController()
	ch := &TestConfigHolder{}

	assert.Equal(t, contr.GetInfo().State, PROXY_STOPPED)

	contr.Start(ch)
	assert.Equal(t, contr.GetInfo().State, PROXY_RUNNING)

	contr.Stop()
	assert.Equal(t, contr.GetInfo().State, PROXY_STOPPED)
}

func TestControllerPause(t *testing.T) {
	contr := NewProxyController()
	contr.Start(&TestConfigHolder{})
	defer contr.Stop()
	assert.Equal(t, contr.GetInfo().State, PROXY_RUNNING)

	// in PROXY_RUNNING: requests are executed immediately
	r0 := NewTestRequest(contr, func() {})
	go r0.Do()
	waitUntil(t, func() bool { return r0.done })

	// in PROXY_PAUSED: requests are queued
	r1 := NewTestRequest(contr, func() {})
	contr.PauseAndWait() // --------------- pause starts
	go r1.Do()
	waitUntil(t, func() bool { return contr.GetInfo().WaitingRequests == 1 })

	time.Sleep(250 * time.Millisecond)
	assert.Equal(t, contr.GetInfo().WaitingRequests, 1)
	assert.False(t, r1.done)

	// back to PROXY_RUNNING: queued requests get executed
	contr.Unpause() // --------------- pause ends
	waitUntil(t, func() bool { return contr.GetInfo().WaitingRequests == 0 })
	waitUntil(t, func() bool { return r1.done })
}

func TestControllerAllowsParallelRequests(t *testing.T) {
	contr := NewProxyController()
	contr.Start(&TestConfigHolder{})
	defer contr.Stop()

	finish := make(chan struct{})
	waiting := 0

	go NewTestRequest(contr, func() { waiting += 1; <-finish; waiting -= 1 }).Do()
	go NewTestRequest(contr, func() { waiting += 1; <-finish; waiting -= 1 }).Do()

	waitUntil(t, func() bool { return waiting == 2 })
	finish <- struct{}{}
	finish <- struct{}{}

	waitUntil(t, func() bool { return waiting == 0 })
}

func TestControllerPauseDuringActiveRequests(t *testing.T) {
	contr := NewProxyController()
	contr.Start(&TestConfigHolder{})
	defer contr.Stop()

	finish := make(chan struct{})

	go NewTestRequest(contr, func() { <-finish }).Do()
	waitUntil(t, func() bool { return contr.GetInfo().ActiveRequests == 1 })

	contr.Pause()
	waitUntil(t, func() bool { return contr.GetInfo().State == PROXY_PAUSING })

	go NewTestRequest(contr, func() { <-finish }).Do()
	waitUntil(t, func() bool { return contr.GetInfo().WaitingRequests == 1 })

	time.Sleep(250 * time.Millisecond)
	assert.Equal(t, contr.GetInfo().ActiveRequests, 1)
	assert.Equal(t, contr.GetInfo().State, PROXY_PAUSING)

	finish <- struct{}{}
	waitUntil(t, func() bool { return contr.GetInfo().State == PROXY_PAUSED })
	time.Sleep(250 * time.Millisecond)
	assert.Equal(t, contr.GetInfo().ActiveRequests, 0)
	assert.Equal(t, contr.GetInfo().WaitingRequests, 1)

	contr.Unpause()
	waitUntil(t, func() bool { return contr.GetInfo().ActiveRequests == 1 })
	assert.Equal(t, contr.GetInfo().WaitingRequests, 0)
}
