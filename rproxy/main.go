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
	log.Println("Managed proxy:", proxy.ListenAddr())

	if proxy.config.ListenRaw.Addr != "" {
		proxy.rawProxy = NewRawProxy(proxy)
		proxy.rawProxy.Start()
	}

	if proxy.config.Admin.Addr != "" {
		proxy.adminUI = NewAdminUI(proxy)
		proxy.adminUI.Start()
	}

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

	if proxy.adminUI != nil {
		proxy.adminUI.Stop()
		proxy.adminUI = nil
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
		rawConns := 0
		if proxy.rawProxy != nil {
			rawConns = proxy.rawProxy.GetInfo().HandlerCnt
		}
		stateCh <- &ProxyInfo{
			ActiveRequests:  proxy.activeRequests,
			WaitingRequests: len(proxy.channels.requestPermission),
			State:           proxy.State(),
			StateStr:        proxy.State().String(),
			Config:          proxy.GetConfig(),
			RawConnections:  rawConns,
		}

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
		case CmdTerminateRawConnections:
			proxy.rawProxy.TerminateAll()
			cmdPack.Return(nil)
		default:
			err := fmt.Errorf("Unknown proxy command: %v", cmdPack.cmd)
			log.Print(err)
			cmdPack.Return(err)
		}
	}
}
