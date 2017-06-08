package rproxy

import "log"

type ProxyControllerChannels struct {
	requestPermission chan chan struct{}
	releasePermission chan struct{}
	info              chan chan *ControllerInfo
	command           chan ControllerCommand
}

type ProxyControllerProc struct {
	channels ProxyControllerChannels

	confHolder     ProxyConfigHolder
	activeRequests int
	state          ControllerState
}

func NewProxyControllerProc(confHolder ProxyConfigHolder) *ProxyControllerProc {
	return &ProxyControllerProc{
		channels: ProxyControllerChannels{
			requestPermission: make(chan chan struct{}, MAX_CONNECTIONS),
			releasePermission: make(chan struct{}, MAX_CONNECTIONS),
			info:              make(chan chan *ControllerInfo),
			command:           make(chan ControllerCommand),
		},
		confHolder: confHolder,
	}
}

func (p *ProxyControllerProc) run() {
	p.state = PROXY_RUNNING

	channelMap := map[ControllerState]*ProxyControllerChannels{
		PROXY_RUNNING: &p.channels,
		PROXY_PAUSING: &ProxyControllerChannels{
			requestPermission: nil,
			releasePermission: p.channels.releasePermission,
			info:              p.channels.info,
			command:           nil},
		PROXY_RELOADING: &ProxyControllerChannels{
			requestPermission: nil,
			releasePermission: p.channels.releasePermission,
			info:              p.channels.info,
			command:           nil},
		PROXY_PAUSED: &ProxyControllerChannels{
			requestPermission: nil,
			releasePermission: nil,
			info:              p.channels.info,
			command:           p.channels.command},
	}

	for p.state != PROXY_STOPPING {
		switch p.state {
		case PROXY_RUNNING:
			// nothing
		case PROXY_PAUSING:
			if p.activeRequests == 0 {
				p.state = PROXY_PAUSED
				continue
			}
		case PROXY_RELOADING:
			if p.activeRequests == 0 {
				p.confHolder.ReloadConfig()
				p.state = PROXY_RUNNING
				continue
			}
		case PROXY_PAUSED:
			// nothing
		}
		p.handleChannels(channelMap[p.state])
	}
}

func (p *ProxyControllerProc) handleChannels(channels *ProxyControllerChannels) {
	select {
	case permCh := <-channels.requestPermission:
		permCh <- struct{}{}
		p.activeRequests++
	case <-channels.releasePermission:
		p.activeRequests--

	case stateCh := <-channels.info:
		stateCh <- &ControllerInfo{
			ActiveRequests:  p.activeRequests,
			WaitingRequests: len(p.channels.requestPermission),
			State:           p.state,
			Config:          p.confHolder.GetConfig()}

	case cmd := <-channels.command:
		switch cmd {
		case CMD_PAUSE:
			p.state = PROXY_PAUSING
		case CMD_UNPAUSE:
			p.state = PROXY_RUNNING
		case CMD_RELOAD:
			p.state = PROXY_RELOADING
		case CMD_STOP:
			p.state = PROXY_STOPPING
		default:
			log.Print("Unknown controller command:", cmd)
		}
	}
}
