package main

const (
	PROXY_RUNNING = iota
	PROXY_PAUSING
	PROXY_PAUSED

	CMD_PAUSE = iota
	CMD_UNPAUSE
)

// TODO: find better name[s] for state and ControllerState

type ControllerState struct {
	activeRequests int
	state          int
	stateStr       string
}

func getStateStr(state int) string {
	switch state {
	case PROXY_RUNNING:
		return "running"
	case PROXY_PAUSING:
		return "pausing"
	case PROXY_PAUSED:
		return "paused"
	default:
		return "unknown"
	}
}

func (proxy *RedisProxy) controller() {
	activeRequests := 0
	state := PROXY_RUNNING
	requestPermissionChannel := proxy.requestPermissionChannel

	for {
		switch state {
		case PROXY_RUNNING:
			requestPermissionChannel = proxy.requestPermissionChannel
		case PROXY_PAUSING:
			if activeRequests == 0 {
				state = PROXY_PAUSED
				continue
			}
			requestPermissionChannel = nil
		case PROXY_PAUSED:
			requestPermissionChannel = nil
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
				stateStr:       getStateStr(state)}

		case cmd := <-proxy.controllerCommandChannel:
			switch cmd {
			case CMD_PAUSE:
				state = PROXY_PAUSING
			case CMD_UNPAUSE:
				state = PROXY_RUNNING
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
