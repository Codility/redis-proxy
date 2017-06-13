package rproxy

import (
	"testing"
	"time"

	"github.com/stvp/assert"
)

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
	executing := 0

	go NewTestRequest(contr, func() { executing += 1; <-finish; executing -= 1 }).Do()
	go NewTestRequest(contr, func() { executing += 1; <-finish; executing -= 1 }).Do()

	waitUntil(t, func() bool { return executing == 2 })
	finish <- struct{}{}
	finish <- struct{}{}

	waitUntil(t, func() bool { return executing == 0 })
}

func TestControllerPauseDuringActiveRequests(t *testing.T) {
	contr := NewProxyController()
	contr.Start(&TestConfigHolder{})
	defer contr.Stop()

	finish := make(chan struct{})

	reqStartedBeforePauseWorking := false
	reqStartedBeforePause := NewTestRequest(contr, func() {
		reqStartedBeforePauseWorking = true
		<-finish
		reqStartedBeforePauseWorking = false
	})

	go reqStartedBeforePause.Do()
	waitUntil(t, func() bool { return reqStartedBeforePauseWorking })
	assert.Equal(t, contr.GetInfo().ActiveRequests, 1)

	contr.Pause()

	reqStartedDuringPauseWorking := false
	reqStartedDuringPause := NewTestRequest(contr, func() {
		reqStartedDuringPauseWorking = true
		<-finish
		reqStartedDuringPauseWorking = false
	})
	go reqStartedDuringPause.Do()
	waitUntil(t, func() bool { return contr.GetInfo().WaitingRequests == 1 })

	assert.Equal(t, contr.GetInfo().ActiveRequests, 1)
	assert.Equal(t, contr.GetInfo().State, PROXY_PAUSING)
	assert.True(t, reqStartedBeforePauseWorking)

	finish <- struct{}{}
	waitUntil(t, func() bool { return contr.GetInfo().State == PROXY_PAUSED })
	assert.Equal(t, contr.GetInfo().ActiveRequests, 0)
	assert.Equal(t, contr.GetInfo().WaitingRequests, 1)
	assert.False(t, reqStartedBeforePauseWorking)
	assert.False(t, reqStartedDuringPauseWorking)

	contr.Unpause()
	waitUntil(t, func() bool { return contr.GetInfo().ActiveRequests == 1 })
	assert.Equal(t, contr.GetInfo().WaitingRequests, 0)
	waitUntil(t, func() bool { return reqStartedDuringPauseWorking })
}

func TestControllerReloadWaitsForPause(t *testing.T) {
	contr := NewProxyController()
	ch := &TestConfigHolder{}
	contr.Start(ch)
	defer contr.Stop()

	finish := make(chan struct{})
	executing := 0

	go NewTestRequest(contr, func() { executing += 1; <-finish; executing -= 1 }).Do()
	waitUntil(t, func() bool { return executing == 1 })

	contr.Reload()
	assert.Equal(t, contr.GetInfo().State, PROXY_RELOADING)
	assert.Equal(t, ch.ReloadConfigCallCnt, 0)

	finish <- struct{}{}

	waitUntil(t, func() bool { return executing == 0 })
	assert.Equal(t, contr.GetInfo().State, PROXY_RUNNING)
	assert.Equal(t, ch.ReloadConfigCallCnt, 1)
}
