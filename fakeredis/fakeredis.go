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
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Codility/redis-proxy/resp"
)

type FakeRedisServer struct {
	name string

	listener net.Listener

	mu       sync.Mutex
	shutdown bool
	requests []*resp.Msg
}

func New(name string) *FakeRedisServer {
	return &FakeRedisServer{name: name}
}

func Start(name, network string) *FakeRedisServer {
	startedChan := make(chan struct{})

	srv := New(name)

	go srv.Run(startedChan, network)
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

func (s *FakeRedisServer) Run(startedChan chan struct{}, network string) {
	var err error

	switch network {
	case "tcp":
		s.listener, err = net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})

	case "unix":
		dir, err := ioutil.TempDir("/tmp", "fakeredis")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(dir)

		name := dir + "/fakeredis.sock"
		s.listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: name, Net: "unix"})

	default:
		panic("Unknown network: " + network)
	}
	if err != nil {
		panic(err)
	}
	defer s.listener.Close()

	if startedChan != nil {
		startedChan <- struct{}{}
	}

	for !s.IsShuttingDown() {
		switch network {
		case "tcp":
			s.listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
		case "unix":
			s.listener.(*net.UnixListener).SetDeadline(time.Now().Add(time.Second))
		}

		conn, err := s.listener.Accept()
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

func (s *FakeRedisServer) handleConnection(conn net.Conn) {
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
