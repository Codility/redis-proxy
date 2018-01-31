package rproxy

import (
	"net"
	"time"
)

// Convenience Listener
//
// The reason for explicitely storing a reference to original
// net.TCPListener and net.UnixListener is that the proxy needs it to
// set deadlines on accept operations, but the listener from tls
// package does not support them, and does not provide any way to get
// to the underlying Net/UnixListener.

type Listener struct {
	net.Listener
	originalListener AddrDeadliner
}

type AddrDeadliner interface {
	SetDeadline(t time.Time) error
	Addr() net.Addr
}

func (l *Listener) SetDeadline(deadline time.Time) error {
	return l.originalListener.SetDeadline(deadline)
}

func (l *Listener) Addr() net.Addr {
	return l.originalListener.Addr()
}
