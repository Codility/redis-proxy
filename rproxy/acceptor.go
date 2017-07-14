package rproxy

import (
	"log"
	"net"
	"time"

	"github.com/Codility/redis-proxy/resp"
)

func (proxy *Proxy) startConnAcceptor() error {
	ln, tcpLn, addr, err := proxy.config.Listen.Listen()
	if err != nil {
		return err
	}
	proxy.listenAddr = addr
	go proxy.runConnAcceptor(ln, tcpLn)
	return nil
}

func (proxy *Proxy) runConnAcceptor(ln net.Listener, tcpLn *net.TCPListener) {
	defer ln.Close()

	for proxy.Alive() {
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
