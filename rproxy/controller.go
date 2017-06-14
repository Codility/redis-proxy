package rproxy

import (
	"strconv"
	"time"

	"gitlab.codility.net/marcink/redis-proxy/resp"
)

////////////////////////////////////////
// ProxyController interface

const (
	ProxyStopped = ControllerState(iota)
	ProxyRunning
	ProxyPausing
	ProxyPaused
	ProxyReloading
	ProxyStopping

	CmdPause = ControllerCommand(iota)
	CmdUnpause
	CmdReload
	CmdStop

	// MaxConnections: not enforced, used only to ensure enough
	// space in request/release channels to make it easy to
	// measure current state.
	//
	// TODO: enforce?
	MaxConnections = 1000
)

type ProxyController struct {
	proc *ProxyControllerProc
}

type ControllerState int
type ControllerCommand int

func (state ControllerState) String() string {
	switch state {
	case ProxyRunning:
		return "running"
	case ProxyPausing:
		return "pausing"
	case ProxyPaused:
		return "paused"
	case ProxyReloading:
		return "reloading"
	case ProxyStopping:
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
		return &ControllerInfo{State: ProxyStopped}
	}
	ch := make(chan *ControllerInfo)
	controller.proc.channels.info <- ch
	return <-ch
}

func (controller *ProxyController) Pause() {
	controller.proc.channels.command <- CmdPause
}

func (controller *ProxyController) PauseAndWait() {
	// TODO: push the state change instead of having the client
	// poll
	controller.proc.channels.command <- CmdPause
	for {
		if controller.GetInfo().ActiveRequests == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (controller *ProxyController) Unpause() {
	controller.proc.channels.command <- CmdUnpause
}

func (controller *ProxyController) Reload() {
	controller.proc.channels.command <- CmdReload
}

func (controller *ProxyController) ReloadAndWait() {
	controller.Reload()
	for {
		if controller.GetInfo().State == ProxyRunning {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (controller *ProxyController) Start(ch ProxyConfigHolder) {
	go controller.run(ch)
	for {
		if controller.GetInfo().State == ProxyRunning {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (controller *ProxyController) Stop() {
	controller.proc.channels.command <- CmdStop
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
