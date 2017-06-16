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

	confHolder     ConfigHolder
	activeRequests int
	state          ControllerState
}

func NewProxyControllerProc(confHolder ConfigHolder) *ProxyControllerProc {
	return &ProxyControllerProc{
		channels: ProxyControllerChannels{
			requestPermission: make(chan chan struct{}, MaxConnections),
			releasePermission: make(chan struct{}, MaxConnections),
			info:              make(chan chan *ControllerInfo),
			command:           make(chan ControllerCommand),
		},
		confHolder: confHolder,
	}
}

func (p *ProxyControllerProc) run() {
	p.state = ProxyRunning

	channelMap := map[ControllerState]*ProxyControllerChannels{
		ProxyRunning: &p.channels,
		ProxyPausing: &ProxyControllerChannels{
			requestPermission: nil,
			releasePermission: p.channels.releasePermission,
			info:              p.channels.info,
			command:           nil},
		ProxyReloading: &ProxyControllerChannels{
			requestPermission: nil,
			releasePermission: p.channels.releasePermission,
			info:              p.channels.info,
			command:           nil},
		ProxyPaused: &ProxyControllerChannels{
			requestPermission: nil,
			releasePermission: nil,
			info:              p.channels.info,
			command:           p.channels.command},
	}

	for p.state != ProxyStopping {
		switch p.state {
		case ProxyRunning:
			// nothing
		case ProxyPausing:
			if p.activeRequests == 0 {
				p.state = ProxyPaused
				continue
			}
		case ProxyReloading:
			if p.activeRequests == 0 {
				p.confHolder.ReloadConfig()
				p.state = ProxyRunning
				continue
			}
		case ProxyPaused:
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
		case CmdPause:
			p.state = ProxyPausing
		case CmdUnpause:
			p.state = ProxyRunning
		case CmdReload:
			p.state = ProxyReloading
		case CmdStop:
			p.state = ProxyStopping
		default:
			log.Print("Unknown controller command:", cmd)
		}
	}
}
