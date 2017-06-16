package rproxy

import (
	"crypto/tls"
	"log"
	"net"
)

// Returns:
// - top-level generic net.Listener
// - the underlying net.TCPListener (different from the first listener in case of TLS)
// - effective address
// - error, if any
func getListener(addrSpec AddrSpec) (net.Listener, *net.TCPListener, *net.Addr, error) {
	ln, err := net.Listen("tcp", addrSpec.Addr)
	if err != nil {
		log.Fatalf("Could not listen: %s", err)
		return nil, nil, nil, err
	}
	addr := ln.(*net.TCPListener).Addr()

	tlsSpec := addrSpec.TLS
	if tlsSpec == nil {
		return ln, ln.(*net.TCPListener), &addr, nil
	}

	cer, err := tls.LoadX509KeyPair(tlsSpec.CertFile, tlsSpec.KeyFile)
	if err != nil {
		log.Fatalf("Could not load key pair (%s, %s): %s",
			tlsSpec.CertFile, tlsSpec.KeyFile, err)
		return nil, nil, nil, err
	}
	tlsLn := tls.NewListener(ln, &tls.Config{
		Certificates: []tls.Certificate{cer},
	})
	return tlsLn, ln.(*net.TCPListener), &addr, nil
}
