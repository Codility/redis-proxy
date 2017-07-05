package main

import (
	"flag"

	"github.com/codility/redis-proxy/rproxy"
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
	panic(proxy.Run())
}
