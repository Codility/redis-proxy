package fakeredis

//////////////////////////////////////////////////////////////////////
// Minimal Redis-like server that exposes RESP via TCP.

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"gitlab.codility.net/marcink/redis-proxy/resp"
)

type FakeRedisServer struct {
	name string

	listener *net.TCPListener

	mu       sync.Mutex
	shutdown bool
	reqCnt   int
}

func New(name string) *FakeRedisServer {
	return &FakeRedisServer{name: name}
}

func Start(name string) *FakeRedisServer {
	startedChan := make(chan struct{})

	srv := New(name)
	go srv.Run(startedChan)

	<-startedChan

	return srv
}

func (s *FakeRedisServer) IsShuttingDown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shutdown
}

func (s *FakeRedisServer) Run(startedChan chan struct{}) {
	var err error

	s.listener, err = net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		panic(err)
	}
	defer s.listener.Close()

	if startedChan != nil {
		startedChan <- struct{}{}
	}

	for !s.IsShuttingDown() {
		s.listener.SetDeadline(time.Now().Add(time.Second))
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			if resp.IsNetTimeout(err) {
				continue
			}
			panic(err)
		}
		go s.handleConnection(conn)
	}
}

func (s *FakeRedisServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.shutdown = true
	// Don't bother waiting for the server to actually close, it's
	// just in tests anyway.
}

func (s *FakeRedisServer) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *FakeRedisServer) handleConnection(conn *net.TCPConn) {
	rc := resp.NewConn(conn, 100, false)
	for !s.IsShuttingDown() {
		_, err := rc.ReadMsg()
		if err != nil {
			if resp.IsNetTimeout(err) || (err == io.EOF) {
				continue
			}
			panic(err)
		}
		s.BumpReqCnt()

		res := fmt.Sprintf("$%d\r\n%s\r\n", len(s.name), s.name)
		rc.MustWrite([]byte(res))
	}
}

func (s *FakeRedisServer) ReqCnt() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.reqCnt
}

func (s *FakeRedisServer) BumpReqCnt() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reqCnt++
}
