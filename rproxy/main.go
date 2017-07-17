package rproxy

import "log"

func (proxy *Proxy) Run() {
	proxy.SetState(ProxyStarting)

	if err := proxy.startListening(); err != nil {
		log.Println("Could not start listening: ", err)
		proxy.SetState(ProxyStopped)
		return
	}
	log.Println("Listening on", proxy.ListenAddr())
	proxy.publishAdminInterface()
	proxy.SetState(ProxyRunning)

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

	for {
		st := proxy.State()
		if st == ProxyStopping {
			break
		}
		switch st {
		case ProxyPausing:
			if proxy.activeRequests == 0 {
				proxy.SetState(ProxyPaused)
				continue
			}
		case ProxyReloading:
			if proxy.activeRequests == 0 {
				proxy.ReloadConfig()
				proxy.SetState(ProxyRunning)
				continue
			}
		case ProxyRunning:
		case ProxyPaused:
		}
		proxy.handleChannels(channelMap[st])
	}

	proxy.SetState(ProxyStopped)
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
			State:           proxy.State(),
			Config:          proxy.GetConfig()}

	case cmd := <-channels.command:
		switch cmd {
		case CmdPause:
			proxy.SetState(ProxyPausing)
		case CmdUnpause:
			proxy.SetState(ProxyRunning)
		case CmdReload:
			proxy.SetState(ProxyReloading)
		case CmdStop:
			proxy.SetState(ProxyStopping)
		default:
			log.Print("Unknown proxy command:", cmd)
		}
	}
}
