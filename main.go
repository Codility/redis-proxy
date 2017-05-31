package main

// TODO: add TLS to listener and admin
// TODO: add TLS to uplink (including reloads)
// TODO: better names for resp New*er
// TODO: proper logging
// TODO: guard config access with a mutex
// TODO: keepalive?

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const CONFIG_FILE = "config.json"
const READ_TIME_LIMIT = 5 * time.Second

func main() {
	config, err := loadConfig(CONFIG_FILE)
	if err != nil {
		panic(err)
	}
	proxy := NewRedisProxy(config)
	proxy.run()
}

////////////////////////////////////////
// RedisProxyConfig

type RedisProxyConfig struct {
	UplinkAddr   string `json:"uplink_addr"`
	UplinkUseTLS bool   `json:"uplink_use_tls"`
	ListenOn     string `json:"listen_on"`
	AdminOn      string `json:"admin_on"`
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

const TICKET_COUNT = 100

type RedisProxy struct {
	config                *RedisProxyConfig
	enterExecutionChannel chan bool
	leaveExecutionChannel chan bool
}

func NewRedisProxy(config *RedisProxyConfig) *RedisProxy {
	return &RedisProxy{
		config:                config,
		enterExecutionChannel: make(chan bool, TICKET_COUNT),
		leaveExecutionChannel: make(chan bool, TICKET_COUNT)}
}

func (proxy *RedisProxy) run() {
	listener, err := net.Listen("tcp", proxy.config.ListenOn)
	if err != nil {
		panic(err)
	}

	fmt.Println("Listening on", proxy.config.ListenOn)

	go proxy.watchSignals()
	go proxy.executionLimiter()
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
	newConfig, err := loadConfig(CONFIG_FILE)
	if err != nil {
		fmt.Printf("Got an error while loading %s: %s.  Keeping old config.", CONFIG_FILE, err)
		return
	}

	if err := proxy.verifyNewConfig(newConfig); err != nil {
		fmt.Printf("Can not reload into new config: %s.  Keeping old config.", err)
		return
	}
	proxy.config = newConfig
}

func (proxy *RedisProxy) watchSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	for {
		s := <-c
		fmt.Printf("Got signal: %v, reloading config\n", s)
		proxy.reloadConfig()
	}
}

func (proxy *RedisProxy) activeRequests() int {
	return TICKET_COUNT - len(proxy.enterExecutionChannel)
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

func (proxy *RedisProxy) executionLimiter() {
	for i := 0; i < TICKET_COUNT; i++ {
		proxy.enterExecutionChannel <- true
	}
	for {
		<-proxy.leaveExecutionChannel
		proxy.enterExecutionChannel <- true
	}
}

func (proxy *RedisProxy) enterExecution() {
	<-proxy.enterExecutionChannel
}

func (proxy *RedisProxy) leaveExecution() {
	proxy.leaveExecutionChannel <- true
}

func (proxy *RedisProxy) executeCall(block func() ([]byte, error)) ([]byte, error) {
	proxy.enterExecution()
	defer proxy.leaveExecution()

	return block()
}

func (proxy *RedisProxy) handleClient(cliConn net.Conn) {
	fmt.Println("Handling new client:", cliConn)

	// TODO: catch and log panics
	defer cliConn.Close()
	cliReader := NewReader(bufio.NewReader(cliConn))
	cliWriter := bufio.NewWriter(cliConn)

	uplinkAddr := proxy.config.UplinkAddr

	fmt.Println("Dialing", uplinkAddr)
	uplinkConn, err := net.Dial("tcp", uplinkAddr)
	if err != nil {
		fmt.Printf("Dial error: %v\n", err)
		return
	}
	defer func() {
		// TODO: does this actually close the right connection
		// after reloads?
		uplinkConn.Close()
	}()
	uplinkReader := NewReader(bufio.NewReader(uplinkConn))
	uplinkWriter := bufio.NewWriter(uplinkConn)

	for {
		req, err := cliReader.ReadObject()
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			return
		}

		resp, err := proxy.executeCall(func() ([]byte, error) {
			currUplinkAddr := proxy.config.UplinkAddr
			if uplinkAddr != currUplinkAddr {
				uplinkAddr = currUplinkAddr
				fmt.Println("Redialing", uplinkAddr)
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

			uplinkConn.SetReadDeadline(time.Now().Add(READ_TIME_LIMIT))
			return uplinkReader.ReadObject()
		})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		cliWriter.Write(resp)
		cliWriter.Flush()
	}
}
