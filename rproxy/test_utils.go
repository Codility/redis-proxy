package rproxy

import (
	"net"
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
	quit     chan struct{}
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

	for {
		select {
		case <-s.quit:
			return
		default:
		}

		s.listener.SetDeadline(time.Now().Add(time.Second))
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			if err, ok := err.(*net.OpError); ok && err.Timeout() {
				// it was a timeout; continue the loop
			} else {
				panic(err)
			}
		} else {
			go s.handleConnection(conn)
		}
	}
}

func (s *FakeRedisServer) Stop() {
	s.quit <- struct{}{}
}

func (s *FakeRedisServer) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *FakeRedisServer) handleConnection(conn *net.TCPConn) {
	// TODO
}

func (s *FakeRedisServer) ReqCnt() int {
	// TODO
	return 0
}
