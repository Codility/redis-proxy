package rproxy

import (
	"net"
	"time"
)

// Convenience Listener
//
// The reason for explicitely storing a reference to net.TCPListener
// is that the proxy needs it to set deadlines on accept operations,
// but the listener from tls package does not support them, and does
// not provide any way to get to the underlying TCPListener.

type Listener struct {
	net.Listener
	TCPListener *net.TCPListener
}

func (l *Listener) SetDeadline(t time.Time) error {
	return l.TCPListener.SetDeadline(t)
}
