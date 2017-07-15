package rproxy

import "log"

type ProxyChannels struct {
	requestPermission chan chan struct{}
	releasePermission chan struct{}
	info              chan chan *ProxyInfo
	command           chan ProxyCommand
}

type ProxyProc struct {
	channels ProxyChannels

	confHolder     ConfigHolder
	activeRequests int
	state          ProxyState
}

func NewProxyProc(confHolder ConfigHolder) *ProxyProc {
	return &ProxyProc{
		channels: ProxyChannels{
			requestPermission: make(chan chan struct{}, MaxConnections),
			releasePermission: make(chan struct{}, MaxConnections),
			info:              make(chan chan *ProxyInfo),
			command:           make(chan ProxyCommand),
		},
		confHolder: confHolder,
	}
}

func (p *ProxyProc) run() {
	p.state = ProxyRunning

	channelMap := map[ProxyState]*ProxyChannels{
		ProxyRunning: &p.channels,
		ProxyPausing: &ProxyChannels{
			requestPermission: nil,
			releasePermission: p.channels.releasePermission,
			info:              p.channels.info,
			command:           nil},
		ProxyReloading: &ProxyChannels{
			requestPermission: nil,
			releasePermission: p.channels.releasePermission,
			info:              p.channels.info,
			command:           nil},
		ProxyPaused: &ProxyChannels{
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

func (p *ProxyProc) handleChannels(channels *ProxyChannels) {
	select {
	case permCh := <-channels.requestPermission:
		permCh <- struct{}{}
		p.activeRequests++
	case <-channels.releasePermission:
		p.activeRequests--

	case stateCh := <-channels.info:
		stateCh <- &ProxyInfo{
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
			log.Print("Unknown proxy command:", cmd)
		}
	}
}
