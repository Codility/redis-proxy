package rproxy

import (
	"time"

	"github.com/codility/redis-proxy/resp"
)

////////////////////////////////////////
// Controller interface

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

type Controller struct {
	proc *ControllerProc
}

type ControllerState int
type ControllerCommand int

var controllerStateTxt = [...]string{
	"stopped",
	"running",
	"pausing",
	"reloading",
	"stopping",
}

func (s ControllerState) String() string {
	return controllerStateTxt[s]
}

type ControllerInfo struct {
	ActiveRequests  int
	WaitingRequests int
	State           ControllerState
	Config          *Config
}

func NewController() *Controller {
	return &Controller{}
}

func (controller *Controller) CallUplink(block func() (*resp.Msg, error)) (*resp.Msg, error) {
	controller.enterExecution()
	defer controller.leaveExecution()

	return block()
}

func (controller *Controller) GetInfo() *ControllerInfo {
	if controller.proc == nil {
		return &ControllerInfo{State: ProxyStopped}
	}
	ch := make(chan *ControllerInfo)
	controller.proc.channels.info <- ch
	return <-ch
}

func (controller *Controller) Pause() {
	controller.proc.channels.command <- CmdPause
}

func (controller *Controller) PauseAndWait() {
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

func (controller *Controller) Unpause() {
	controller.proc.channels.command <- CmdUnpause
}

func (controller *Controller) Reload() {
	controller.proc.channels.command <- CmdReload
}

func (controller *Controller) ReloadAndWait() {
	controller.Reload()
	for {
		if controller.GetInfo().State == ProxyRunning {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (controller *Controller) Start(ch ConfigHolder) {
	go controller.run(ch)
	for {
		if controller.GetInfo().State == ProxyRunning {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (controller *Controller) Stop() {
	controller.proc.channels.command <- CmdStop
	for {
		if controller.proc == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (controller *Controller) Alive() bool {
	return controller.proc != nil
}

////////////////////////////////////////
// Controller implementation

func (controller *Controller) run(confHolder ConfigHolder) {
	controller.proc = NewControllerProc(confHolder)
	defer func() {
		controller.proc = nil
	}()
	controller.proc.run()
}

func (controller *Controller) enterExecution() {
	ch := make(chan struct{})
	controller.proc.channels.requestPermission <- ch
	<-ch
}

func (controller *Controller) leaveExecution() {
	controller.proc.channels.releasePermission <- struct{}{}
}
