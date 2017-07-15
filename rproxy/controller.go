package rproxy

import (
	"time"

	"github.com/Codility/redis-proxy/resp"
)

////////////////////////////////////////
// Controller interface

const (
	ProxyStarting = ProxyState(iota)
	ProxyRunning
	ProxyPausing
	ProxyPaused
	ProxyReloading
	ProxyStopping

	CmdPause = ProxyCommand(iota)
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

type ProxyState int
type ProxyCommand int

var proxyStateTxt = [...]string{
	"starting",
	"running",
	"pausing",
	"reloading",
	"stopping",
}

func (s ProxyState) String() string {
	return proxyStateTxt[s]
}

type ProxyInfo struct {
	ActiveRequests  int
	WaitingRequests int
	State           ProxyState
	Config          *Config
}

func (proxy *Proxy) CallUplink(block func() (*resp.Msg, error)) (*resp.Msg, error) {
	proxy.enterExecution()
	defer proxy.leaveExecution()

	return block()
}

func (proxy *Proxy) GetInfo() *ProxyInfo {
	if proxy.proc == nil {
		return &ProxyInfo{State: ProxyStarting}
	}
	ch := make(chan *ProxyInfo)
	proxy.proc.channels.info <- ch
	return <-ch
}

func (proxy *Proxy) Pause() {
	proxy.proc.channels.command <- CmdPause
}

func (proxy *Proxy) PauseAndWait() {
	// TODO: push the state change instead of having the client
	// poll
	proxy.proc.channels.command <- CmdPause
	for {
		if proxy.GetInfo().ActiveRequests == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (proxy *Proxy) Unpause() {
	proxy.proc.channels.command <- CmdUnpause
}

func (proxy *Proxy) Reload() {
	proxy.proc.channels.command <- CmdReload
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
	proxy.proc.channels.command <- CmdStop
	for {
		if proxy.proc == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

////////////////////////////////////////
// Controller implementation

func (proxy *Proxy) runProc() {
	proxy.proc = NewProxyProc(proxy)
	defer func() {
		proxy.proc = nil
	}()
	proxy.proc.run()
}

func (proxy *Proxy) enterExecution() {
	ch := make(chan struct{})
	proxy.proc.channels.requestPermission <- ch
	<-ch
}

func (proxy *Proxy) leaveExecution() {
	proxy.proc.channels.releasePermission <- struct{}{}
}
