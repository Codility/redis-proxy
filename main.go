package main

// TODO: add TLS to listener and admin
// TODO: add TLS to uplink (including reloads)
// TODO: proper logging
// TODO: keepalive?
// TODO: disconnect controller from RedisProxy completely
// TODO: get rid of circular dependency between RedisProxy and ProxyController

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	config_file     = flag.String("f", "config.json", "Config file")
	read_time_limit = flag.Duration("t", 5*time.Second, "Max duration for a query")
	log_messages    = flag.Bool("o", false, "Show requests and responses")
)

func main() {
	flag.Parse()
	config, err := loadConfig(*config_file)
	if err != nil {
		panic(err)
	}
	proxy := NewRedisProxy(config)
	proxy.run()
}

////////////////////////////////////////
// RedisProxyConfig

type RedisProxyConfig struct {
	UplinkAddr string `json:"uplink_addr"`
	ListenOn   string `json:"listen_on"`
	AdminOn    string `json:"admin_on"`
}

func loadConfig(fname string) (*RedisProxyConfig, error) {
	configJson, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var config RedisProxyConfig
	return &config, json.Unmarshal(configJson, &config)
}

////////////////////////////////////////
// RedisProxy

type RedisProxy struct {
	config     *RedisProxyConfig
	controller *ProxyController
}

func NewRedisProxy(config *RedisProxyConfig) *RedisProxy {
	proxy := &RedisProxy{
		config:     config,
		controller: NewProxyController()}
	// TODO: clean this up when getting rid of circular dep
	proxy.controller.proxy = proxy
	return proxy
}

func (proxy *RedisProxy) run() {
	listener, err := net.Listen("tcp", proxy.config.ListenOn)
	if err != nil {
		panic(err)
	}

	log.Println("Listening on", proxy.config.ListenOn)

	go proxy.watchSignals()
	go proxy.controller.run()
	go proxy.publishAdminInterface()

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go proxy.handleClient(conn)
	}
}

func (proxy *RedisProxy) reloadConfig() {
	newConfig, err := loadConfig(*config_file)
	if err != nil {
		log.Printf("Got an error while loading %s: %s.  Keeping old config.", *config_file, err)
		return
	}

	if err := proxy.verifyNewConfig(newConfig); err != nil {
		log.Printf("Can not reload into new config: %s.  Keeping old config.", err)
		return
	}
	proxy.config = newConfig
}

func (proxy *RedisProxy) watchSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	for {
		s := <-c
		log.Printf("Got signal: %v, reloading config\n", s)
		proxy.controller.Reload()
	}
}

func (proxy *RedisProxy) verifyNewConfig(newConfig *RedisProxyConfig) error {
	config := proxy.config
	if config.ListenOn != newConfig.ListenOn {
		return errors.New("New config must have the same listen_on address as the old one.")
	}
	if config.AdminOn != newConfig.AdminOn {
		return errors.New("New config must have the same admin_on address as the old one.")
	}
	return nil
}

func (proxy *RedisProxy) handleClient(cliConn net.Conn) {
	log.Printf("Handling new client: connection from %s", cliConn.RemoteAddr())

	// TODO: catch and log panics
	defer cliConn.Close()
	cliReader := NewReader(bufio.NewReader(cliConn))
	cliWriter := bufio.NewWriter(cliConn)

	uplinkAddr := proxy.config.UplinkAddr

	log.Println("Dialing", uplinkAddr)
	uplinkConn, err := net.Dial("tcp", uplinkAddr)
	if err != nil {
		log.Printf("Dial error: %v\n", err)
		return
	}
	defer func() {
		if uplinkConn != nil {
			uplinkConn.Close()
		}
	}()
	uplinkReader := NewReader(bufio.NewReader(uplinkConn))
	uplinkWriter := bufio.NewWriter(uplinkConn)

	for {
		req, err := cliReader.ReadObject()
		if err != nil {
			log.Printf("Read error: %v\n", err)
			return
		}
		proxy.LogMessage(cliConn.RemoteAddr(), true, req)

		resp, err := proxy.controller.ExecuteCall(func() ([]byte, error) {
			currUplinkAddr := proxy.config.UplinkAddr
			if uplinkAddr != currUplinkAddr {
				uplinkAddr = currUplinkAddr
				log.Println("Redialing", uplinkAddr)
				uplinkConn.Close()
				uplinkConn, err = net.Dial("tcp", uplinkAddr)
				if err != nil {
					return nil, err
				}
				uplinkReader = NewReader(bufio.NewReader(uplinkConn))
				uplinkWriter = bufio.NewWriter(uplinkConn)
			}

			uplinkWriter.Write(req)
			uplinkWriter.Flush()

			uplinkConn.SetReadDeadline(time.Now().Add(*read_time_limit))
			return uplinkReader.ReadObject()
		})
		if err != nil {
			log.Printf("Error: %v\n", err)
			return
		}
		proxy.LogMessage(cliConn.RemoteAddr(), false, resp)

		cliWriter.Write(resp)
		cliWriter.Flush()
	}
}

func (proxy *RedisProxy) LogMessage(addr net.Addr, inbound bool, msg []byte) {
	if !*log_messages {
		return
	}
	dirStr := "<"
	if inbound {
		dirStr = ">"
	}

	msgStr := string(msg)
	msgStr = strings.Replace(msgStr, "\n", "\\n", -1)
	msgStr = strings.Replace(msgStr, "\r", "\\r", -1)

	log.Printf("%s %s %s", addr, dirStr, msgStr)
}
