package fakeredis

//////////////////////////////////////////////////////////////////////
// Minimal Redis-like server that exposes RESP via TCP.
//
// It responds with:
//  - "+OK\r\n" to "SELECT n" and "AUTH x"
//  - its name (as passed to New()) to all other requests

import (
	"fmt"
	"io"
	"log"
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
	requests []*resp.Msg
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

func (s *FakeRedisServer) Requests() []*resp.Msg {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.requests
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
	defer rc.Close()

	for !s.IsShuttingDown() {
		req, err := rc.ReadMsg()
		if err != nil {
			if resp.IsNetTimeout(err) {
				continue
			}
			if err != io.EOF {
				log.Print(err)
			}
			return
		}
		s.RecordRequest(req)

		if (req.Op() == resp.MsgOpAuth) || (req.Op() == resp.MsgOpSelect) {
			rc.MustWrite([]byte("+OK\r\n"))
		} else {
			res := fmt.Sprintf("$%d\r\n%s\r\n", len(s.name), s.name)
			rc.MustWrite([]byte(res))
		}
	}
}

func (s *FakeRedisServer) ReqCnt() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.requests)
}

func (s *FakeRedisServer) RecordRequest(req *resp.Msg) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = append(s.requests, req)
}

func (s *FakeRedisServer) LastRequest() *resp.Msg {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.requests[len(s.requests)-1]
}
