all: redis-proxy

redis-proxy: rproxy/*.go cmd/redis-proxy/*.go
	./scripts/go build -o "$@" gitlab.codility.net/marcink/redis-proxy/cmd/redis-proxy

.PHONY: test
test:
	./scripts/go test

.PHONY: config
config: config.json

config.json:
	echo "Create config.json based on config_example.json"
	exit 1

.PHONY: run
run: redis-proxy config.json
	./redis-proxy config.json
