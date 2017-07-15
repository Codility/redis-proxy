package rproxy

import "log"

func (proxy *Proxy) Run() error {
	proxy.state = ProxyStarting
	proxy.publishAdminInterface()

	if err := proxy.startListening(); err != nil {
		return err
	}
	log.Println("Listening on", proxy.ListenAddr())
	go proxy.watchSignals()

	proxy.state = ProxyRunning

	channelMap := map[ProxyState]*ProxyChannels{
		ProxyRunning: &proxy.channels,
		ProxyPausing: &ProxyChannels{
			requestPermission: nil,
			releasePermission: proxy.channels.releasePermission,
			info:              proxy.channels.info,
			command:           nil},
		ProxyReloading: &ProxyChannels{
			requestPermission: nil,
			releasePermission: proxy.channels.releasePermission,
			info:              proxy.channels.info,
			command:           nil},
		ProxyPaused: &ProxyChannels{
			requestPermission: nil,
			releasePermission: nil,
			info:              proxy.channels.info,
			command:           proxy.channels.command},
	}

	for proxy.state != ProxyStopping {
		switch proxy.state {
		case ProxyRunning:
			// nothing
		case ProxyPausing:
			if proxy.activeRequests == 0 {
				proxy.state = ProxyPaused
				continue
			}
		case ProxyReloading:
			if proxy.activeRequests == 0 {
				proxy.ReloadConfig()
				proxy.state = ProxyRunning
				continue
			}
		case ProxyPaused:
			// nothing
		}
		proxy.handleChannels(channelMap[proxy.state])
	}

	proxy.state = ProxyStopped

	return nil
}

func (proxy *Proxy) handleChannels(channels *ProxyChannels) {
	select {
	case permCh := <-channels.requestPermission:
		permCh <- struct{}{}
		proxy.activeRequests++
	case <-channels.releasePermission:
		proxy.activeRequests--

	case stateCh := <-channels.info:
		stateCh <- &ProxyInfo{
			ActiveRequests:  proxy.activeRequests,
			WaitingRequests: len(proxy.channels.requestPermission),
			State:           proxy.state,
			Config:          proxy.GetConfig()}

	case cmd := <-channels.command:
		switch cmd {
		case CmdPause:
			proxy.state = ProxyPausing
		case CmdUnpause:
			proxy.state = ProxyRunning
		case CmdReload:
			proxy.state = ProxyReloading
		case CmdStop:
			proxy.state = ProxyStopping
		default:
			log.Print("Unknown proxy command:", cmd)
		}
	}
}
