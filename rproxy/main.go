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

	proxy.adminUI = NewAdminUI(proxy)
	proxy.adminUI.Start()

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
			proxy.SetState(ProxyReloading)
			cmdPack.Return(nil)
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
