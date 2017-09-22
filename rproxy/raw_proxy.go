package rproxy

import (
	"log"
	"net"
	"time"

	"github.com/Codility/redis-proxy/resp"
)

type RawProxy struct {
	Addr             net.Addr
	proxy            *Proxy
	terminateAllChan chan chan struct{}
	getInfoChan      chan chan *RawProxyInfo
	deadHandlerChan  chan *RawHandler
}

type RawProxyInfo struct {
	HandlerCnt int
}

type RawProxyCmd int

const (
	RawCmdTerminateAll = RawProxyCmd(iota)
)

func NewRawProxy(proxy *Proxy) *RawProxy {
	return &RawProxy{
		proxy:            proxy,
		terminateAllChan: make(chan chan struct{}),
		getInfoChan:      make(chan chan *RawProxyInfo),
		deadHandlerChan:  make(chan *RawHandler),
	}
}

func (r *RawProxy) Start() error {
	config := r.proxy.GetConfig()

	ln, err := config.ListenRaw.Listen()
	if err != nil {
		return err
	}
	r.Addr = ln.Addr()
	log.Println("Raw proxy:", r.Addr)
	go r.proxyLoop(r.startAcceptor(ln))
	return nil
}

func (r *RawProxy) startAcceptor(ln *Listener) chan net.Conn {
	connections := make(chan net.Conn)

	go func() {
		defer close(connections)
		defer ln.Close()

		for r.proxy.State().IsStartingOrAlive() {
			ln.SetDeadline(time.Now().Add(time.Second))
			conn, err := ln.Accept()
			if err != nil {
				if resp.IsNetTimeout(err) {
					continue
				}
				log.Printf("Raw Proxy: Got an error accepting a connection: %s", err)
			}
			connections <- conn
		}
	}()

	return connections
}

func (r *RawProxy) proxyLoop(connections chan net.Conn) {
	handlers := map[net.Addr]*RawHandler{}

loop:
	for r.proxy.State().IsStartingOrAlive() {
		select {
		case conn := <-connections:
			if conn == nil {
				continue loop
			}
			h := NewRawHandler(conn, r.proxy)
			handlers[h.CliAddr()] = h
			go h.Run()
		case ret := <-r.terminateAllChan:
			for _, h := range handlers {
				h.Terminate()
			}
			ret <- struct{}{}
		case dead := <-r.deadHandlerChan:
			delete(handlers, dead.CliAddr())
		case ret := <-r.getInfoChan:
			ret <- &RawProxyInfo{HandlerCnt: len(handlers)}
		}
	}
}

func (r *RawProxy) TerminateAll() {
	ret := make(chan struct{})
	r.terminateAllChan <- ret
	<-ret
}

func (r *RawProxy) GetInfo() *RawProxyInfo {
	ret := make(chan *RawProxyInfo)
	r.getInfoChan <- ret
	return <-ret
}
