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
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	for {
		s := <-c
		log.Printf("Got signal: %v, reloading config\n", s)
		proxy.Reload()
	}
}
