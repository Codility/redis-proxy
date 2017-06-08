package rproxy

import (
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type ProxyConfigHolder interface {
	ReloadConfig()
	GetConfig() *ProxyConfig
}

type Proxy struct {
	config_file string
	config      *ProxyConfig
	controller  *ProxyController
}

func NewProxy(config_file string) (*Proxy, error) {
	config, err := LoadConfig(config_file)
	if err != nil {
		return nil, err
	}
	proxy := &Proxy{
		config_file: config_file,
		config:      config,
		controller:  NewProxyController()}
	return proxy, nil
}

func (proxy *Proxy) Run() error {
	listener, err := net.Listen("tcp", proxy.config.ListenOn)
	if err != nil {
		return err
	}

	log.Println("Listening on", proxy.config.ListenOn)

	proxy.controller.Start(proxy) // TODO: clean this up when getting rid of circular dep
	go proxy.watchSignals()
	go proxy.publishAdminInterface()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go proxy.handleClient(NewRespConn(conn, 0, proxy.config.LogMessages))
	}
}

func (proxy *Proxy) ReloadConfig() {
	newConfig, err := LoadConfig(proxy.config_file)
	if err != nil {
		log.Printf("Got an error while loading %s: %s.  Keeping old config.", proxy.config_file, err)
		return
	}

	if err := proxy.verifyNewConfig(newConfig); err != nil {
		log.Printf("Can not reload into new config: %s.  Keeping old config.", err)
		return
	}
	proxy.config = newConfig
}

func (proxy *Proxy) GetConfig() *ProxyConfig {
	return proxy.config
}

func (proxy *Proxy) watchSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	for {
		s := <-c
		log.Printf("Got signal: %v, reloading config\n", s)
		proxy.controller.Reload()
	}
}

func (proxy *Proxy) verifyNewConfig(newConfig *ProxyConfig) error {
	config := proxy.config
	if config.ListenOn != newConfig.ListenOn {
		return errors.New("New config must have the same listen_on address as the old one.")
	}
	if config.AdminOn != newConfig.AdminOn {
		return errors.New("New config must have the same admin_on address as the old one.")
	}
	return nil
}

func (proxy *Proxy) handleClient(cliConn *RespConn) {
	log.Printf("Handling new client: connection from %s", cliConn.RemoteAddr())

	uplinkAddr := ""
	var uplinkConn *RespConn

	defer func() {
		cliConn.Close()
		if uplinkConn != nil {
			uplinkConn.Close()
		}
	}()

	for {
		req, err := cliConn.ReadMsg()
		if err != nil {
			log.Printf("Read error: %v\n", err)
			return
		}

		res, err := proxy.controller.CallUplink(func() (*RespMsg, error) {
			currUplinkAddr := proxy.config.UplinkAddr
			if uplinkAddr != currUplinkAddr {
				uplinkAddr = currUplinkAddr
				if uplinkConn != nil {
					uplinkConn.Close()
				}
				uplinkConn, err = RespDial("tcp", uplinkAddr,
					proxy.config.ReadTimeLimitMs,
					proxy.config.LogMessages,
				)
				if err != nil {
					return nil, err
				}
			}

			_, err := uplinkConn.WriteMsg(req)
			if err != nil {
				return nil, err
			}
			return uplinkConn.ReadMsg()
		})
		if err != nil {
			log.Printf("Error: %v\n", err)
			return
		}

		cliConn.WriteMsg(res)
	}
}
