package rproxy

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type ConfigHolder interface {
	ReloadConfig()
	GetConfig() *Config
}

type Proxy struct {
	configLoader          ConfigLoader
	config                *Config
	listenAddr, adminAddr *net.Addr
	controllerProc        *ControllerProc
}

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
		configLoader: cl,
		config:       config}
	return proxy, nil
}

func (proxy *Proxy) RunAndReport(doneChan chan struct{}) error {
	proxy.publishAdminInterface()

	if err := proxy.startConnAcceptor(); err != nil {
		return err
	}
	log.Println("Listening on", proxy.ListenAddr())
	go proxy.watchSignals()

	if doneChan != nil {
		doneChan <- struct{}{}
	}

	proxy.runControllerProc()

	return nil
}

func (proxy *Proxy) Run() error {
	return proxy.RunAndReport(nil)
}

func (proxy *Proxy) Start() {
	doneChan := make(chan struct{})
	go proxy.RunAndReport(doneChan)
	<-doneChan
}

func (proxy *Proxy) Alive() bool {
	return proxy.controllerProc != nil
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

func (proxy *Proxy) GetConfig() *Config {
	return proxy.config
}

func (proxy *Proxy) watchSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	for {
		s := <-c
		log.Printf("Got signal: %v, reloading config\n", s)
		proxy.Reload()
	}
}

func (proxy *Proxy) verifyNewConfig(newConfig *Config) error {
	errList := newConfig.Prepare()
	if !errList.Ok() {
		return errList.AsError()
	}

	return proxy.config.ValidateSwitchTo(newConfig)
}
