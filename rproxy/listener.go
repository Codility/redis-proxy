package rproxy

import (
	"log"
	"net"
	"time"

	"github.com/Codility/redis-proxy/resp"
)

func (proxy *Proxy) startListening() error {
	ln, tcpLn, addr, err := proxy.config.Listen.Listen()
	if err != nil {
		return err
	}
	proxy.listenAddr = addr
	go proxy.listenForClients(ln, tcpLn)
	return nil
}

func (proxy *Proxy) listenForClients(ln net.Listener, tcpLn *net.TCPListener) {
	defer ln.Close()

	for proxy.State().IsStartingOrAlive() {
		tcpLn.SetDeadline(time.Now().Add(time.Second))
		conn, err := ln.Accept()
		if err != nil {
			if resp.IsNetTimeout(err) {
				continue
			}
			log.Printf("Got an error accepting a connection: %s", err)
		} else {
			rc := resp.NewConn(conn, 0, proxy.config.LogMessages)
			go NewCliHandler(rc, proxy).Run()
		}
	}
}
