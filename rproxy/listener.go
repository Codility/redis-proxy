package rproxy

import (
	"fmt"
	"net"
	"time"
)

// Convenience Listener
//
// The reason for explicitely storing a reference to net.TCPListener and
// net.UnixListener is that the proxy needs it to set deadlines on accept
// operations, but the listener from tls package does not support them, and
// does not provide any way to get to the underlying Net/UnixListener.

type Listener struct {
	net.Listener
	OriginalListener net.Listener
}

func (l *Listener) SetDeadline(deadline time.Time) error {
	switch t := l.OriginalListener.(type) {
	case *net.TCPListener:
		return l.OriginalListener.(*net.TCPListener).SetDeadline(deadline)
	case *net.UnixListener:
		return l.OriginalListener.(*net.UnixListener).SetDeadline(deadline)
	default:
		return fmt.Errorf("Listener: unknown underlying type: %s", t)
	}
}

func (l *Listener) Addr() net.Addr {
	switch t := l.OriginalListener.(type) {
	case *net.TCPListener:
		return l.OriginalListener.(*net.TCPListener).Addr()
	case *net.UnixListener:
		return l.OriginalListener.(*net.UnixListener).Addr()
	default:
		panic(fmt.Sprintf("Listener: unknown underlying type: %s", t))
	}
}
