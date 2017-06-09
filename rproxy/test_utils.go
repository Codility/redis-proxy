package rproxy

import (
	"net"
	"sync"
	"time"
)

////////////////////////////////////////
// TestConfigHolder

type TestConfigHolder struct {
	config              *ProxyConfig
	GetConfigCallCnt    int
	ReloadConfigCallCnt int
}

func (ch *TestConfigHolder) GetConfig() *ProxyConfig {
	ch.GetConfigCallCnt += 1
	return ch.config
}

func (ch *TestConfigHolder) ReloadConfig() {
	ch.ReloadConfigCallCnt += 1
}

////////////////////////////////////////
// TestRequest

type TestRequest struct {
	contr *ProxyController
	done  bool
	block func()
}

func NewTestRequest(contr *ProxyController, block func()) *TestRequest {
	return &TestRequest{contr: contr, block: block}
}

func (r *TestRequest) Do() {
	r.contr.CallUplink(func() (*RespMsg, error) {
		r.block()
		return nil, nil
	})
	r.done = true
}

////////////////////////////////////////
// FakeRedisServer

type FakeRedisServer struct {
	listener *net.TCPListener

	mu       sync.Mutex
	shutdown bool
}

func NewFakeRedisServer() *FakeRedisServer {
	return &FakeRedisServer{}
}

func StartFakeRedisServer() *FakeRedisServer {
	startedChan := make(chan struct{})

	srv := NewFakeRedisServer()
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
			if IsTimeout(err) {
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
	rc := NewRespConn(conn, 100, false)
	for !s.IsShuttingDown() {
		_, err := rc.ReadMsg()
		if err != nil {
			if IsTimeout(err) {
				continue
			}
			panic(err)
		}
		_, err = rc.WriteMsg(&RespMsg{[]byte("$4\r\nfake\r\n")})
		if err != nil {
			panic(err)
		}
	}
}

func (s *FakeRedisServer) ReqCnt() int {
	// TODO
	return 0
}
