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
	config_file = flag.String("f", "config.json", "Config file (or \"-\" for standard input)")
)

func main() {
	flag.Parse()
	var configLoader rproxy.ConfigLoader
	if *config_file == "-" {
		configLoader = rproxy.NewInputConfigLoader(os.Stdin)
	} else {
		configLoader = rproxy.NewFileConfigLoader(*config_file)
	}
	proxy, err := rproxy.NewProxy(configLoader)
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
