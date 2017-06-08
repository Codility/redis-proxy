package rproxy

import "log"

type ProxyControllerProc struct {
	requestPermission chan chan struct{}
	releasePermission chan struct{}
	info              chan chan *ControllerInfo
	command           chan ControllerCommand
}

func NewProxyControllerProc() *ProxyControllerProc {
	return &ProxyControllerProc{
		requestPermission: make(chan chan struct{}, MAX_CONNECTIONS),
		releasePermission: make(chan struct{}, MAX_CONNECTIONS),
		info:              make(chan chan *ControllerInfo),
		command:           make(chan ControllerCommand),
	}
}

func (p *ProxyControllerProc) run(confHolder ProxyConfigHolder) {
	activeRequests := 0
	state := PROXY_RUNNING
	requestPermissionChannel := p.requestPermission

	for {
		requestPermissionChannel = nil
		switch state {
		case PROXY_RUNNING:
			requestPermissionChannel = p.requestPermission
		case PROXY_PAUSING:
			if activeRequests == 0 {
				state = PROXY_PAUSED
				continue
			}
		case PROXY_RELOADING:
			if activeRequests == 0 {
				confHolder.ReloadConfig()
				state = PROXY_RUNNING
				continue
			}
		case PROXY_PAUSED:
			// nothing
		}
		select {
		// In states other than PROXY_RUNNING
		// requestPermissionChannel is nil, so the controller
		// will not receive any requests for permission.
		case permCh := <-requestPermissionChannel:
			permCh <- struct{}{}
			activeRequests++
		case <-p.releasePermission:
			activeRequests--

		case stateCh := <-p.info:
			stateCh <- &ControllerInfo{
				ActiveRequests:  activeRequests,
				WaitingRequests: len(p.requestPermission),
				State:           state,
				Config:          confHolder.GetConfig()}

		case cmd := <-p.command:
			switch cmd {
			case CMD_PAUSE:
				state = PROXY_PAUSING
			case CMD_UNPAUSE:
				state = PROXY_RUNNING
			case CMD_RELOAD:
				state = PROXY_RELOADING
			case CMD_STOP:
				return
			default:
				log.Print("Unknown controller command:", cmd)
			}
		}
	}
}
