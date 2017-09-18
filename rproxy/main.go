package rproxy

import (
	"fmt"
	"log"
)

func (proxy *Proxy) Run() {
	proxy.SetState(ProxyStarting)

	if err := proxy.startListening(); err != nil {
		log.Println("Could not start listening: ", err)
		proxy.SetState(ProxyStopped)
		return
	}
	log.Println("Listening on", proxy.ListenAddr())

	if proxy.config.ListenUnmanaged.Addr != "" {
		proxy.rawProxy = NewRawProxy(proxy)
		proxy.rawProxy.Start()
	}

	proxy.adminUI = NewAdminUI(proxy)
	proxy.adminUI.Start()

	proxy.SetState(ProxyRunning)

	channelMap := map[ProxyState]*ProxyChannels{
		ProxyRunning: &proxy.channels,
		ProxyPausing: &ProxyChannels{
			requestPermission: nil,
			releasePermission: proxy.channels.releasePermission,
			info:              proxy.channels.info,
			command:           proxy.channels.command},
		ProxyPaused: &ProxyChannels{
			requestPermission: nil,
			releasePermission: nil,
			info:              proxy.channels.info,
			command:           proxy.channels.command},
	}

	for {
		statRecordProxyState(proxy.activeRequests)
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
		case ProxyRunning:
		case ProxyPaused:
		}
		proxy.handleChannels(channelMap[st])
	}

	proxy.adminUI.Stop()
	proxy.adminUI = nil

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

	case cmdPack := <-channels.command:
		switch cmdPack.cmd {
		case CmdPause:
			proxy.SetState(ProxyPausing)
			cmdPack.Return(nil)
		case CmdUnpause:
			proxy.SetState(ProxyRunning)
			cmdPack.Return(nil)
		case CmdReload:
			cmdPack.Return(proxy.ReloadConfig())
		case CmdStop:
			proxy.SetState(ProxyStopping)
			cmdPack.Return(nil)
		default:
			err := fmt.Errorf("Unknown proxy command: %v", cmdPack.cmd)
			log.Print(err)
			cmdPack.Return(err)
		}
	}
}
