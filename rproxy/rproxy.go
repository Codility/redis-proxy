package rproxy

import (
	"log"
	"net"
	"time"

	"github.com/Codility/redis-proxy/resp"
)

const (
	// MaxConnections: not enforced, used only to ensure enough
	// space in request/release channels to make it easy to
	// measure current state.
	//
	// TODO: enforce?
	MaxConnections = 1000
)

type ConfigHolder interface {
	ReloadConfig()
	GetConfig() *Config
}

type Proxy struct {
	configLoader          ConfigLoader
	config                *Config
	listenAddr, adminAddr *net.Addr

	channels       ProxyChannels
	activeRequests int
	state          ProxyState
}

type ProxyChannels struct {
	requestPermission chan chan struct{}
	releasePermission chan struct{}
	info              chan chan *ProxyInfo
	command           chan ProxyCommand
}

////////////////////////////////////////
// Interface

func NewProxy(cl ConfigLoader) (*Proxy, error) {
	config, err := cl.Load()
	if err != nil {
		return nil, err
	}

	errList := config.Prepare()
	if !errList.Ok() {
		err := errList.AsError()
		log.Print(err)
		return nil, err
	}

	proxy := &Proxy{
		channels: ProxyChannels{
			requestPermission: make(chan chan struct{}, MaxConnections),
			releasePermission: make(chan struct{}, MaxConnections),
			info:              make(chan chan *ProxyInfo),
			command:           make(chan ProxyCommand),
		},
		configLoader: cl,
		config:       config,
	}
	return proxy, nil
}

func (proxy *Proxy) Start() {
	if proxy.State() != ProxyStopped {
		return
	}
	go proxy.Run()
	for proxy.State() != ProxyRunning {
		time.Sleep(50 * time.Millisecond)
	}
}

func (proxy *Proxy) ListenAddr() net.Addr {
	return *proxy.listenAddr
}

func (proxy *Proxy) AdminAddr() net.Addr {
	return *proxy.adminAddr
}

func (proxy *Proxy) RequiresClientAuth() bool {
	return proxy.config.Listen.Pass != ""
}

func (proxy *Proxy) State() ProxyState {
	return proxy.state
}

func (proxy *Proxy) SetState(st ProxyState) {
	proxy.state = st
}

func (proxy *Proxy) ReloadConfig() {
	newConfig, err := proxy.configLoader.Load()
	if err != nil {
		log.Printf("Got an error while loading %v: %s.  Keeping old config.", proxy, err)
		return
	}

	if err := proxy.verifyNewConfig(newConfig); err != nil {
		log.Printf("Can not reload into new config: %s.  Keeping old config.", err)
		return
	}
	proxy.config = newConfig
}

func (proxy *Proxy) Pause() {
	proxy.channels.command <- CmdPause
}

func (proxy *Proxy) PauseAndWait() {
	// TODO: push the state change instead of having the client
	// poll
	proxy.channels.command <- CmdPause
	for proxy.GetInfo().ActiveRequests > 0 {
		time.Sleep(50 * time.Millisecond)
	}
}

func (proxy *Proxy) Unpause() {
	proxy.channels.command <- CmdUnpause
}

func (proxy *Proxy) Reload() {
	proxy.channels.command <- CmdReload
}

func (proxy *Proxy) ReloadAndWait() {
	proxy.Reload()
	for proxy.GetInfo().State != ProxyRunning {
		time.Sleep(50 * time.Millisecond)
	}
}

func (proxy *Proxy) Stop() {
	proxy.channels.command <- CmdStop
	for proxy.State() != ProxyStopped {
		time.Sleep(50 * time.Millisecond)
	}
}

func (proxy *Proxy) GetConfig() *Config {
	return proxy.config
}

func (proxy *Proxy) GetInfo() *ProxyInfo {
	ch := make(chan *ProxyInfo)
	proxy.channels.info <- ch
	return <-ch
}

func (proxy *Proxy) verifyNewConfig(newConfig *Config) error {
	errList := newConfig.Prepare()
	if !errList.Ok() {
		return errList.AsError()
	}

	return proxy.config.ValidateSwitchTo(newConfig)
}

func (proxy *Proxy) CallUplink(block func() (*resp.Msg, error)) (*resp.Msg, error) {
	proxy.enterExecution()
	defer proxy.leaveExecution()

	return block()
}

func (proxy *Proxy) enterExecution() {
	ch := make(chan struct{})
	proxy.channels.requestPermission <- ch
	<-ch
}

func (proxy *Proxy) leaveExecution() {
	proxy.channels.releasePermission <- struct{}{}
}
