package rproxy

import (
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
	t.Fatalf("Expression still false after %v", duration)
}

func TestControllerStartStop(t *testing.T) {
	contr := NewProxyController()
	ch := &TestConfigHolder{}

	assert.Equal(t, contr.GetInfo().State, PROXY_STOPPED)

	runDone := false
	go func() {
		contr.run(ch)
		runDone = true
	}()

	waitUntil(t, func() bool {
		return contr.GetInfo().State == PROXY_RUNNING
	})
	assert.False(t, runDone)

	contr.Stop()

	assert.Equal(t, contr.GetInfo().State, PROXY_STOPPED)
	assert.True(t, runDone)
}

func TestControllerPause(t *testing.T) {
	contr := NewProxyController()
	contr.Start(&TestConfigHolder{})
	defer contr.Stop()
	waitUntil(t, func() bool { return contr.GetInfo().State == PROXY_RUNNING })

	r0 := NewTestRequest(contr)
	go r0.Do()
	waitUntil(t, func() bool { return r0.done })

	r1 := NewTestRequest(contr)
	contr.PauseAndWait() // --------------- pause starts
	go r1.Do()
	waitUntil(t, func() bool { return contr.GetInfo().WaitingRequests == 1 })

	time.Sleep(250 * time.Millisecond)
	assert.Equal(t, contr.GetInfo().WaitingRequests, 1)
	assert.False(t, r1.done)

	contr.Unpause() // --------------- pause ends
	waitUntil(t, func() bool { return contr.GetInfo().WaitingRequests == 0 })
	waitUntil(t, func() bool { return r1.done })
}
