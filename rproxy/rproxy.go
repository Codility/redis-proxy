package rproxy

import (
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type ProxyConfigHolder interface {
	ReloadConfig()
	GetConfig() *ProxyConfig
}

type Proxy struct {
	configLoader ConfigLoader
	config       *ProxyConfig
	controller   *ProxyController
	listenAddr   *net.Addr
}

func NewProxy(cl ConfigLoader) (*Proxy, error) {
	config, err := cl.Load()
	if err != nil {
		return nil, err
	}
	proxy := &Proxy{
		configLoader: cl,
		config:       config,
		controller:   NewProxyController()}
	return proxy, nil
}

func (proxy *Proxy) Run() error {
	genListener, err := net.Listen("tcp", proxy.config.ListenOn)
	if err != nil {
		return err
	}
	defer genListener.Close()
	listener := genListener.(*net.TCPListener)

	addr := listener.Addr()
	proxy.listenAddr = &addr

	log.Println("Listening on", proxy.ListenAddr())

	proxy.controller.Start(proxy) // TODO: clean this up when getting rid of circular dep
	go proxy.watchSignals()
	go proxy.publishAdminInterface()

	for proxy.controller.Alive() {
		listener.SetDeadline(time.Now().Add(time.Second))
		conn, err := listener.Accept()
		if err != nil {
			if IsTimeout(err) {
				continue
			}
			return err
		} else {
			go proxy.handleClient(NewRespConn(conn, 0, proxy.config.LogMessages))
		}
	}
	return nil
}

func (proxy *Proxy) Alive() bool {
	return proxy.controller.Alive()
}

func (proxy *Proxy) ListenAddr() net.Addr {
	return *proxy.listenAddr
}

func (proxy *Proxy) ReloadConfig() {
	newConfig, err := proxy.configLoader.Load()
	if err != nil {
		log.Printf("Got an error while loading %s: %s.  Keeping old config.", proxy, err)
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
