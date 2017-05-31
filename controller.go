package main

type ControllerState struct {
	activeRequests int
}

func (proxy *RedisProxy) controller() {
	activeRequests := 0

	for {
		select {
		case permCh := <-proxy.requestPermissionChannel:
			permCh <- true
			activeRequests++
		case _ = <-proxy.releasePermissionChannel:
			activeRequests--
		case stateCh := <-proxy.controllerStateChannel:
			stateCh <- ControllerState{activeRequests: activeRequests}
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
