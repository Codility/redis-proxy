package rproxy

import (
	"time"

	"github.com/Codility/redis-proxy/resp"
)

////////////////////////////////////////
// Controller interface

const (
	ProxyStarting = ControllerState(iota)
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

type ControllerState int
type ControllerCommand int

var controllerStateTxt = [...]string{
	"starting",
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

func (proxy *Proxy) CallUplink(block func() (*resp.Msg, error)) (*resp.Msg, error) {
	proxy.enterExecution()
	defer proxy.leaveExecution()

	return block()
}

func (proxy *Proxy) GetInfo() *ControllerInfo {
	if proxy.controllerProc == nil {
		return &ControllerInfo{State: ProxyStarting}
	}
	ch := make(chan *ControllerInfo)
	proxy.controllerProc.channels.info <- ch
	return <-ch
}

func (proxy *Proxy) Pause() {
	proxy.controllerProc.channels.command <- CmdPause
}

func (proxy *Proxy) PauseAndWait() {
	// TODO: push the state change instead of having the client
	// poll
	proxy.controllerProc.channels.command <- CmdPause
	for {
		if proxy.GetInfo().ActiveRequests == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (proxy *Proxy) Unpause() {
	proxy.controllerProc.channels.command <- CmdUnpause
}

func (proxy *Proxy) Reload() {
	proxy.controllerProc.channels.command <- CmdReload
}

func (proxy *Proxy) ReloadAndWait() {
	proxy.Reload()
	for {
		if proxy.GetInfo().State == ProxyRunning {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (proxy *Proxy) Stop() {
	proxy.controllerProc.channels.command <- CmdStop
	for {
		if proxy.controllerProc == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

////////////////////////////////////////
// Controller implementation

func (proxy *Proxy) runControllerProc() {
	proxy.controllerProc = NewControllerProc(proxy)
	defer func() {
		proxy.controllerProc = nil
	}()
	proxy.controllerProc.run()
}

func (proxy *Proxy) enterExecution() {
	ch := make(chan struct{})
	proxy.controllerProc.channels.requestPermission <- ch
	<-ch
}

func (proxy *Proxy) leaveExecution() {
	proxy.controllerProc.channels.releasePermission <- struct{}{}
}
