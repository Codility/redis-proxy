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
	"sync"
	"syscall"
)

const CONFIG_FILE = "config.json"

func main() {
	config, err := loadConfig(CONFIG_FILE)
	if err != nil {
		panic(err)
	}
	proxy := RedisProxy{config: config}
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

func (config *RedisProxyConfig) canReload(newConfig *RedisProxyConfig) error {
	if config.ListenOn != newConfig.ListenOn {
		return errors.New("New config must have the same listen_on address as the old one.")
	}
	if config.AdminOn != newConfig.AdminOn {
		return errors.New("New config must have the same admin_on address as the old one.")
	}
	return nil
}

////////////////////////////////////////
// RedisProxy

type RedisProxy struct {
	mu     sync.Mutex
	config *RedisProxyConfig
}

func (proxy *RedisProxy) run() {
	listener, err := net.Listen("tcp", proxy.config.ListenOn)
	if err != nil {
		panic(err)
	}

	fmt.Println("Listening on", proxy.config.ListenOn)

	go proxy.watchSignals()

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go proxy.handleClient(conn)
	}
}

func (proxy *RedisProxy) getConfig() *RedisProxyConfig {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()

	return proxy.config
}

func (proxy *RedisProxy) setConfig(config *RedisProxyConfig) {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()

	proxy.config = config
}

func (proxy *RedisProxy) reloadConfig() {
	newConfig, err := loadConfig(CONFIG_FILE)
	if err != nil {
		fmt.Printf("Got an error while loading %s: %s.  Keeping old config.", CONFIG_FILE, err)
		return
	}

	if err := newConfig.canReload(proxy.getConfig()); err != nil {
		fmt.Printf("Can not reload into new config: %s.  Keeping old config.", err)
		return
	}
	proxy.setConfig(newConfig)
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

func (proxy *RedisProxy) handleClient(cliConn net.Conn) {
	// TODO: catch and log panics
	defer cliConn.Close()
	cliReader := NewReader(bufio.NewReader(cliConn))
	cliWriter := bufio.NewWriter(cliConn)

	config := proxy.getConfig()

	fmt.Println("Dialing", config.UplinkAddr)
	uplinkConn, err := net.Dial("tcp", config.UplinkAddr)
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

		currConfig := proxy.getConfig()
		if config != currConfig {
			config = currConfig
			fmt.Println("Redialing", config.UplinkAddr)
			uplinkConn.Close()
			uplinkConn, err = net.Dial("tcp", config.UplinkAddr)
			if err != nil {
				fmt.Printf("Dial error: %v\n", err)
				return
			}
			uplinkReader = NewReader(bufio.NewReader(uplinkConn))
			uplinkWriter = bufio.NewWriter(uplinkConn)
		}

		uplinkWriter.Write(req)
		uplinkWriter.Flush()

		resp, err := uplinkReader.ReadObject()
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			return
		}

		cliWriter.Write(resp)
		cliWriter.Flush()
	}
}
