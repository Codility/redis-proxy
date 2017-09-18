package rproxy

import (
	"io"
	"log"
	"net"
)

type RawHandler struct {
	cliConn net.Conn
	proxy   *Proxy
}

func NewRawHandler(conn net.Conn, proxy *Proxy) *RawHandler {
	return &RawHandler{cliConn: conn, proxy: proxy}
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
	defer r.cliConn.Close()
	uplinkConn := r.DialUplink()
	defer uplinkConn.Close()

	log.Printf("Starting raw proxy for %s <-> %s", r.cliConn.RemoteAddr(), uplinkConn.RemoteAddr())
	doneChan := make(chan struct{})

	// incoming
	go func() {
		io.Copy(r.cliConn, uplinkConn)
		doneChan <- struct{}{}
	}()

	// outgoing
	go func() {
		io.Copy(uplinkConn, r.cliConn)
		doneChan <- struct{}{}
	}()

	// Wait until one side gets closed.  Deferred calls above will
	// close both connections, and that will terminate the other
	// goroutine.
	<-doneChan
	log.Printf("Closing raw proxy for %s <-> %s", r.cliConn.RemoteAddr(), uplinkConn.RemoteAddr())
}
