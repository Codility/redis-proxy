package rproxy

import (
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitlab.codility.net/marcink/redis-proxy/resp"
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
	genListener, err := net.Listen("tcp", proxy.config.Listen.Addr)
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
			if resp.IsNetTimeout(err) {
				continue
			}
			return err
		} else {
			go proxy.handleClient(resp.NewConn(conn, 0, proxy.config.LogMessages))
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

func (proxy *Proxy) RequiresClientAuth() bool {
	return proxy.config.Listen.Pass != ""
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
	if !config.Listen.Equal(&newConfig.Listen) {
		return errors.New("New config must have the same `listen` block as the old one.")
	}
	if !config.Admin.Equal(&newConfig.Admin) {
		return errors.New("New config must have the same `admin` block as the old one.")
	}
	return nil
}

func (proxy *Proxy) handleClient(cliConn *resp.Conn) {
	log.Printf("Handling new client: connection from %s", cliConn.RemoteAddr())

	uplinkConf := &AddrSpec{}
	var uplinkConn *resp.Conn
	cliAuthenticated := false
	db := 0

	defer func() {
		cliConn.Close()
		if uplinkConn != nil {
			uplinkConn.Close()
		}
	}()

	for {
		req, err := cliConn.ReadMsg()
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error: %v\n", err)
			}
			return
		}

		if req.Op() == resp.MsgOpBroken {
			cliConn.Write(resp.MsgParseError)
			return
		}

		if req.Op() == resp.MsgOpAuth {
			if proxy.RequiresClientAuth() {
				cliAuthenticated = (req.FirstArg() == proxy.config.Listen.Pass)
				if cliAuthenticated {
					cliConn.Write([]byte(resp.MsgOk))
				} else {
					cliConn.Write([]byte(resp.MsgInvalidPass))
				}
			} else {
				cliConn.Write([]byte(resp.MsgNoPasswordSet))
			}
			continue
		}

		if proxy.RequiresClientAuth() && !cliAuthenticated {
			cliConn.Write([]byte(resp.MsgNoAuth))
			continue
		}

		res, err := proxy.controller.CallUplink(func() (*resp.Msg, error) {
			config := proxy.config
			currUplinkConf := &config.Uplink
			if !uplinkConf.Equal(currUplinkConf) {
				uplinkConf = currUplinkConf
				if uplinkConn != nil {
					uplinkConn.Close()
				}
				uplinkConn, err = resp.Dial("tcp", uplinkConf.Addr,
					config.ReadTimeLimitMs,
					config.LogMessages,
				)
				if err != nil {
					return nil, err
				}

				if uplinkConf.Pass != "" {
					if err := uplinkConn.Authenticate(uplinkConf.Pass); err != nil {
						return nil, err
					}
				}

				if db != 0 {
					if err := uplinkConn.Select(db); err != nil {
						return nil, err
					}
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

		if (req.Op() == resp.MsgOpSelect) && res.IsOk() {
			db = req.FirstArgInt()
		}

		cliConn.WriteMsg(res)
	}
}
