all: redis-proxy

redis-proxy: *.go
	./scripts/go build -o "$@"

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
