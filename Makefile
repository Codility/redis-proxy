BINARIES=redis-proxy switch-test

all: $(BINARIES)

########################################
# redis-proxy and related targets

redis-proxy: rproxy/*.go cmd/redis-proxy/*.go
	./scripts/go build -o "$@" github.com/Codility/redis-proxy/cmd/redis-proxy

.PHONY: clean
clean:
	rm -rf $(BINARIES) .gopath

.PHONY: config
config: config.json

config.json:
	echo "Create config.json based on config_example.json"
	exit 1

.PHONY: run
run: redis-proxy config.json
	./redis-proxy config.json

.PHONY: test
test: goimports govet
	./scripts/go test -v github.com/Codility/redis-proxy/fakeredis/
	./scripts/go test -v github.com/Codility/redis-proxy/resp/
	./scripts/go test -v github.com/Codility/redis-proxy/rproxy/

.PHONY: govet
govet:
	find . -name '*.go' -and -not -path './vendor/*' -and -not -path './.gopath/*' | \
		while read f; do echo `dirname "$$f"`; done | uniq | xargs ./scripts/go vet

.PHONY: goimports
goimports:
	./scripts/go get golang.org/x/tools/cmd/goimports
	@echo "Running goimports..."
	@output=$$(find . -name '*.go' -and -not -path './vendor/*' -and -not -path './.gopath/*' | xargs ./.gopath/bin/goimports -d) && \
		if ! [ -z "$$output" ]; then \
			echo "$$output"; \
			echo "goimports failed!"; \
			exit 1; \
		fi
	@echo "goimports passed"

########################################
# switch-test and related targets

switch-test: redis-proxy cmd/switch-test/*.go
	./scripts/go build -o "$@" github.com/Codility/redis-proxy/cmd/switch-test

run-switch-test: switch-test redis-proxy
	./switch-test
