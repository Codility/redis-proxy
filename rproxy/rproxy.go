package rproxy

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	resp "redisgreen.net/resp"
)

////////////////////////////////////////
// RedisProxyConfig

type RedisProxyConfig struct {
	UplinkAddr      string `json:"uplink_addr"`
	ListenOn        string `json:"listen_on"`
	AdminOn         string `json:"admin_on"`
	ReadTimeLimitMs int64  `json:"read_time_limit_ms"`
	LogMessages     bool   `json:"log_messages"`
}

func LoadConfig(fname string) (*RedisProxyConfig, error) {
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
	config_file string
	config      *RedisProxyConfig
	controller  *ProxyController
}

func NewRedisProxy(config_file string) (*RedisProxy, error) {
	config, err := LoadConfig(config_file)
	if err != nil {
		return nil, err
	}
	proxy := &RedisProxy{
		config_file: config_file,
		config:      config,
		controller:  NewProxyController()}
	return proxy, nil
}

func (proxy *RedisProxy) Run() error {
	listener, err := net.Listen("tcp", proxy.config.ListenOn)
	if err != nil {
		return err
	}

	log.Println("Listening on", proxy.config.ListenOn)

	go proxy.watchSignals()
	go proxy.controller.run(proxy) // TODO: clean this up when getting rid of circular dep
	go proxy.publishAdminInterface()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go proxy.handleClient(conn)
	}
}

func (proxy *RedisProxy) ReloadConfig() {
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

	defer cliConn.Close()
	cliReader := resp.NewReader(bufio.NewReader(cliConn))
	cliWriter := bufio.NewWriter(cliConn)

	uplinkAddr := ""

	var uplinkConn net.Conn
	var uplinkReader *resp.RESPReader
	var uplinkWriter *bufio.Writer

	defer func() {
		if uplinkConn != nil {
			uplinkConn.Close()
		}
	}()

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
				log.Println("Dialing", uplinkAddr)
				if uplinkConn != nil {
					uplinkConn.Close()
				}
				uplinkConn, err = net.Dial("tcp", uplinkAddr)
				if err != nil {
					return nil, err
				}
				uplinkReader = resp.NewReader(bufio.NewReader(uplinkConn))
				uplinkWriter = bufio.NewWriter(uplinkConn)
			}

			uplinkWriter.Write(req)
			uplinkWriter.Flush()

			uplinkConn.SetReadDeadline(time.Now().Add(time.Duration(proxy.config.ReadTimeLimitMs) * time.Millisecond))
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
	if !proxy.config.LogMessages {
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
