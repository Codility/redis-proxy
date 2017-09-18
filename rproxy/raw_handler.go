package rproxy

import (
	"io"
	"log"
	"net"
)

type RawHandler struct {
	proxy               *Proxy
	cliConn, uplinkConn net.Conn

	terminateChan chan struct{}
}

func NewRawHandler(conn net.Conn, proxy *Proxy) *RawHandler {
	return &RawHandler{
		cliConn:       conn,
		proxy:         proxy,
		terminateChan: make(chan struct{}),
	}
}

func (r *RawHandler) DialUplink() net.Conn {
	uplinkConn, err := r.proxy.GetConfig().Uplink.Dial()
	if err != nil {
		log.Printf("Error: %v\n", err)
		return nil
	}
	return uplinkConn
}

func (r *RawHandler) Run() {
	defer func() {
		r.cliConn.Close()
		r.uplinkConn.Close()
	}()

	r.uplinkConn = r.DialUplink()
	doneChan := make(chan struct{})
	terminating := false

	pump := func(from, to net.Conn) {
		_, err := io.Copy(from, to)
		if !terminating && err != nil {
			log.Print("Raw proxy error:", err)
		}
		doneChan <- struct{}{}
	}

	log.Printf("Starting raw proxy for %s <-> %s", r.cliConn.RemoteAddr(), r.uplinkConn.RemoteAddr())

	go pump(r.cliConn, r.uplinkConn)
	go pump(r.uplinkConn, r.cliConn)

	// Both clauses in select should have the same result: finish this goroutine
	select {
	case <-doneChan:
	case <-r.terminateChan:
	}
	terminating = true

	log.Printf("Closing raw proxy for %s <-> %s", r.cliConn.RemoteAddr(), r.uplinkConn.RemoteAddr())
}

func (r *RawHandler) Terminate() {
	r.terminateChan <- struct{}{}
}
