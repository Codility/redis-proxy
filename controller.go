package main

import "fmt"

const (
	PROXY_RUNNING = iota
	PROXY_PAUSING
	PROXY_PAUSED
	PROXY_RELOADING

	CMD_PAUSE = iota
	CMD_UNPAUSE
	CMD_RELOAD
)

// TODO: find better name[s] for state and ControllerState

type ControllerState struct {
	activeRequests int
	state          int
	stateStr       string
	config         *RedisProxyConfig
}

func getStateStr(state int) string {
	switch state {
	case PROXY_RUNNING:
		return "running"
	case PROXY_PAUSING:
		return "pausing"
	case PROXY_PAUSED:
		return "paused"
	case PROXY_RELOADING:
		return "reloading"
	default:
		return "unknown"
	}
}

func (proxy *RedisProxy) controller() {
	activeRequests := 0
	state := PROXY_RUNNING
	requestPermissionChannel := proxy.requestPermissionChannel

	for {
		requestPermissionChannel = nil
		switch state {
		case PROXY_RUNNING:
			requestPermissionChannel = proxy.requestPermissionChannel
		case PROXY_PAUSING:
			if activeRequests == 0 {
				state = PROXY_PAUSED
				continue
			}
		case PROXY_RELOADING:
			if activeRequests == 0 {
				proxy.reloadConfig()
				state = PROXY_RUNNING
				continue
			}
		case PROXY_PAUSED:
			// nothing
		}
		select {
		// In states other than PROXY_RUNNING
		// requestPermissionChannel is nil, so the controller
		// will not receive any requests for permission.
		case permCh := <-requestPermissionChannel:
			permCh <- true
			activeRequests++
		case _ = <-proxy.releasePermissionChannel:
			activeRequests--

		case stateCh := <-proxy.controllerStateChannel:
			stateCh <- &ControllerState{
				activeRequests: activeRequests,
				state:          state,
				stateStr:       getStateStr(state),
				config:         proxy.config}

		case cmd := <-proxy.controllerCommandChannel:
			switch cmd {
			case CMD_PAUSE:
				state = PROXY_PAUSING
			case CMD_UNPAUSE:
				state = PROXY_RUNNING
			case CMD_RELOAD:
				state = PROXY_RELOADING
			default:
				fmt.Println("Unknown controller command:", cmd)
			}
		}
	}
}

func (proxy *RedisProxy) enterExecution() {
	ch := make(chan bool)
	proxy.requestPermissionChannel <- ch
	<-ch
}

func (proxy *RedisProxy) leaveExecution() {
	proxy.releasePermissionChannel <- true
}

func (proxy *RedisProxy) executeCall(block func() ([]byte, error)) ([]byte, error) {
	proxy.enterExecution()
	defer proxy.leaveExecution()

	return block()
}

func (proxy *RedisProxy) getControllerState() *ControllerState {
	ch := make(chan *ControllerState)
	proxy.controllerStateChannel <- ch
	return <-ch
}

func (proxy *RedisProxy) pause() {
	proxy.controllerCommandChannel <- CMD_PAUSE
}

func (proxy *RedisProxy) unpause() {
	proxy.controllerCommandChannel <- CMD_UNPAUSE
}

func (proxy *RedisProxy) reload() {
	proxy.controllerCommandChannel <- CMD_RELOAD
}
