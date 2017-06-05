all: redis-proxy switch-test

########################################
# redis-proxy and related targets

redis-proxy: rproxy/*.go cmd/redis-proxy/*.go
	./scripts/go build -o "$@" gitlab.codility.net/marcink/redis-proxy/cmd/redis-proxy

#.PHONY: test
#test:
#	./scripts/go test

.PHONY: config
config: config.json

config.json:
	echo "Create config.json based on config_example.json"
	exit 1

.PHONY: run
run: redis-proxy config.json
	./redis-proxy config.json

########################################
# switch-test and related targets

switch-test: redis-proxy cmd/switch-test/*.go
	./scripts/go build -o "$@" gitlab.codility.net/marcink/redis-proxy/cmd/switch-test

run-switch-test: switch-test redis-proxy
	./switch-test
