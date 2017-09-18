package rproxy

import (
	"log"
	"net"
	"time"

	"github.com/Codility/redis-proxy/resp"
)

type RawProxy struct {
	Addr  net.Addr
	proxy *Proxy
}

func NewRawProxy(proxy *Proxy) *RawProxy {
	return &RawProxy{proxy: proxy}
}

func (r *RawProxy) Start() error {
	config := r.proxy.GetConfig()

	ln, err := config.ListenUnmanaged.Listen()
	if err != nil {
		return err
	}
	r.Addr = ln.Addr()
	log.Println("Raw proxy started on", r.Addr)
	go r.listenForClients(ln)
	return nil
}

func (r *RawProxy) listenForClients(ln *Listener) {
	defer ln.Close()

	for r.proxy.State().IsStartingOrAlive() {
		ln.SetDeadline(time.Now().Add(time.Second))
		conn, err := ln.Accept()
		if err != nil {
			if resp.IsNetTimeout(err) {
				continue
			}
			log.Printf("Unmanaged Proxy: Got an error accepting a connection: %s", err)
		} else {
			go NewRawHandler(conn, r.proxy).Run()
		}
	}
}
