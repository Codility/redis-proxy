package rproxy

import (
	"strconv"
	"time"

	"gitlab.codility.net/marcink/redis-proxy/resp"
)

////////////////////////////////////////
// ProxyController interface

const (
	PROXY_STOPPED = ControllerState(iota)
	PROXY_RUNNING
	PROXY_PAUSING
	PROXY_PAUSED
	PROXY_RELOADING
	PROXY_STOPPING

	CMD_PAUSE = ControllerCommand(iota)
	CMD_UNPAUSE
	CMD_RELOAD
	CMD_STOP

	MAX_CONNECTIONS = 1000
)

type ProxyController struct {
	proc *ProxyControllerProc
}

type ControllerState int
type ControllerCommand int

func (state ControllerState) String() string {
	switch state {
	case PROXY_RUNNING:
		return "running"
	case PROXY_PAUSING:
		return "pausing"
	case PROXY_PAUSED:
		return "paused"
	case PROXY_RELOADING:
		return "reloading"
	case PROXY_STOPPING:
		return "stopping"
	default:
		return "unknown:" + strconv.Itoa(int(state))
	}
}

type ControllerInfo struct {
	ActiveRequests  int
	WaitingRequests int
	State           ControllerState
	Config          *ProxyConfig
}

func NewProxyController() *ProxyController {
	return &ProxyController{}
}

func (controller *ProxyController) CallUplink(block func() (*resp.Msg, error)) (*resp.Msg, error) {
	controller.enterExecution()
	defer controller.leaveExecution()

	return block()
}

func (controller *ProxyController) GetInfo() *ControllerInfo {
	if controller.proc == nil {
		return &ControllerInfo{State: PROXY_STOPPED}
	}
	ch := make(chan *ControllerInfo)
	controller.proc.channels.info <- ch
	return <-ch
}

func (controller *ProxyController) Pause() {
	controller.proc.channels.command <- CMD_PAUSE
}

func (controller *ProxyController) PauseAndWait() {
	// TODO: push the state change instead of having the client
	// poll
	controller.proc.channels.command <- CMD_PAUSE
	for {
		if controller.GetInfo().ActiveRequests == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (controller *ProxyController) Unpause() {
	controller.proc.channels.command <- CMD_UNPAUSE
}

func (controller *ProxyController) Reload() {
	controller.proc.channels.command <- CMD_RELOAD
}

func (controller *ProxyController) ReloadAndWait() {
	controller.Reload()
	for {
		if controller.GetInfo().State == PROXY_RUNNING {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (controller *ProxyController) Start(ch ProxyConfigHolder) {
	go controller.run(ch)
	for {
		if controller.GetInfo().State == PROXY_RUNNING {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (controller *ProxyController) Stop() {
	controller.proc.channels.command <- CMD_STOP
	for {
		if controller.proc == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (controller *ProxyController) Alive() bool {
	return controller.proc != nil
}

////////////////////////////////////////
// ProxyController implementation

func (controller *ProxyController) run(confHolder ProxyConfigHolder) {
	controller.proc = NewProxyControllerProc(confHolder)
	defer func() {
		controller.proc = nil
	}()
	controller.proc.run()
}

func (controller *ProxyController) enterExecution() {
	ch := make(chan struct{})
	controller.proc.channels.requestPermission <- ch
	<-ch
}

func (controller *ProxyController) leaveExecution() {
	controller.proc.channels.releasePermission <- struct{}{}
}
