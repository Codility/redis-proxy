package rproxy

import "time"

////////////////////////////////////////
// ProxyController interface

const (
	PROXY_STOPPED = ControllerState(iota)
	PROXY_RUNNING
	PROXY_PAUSING
	PROXY_PAUSED
	PROXY_RELOADING

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
	default:
		return "unknown"
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

func (controller *ProxyController) CallUplink(block func() (*RespMsg, error)) (*RespMsg, error) {
	controller.enterExecution()
	defer controller.leaveExecution()

	return block()
}

func (controller *ProxyController) GetInfo() *ControllerInfo {
	if controller.proc == nil {
		return &ControllerInfo{State: PROXY_STOPPED}
	}
	ch := make(chan *ControllerInfo)
	controller.proc.info <- ch
	return <-ch
}

func (controller *ProxyController) Pause() {
	controller.proc.command <- CMD_PAUSE
}

func (controller *ProxyController) PauseAndWait() {
	// TODO: push the state change instead of having the client
	// poll
	controller.proc.command <- CMD_PAUSE
	for {
		if controller.GetInfo().ActiveRequests == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (controller *ProxyController) Unpause() {
	controller.proc.command <- CMD_UNPAUSE
}

func (controller *ProxyController) Reload() {
	controller.proc.command <- CMD_RELOAD
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
	controller.proc.command <- CMD_STOP
	for {
		if controller.proc == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

////////////////////////////////////////
// ProxyController implementation

func (controller *ProxyController) run(confHolder ProxyConfigHolder) {
	controller.proc = NewProxyControllerProc()
	defer func() {
		controller.proc = nil
	}()
	controller.proc.run(confHolder)
}

func (controller *ProxyController) enterExecution() {
	ch := make(chan struct{})
	controller.proc.requestPermission <- ch
	<-ch
}

func (controller *ProxyController) leaveExecution() {
	controller.proc.releasePermission <- struct{}{}
}
