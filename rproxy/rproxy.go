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
	configLoader ConfigLoader
	config       *Config
	listenAddr   *net.Addr
	adminUI      *AdminUI

	channels       ProxyChannels
	activeRequests int
	state          ProxyState
}

type ProxyChannels struct {
	requestPermission chan chan struct{}
	releasePermission chan struct{}
	info              chan chan *ProxyInfo
	command           chan commandCall
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
			command:           make(chan commandCall),
		},
		configLoader: cl,
		config:       config,
	}
	return proxy, nil
}

func (proxy *Proxy) Start() {
	log.Print("Start starts")
	if proxy.State() != ProxyStopped {
		return
	}
	log.Print("Start mids")
	go proxy.Run()
	log.Print("Start waits")
	for proxy.State() != ProxyRunning {
		time.Sleep(50 * time.Millisecond)
	}
	log.Print("Start ends")
}

func (proxy *Proxy) ListenAddr() net.Addr {
	return *proxy.listenAddr
}

func (proxy *Proxy) AdminAddr() net.Addr {
	return *proxy.adminUI.Addr
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

func (proxy *Proxy) ReloadConfig() error {
	newConfig, err := proxy.configLoader.Load()
	if err != nil {
		log.Printf("Got an error while loading %v: %s.  Keeping old config.", proxy, err)
		return err
	}

	if err := proxy.verifyNewConfig(newConfig); err != nil {
		log.Printf("Can not reload into new config: %s.  Keeping old config.", err)
		return err
	}
	proxy.config = newConfig
	return nil
}

func (proxy *Proxy) Pause() error {
	return proxy.command(CmdPause).err
}

func (proxy *Proxy) Unpause() error {
	return proxy.command(CmdUnpause).err
}

func (proxy *Proxy) Reload() error {
	return proxy.command(CmdReload).err
}

func (proxy *Proxy) Stop() error {
	return proxy.command(CmdStop).err
}

func (proxy *Proxy) GetConfig() *Config {
	return proxy.config
}

func (proxy *Proxy) command(cmd command) commandResponse {
	rc := make(chan commandResponse, 1)
	proxy.channels.command <- commandCall{cmd, rc}
	return <-rc
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
