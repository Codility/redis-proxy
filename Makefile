BINARIES=redis-proxy switch-test

all: $(BINARIES)

########################################
# redis-proxy and related targets

redis-proxy: rproxy/*.go cmd/redis-proxy/*.go
	./scripts/go build -o "$@" github.com/codility/redis-proxy/cmd/redis-proxy

.PHONY: clean
clean:
	rm -rf $(BINARIES) .gopath

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

.PHONY: test
test:
	./scripts/go test -v github.com/codility/redis-proxy/fakeredis/
	./scripts/go test -v github.com/codility/redis-proxy/resp/
	./scripts/go test -v github.com/codility/redis-proxy/rproxy/

########################################
# switch-test and related targets

switch-test: redis-proxy cmd/switch-test/*.go
	./scripts/go build -o "$@" github.com/codility/redis-proxy/cmd/switch-test

run-switch-test: switch-test redis-proxy
	./switch-test
