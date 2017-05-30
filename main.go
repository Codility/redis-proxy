package main

// TODO: add TLS to listener and admin
// TODO: add TLS to uplink
// TODO: better names for resp New*er
// TODO: proper logging
// TODO: guard config access with a mutex
// TODO: keepalive?

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
)

func main() {
	config, err := parseConfig("config.json")
	if err != nil {
		panic(err)
	}
	proxy := RedisProxy{config}
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

func parseConfig(fname string) (*RedisProxyConfig, error) {
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
	config *RedisProxyConfig
}

func (proxy *RedisProxy) run() {
	listener, err := net.Listen("tcp", proxy.config.ListenOn)
	if err != nil {
		panic(err)
	}

	fmt.Println("Listening on", proxy.config.ListenOn)

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go proxy.handleClient(conn)
	}
}

func (proxy *RedisProxy) handleClient(cliConn net.Conn) {
	defer cliConn.Close()
	cliReader := NewReader(bufio.NewReader(cliConn))
	cliWriter := bufio.NewWriter(cliConn)

	// TODO: catch and log panics
	fmt.Println("Dialing", proxy.config.UplinkAddr)
	uplinkConn, err := net.Dial("tcp", proxy.config.UplinkAddr)
	if err != nil {
		panic(err)
	}
	defer uplinkConn.Close()
	uplinkReader := NewReader(bufio.NewReader(uplinkConn))
	uplinkWriter := bufio.NewWriter(uplinkConn)

	for {
		req, err := cliReader.ReadObject()
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			return
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
