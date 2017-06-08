package rproxy

import (
	"log"
	"time"
)

const (
	PROXY_STOPPED = iota
	PROXY_RUNNING
	PROXY_PAUSING
	PROXY_PAUSED
	PROXY_RELOADING
	PROXY_STOPPING

	CMD_PAUSE = iota
	CMD_UNPAUSE
	CMD_RELOAD
	CMD_STOP

	MAX_CONNECTIONS = 1000
)

type ProxyControllerChannels struct {
	requestPermission chan chan struct{}
	releasePermission chan struct{}
	info              chan chan *ControllerInfo
	command           chan int
}

type ProxyController struct {
	channels *ProxyControllerChannels
}

func NewProxyController() *ProxyController {
	return &ProxyController{}
}

func (controller *ProxyController) run(confHolder ProxyConfigHolder) {
	controller.channels = &ProxyControllerChannels{
		requestPermission: make(chan chan struct{}, MAX_CONNECTIONS),
		releasePermission: make(chan struct{}, MAX_CONNECTIONS),
		info:              make(chan chan *ControllerInfo),
		command:           make(chan int),
	}

	activeRequests := 0
	state := PROXY_RUNNING
	requestPermissionChannel := controller.channels.requestPermission

	for state != PROXY_STOPPING {
		requestPermissionChannel = nil
		switch state {
		case PROXY_RUNNING:
			requestPermissionChannel = controller.channels.requestPermission
		case PROXY_PAUSING:
			if activeRequests == 0 {
				state = PROXY_PAUSED
				continue
			}
		case PROXY_RELOADING:
			if activeRequests == 0 {
				confHolder.ReloadConfig()
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
			permCh <- struct{}{}
			activeRequests++
		case <-controller.channels.releasePermission:
			activeRequests--

		case stateCh := <-controller.channels.info:
			stateCh <- &ControllerInfo{
				ActiveRequests:  activeRequests,
				WaitingRequests: len(controller.channels.requestPermission),
				State:           state,
				Config:          confHolder.GetConfig()}

		case cmd := <-controller.channels.command:
			switch cmd {
			case CMD_PAUSE:
				state = PROXY_PAUSING
			case CMD_UNPAUSE:
				state = PROXY_RUNNING
			case CMD_RELOAD:
				state = PROXY_RELOADING
			case CMD_STOP:
				state = PROXY_STOPPING
			default:
				log.Print("Unknown controller command:", cmd)
			}
		}
	}
	controller.channels = nil
}

func (controller *ProxyController) enterExecution() {
	ch := make(chan struct{})
	controller.channels.requestPermission <- ch
	<-ch
}

func (controller *ProxyController) leaveExecution() {
	controller.channels.releasePermission <- struct{}{}
}

func (controller *ProxyController) CallUplink(block func() (*RespMsg, error)) (*RespMsg, error) {
	controller.enterExecution()
	defer controller.leaveExecution()

	return block()
}

func (controller *ProxyController) GetInfo() *ControllerInfo {
	if controller.channels == nil {
		return &ControllerInfo{State: PROXY_STOPPED}
	}
	ch := make(chan *ControllerInfo)
	controller.channels.info <- ch
	return <-ch
}

func (controller *ProxyController) Pause() {
	controller.channels.command <- CMD_PAUSE
}

func (controller *ProxyController) PauseAndWait() {
	// TODO: push the state change instead of having the client
	// poll
	controller.channels.command <- CMD_PAUSE
	for {
		info := controller.GetInfo()
		if info.ActiveRequests == 0 {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (controller *ProxyController) Unpause() {
	controller.channels.command <- CMD_UNPAUSE
}

func (controller *ProxyController) Reload() {
	controller.channels.command <- CMD_RELOAD
}

func (controller *ProxyController) Start(ch ProxyConfigHolder) {
	go controller.run(ch)
	for {
		if controller.GetInfo().State == PROXY_RUNNING {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (controller *ProxyController) Stop() {
	controller.channels.command <- CMD_STOP
	for {
		if controller.channels == nil {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

////////////////////////////////////////
// ControllerInfo

type ControllerInfo struct {
	ActiveRequests  int
	WaitingRequests int
	State           int
	Config          *ProxyConfig
}

func (ci *ControllerInfo) StateStr() string {
	switch ci.State {
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
