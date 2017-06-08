package rproxy

import (
	"testing"
	"time"

	"github.com/stvp/assert"
)

////////////////////////////////////////
// TestConfigHolder

type TestConfigHolder struct {
	config *RedisProxyConfig
}

func (ch *TestConfigHolder) GetConfig() *RedisProxyConfig {
	return ch.config
}

func (ch *TestConfigHolder) ReloadConfig() {}

////////////////////////////////////////
// Tests

func waitUntil(t *testing.T, expr func() bool, duration time.Duration) {
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
	}, time.Second)
	assert.False(t, runDone)

	contr.Stop()

	assert.Equal(t, contr.GetInfo().State, PROXY_STOPPED)
	assert.True(t, runDone)
}
