package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Codility/redis-proxy/rproxy"
)

var (
	config_file = flag.String("f", "config.json", "Config file")
)

func main() {
	flag.Parse()
	proxy, err := rproxy.NewProxy(rproxy.NewFileConfigLoader(*config_file))
	if err != nil {
		panic(err)
	}
	go watchSignals(proxy)
	proxy.Run()
}

func watchSignals(proxy *rproxy.Proxy) {
	reload := make(chan os.Signal, 1)
	signal.Notify(reload, syscall.SIGHUP)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT)
	signal.Notify(stop, syscall.SIGTERM)

	for {
		select {
		case s := <-reload:
			log.Printf("Got signal: %v, reloading config\n", s)
			proxy.Reload()
		case s := <-stop:
			log.Printf("Got signal: %v, stopping\n", s)
			proxy.Stop()
		}
	}
}
