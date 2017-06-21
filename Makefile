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

.PHONY: test
test:
	./scripts/go test -v gitlab.codility.net/marcink/redis-proxy/fakeredis/
	./scripts/go test -v gitlab.codility.net/marcink/redis-proxy/resp/
	./scripts/go test -v gitlab.codility.net/marcink/redis-proxy/rproxy/

########################################
# switch-test and related targets

switch-test: redis-proxy cmd/switch-test/*.go
	./scripts/go build -o "$@" gitlab.codility.net/marcink/redis-proxy/cmd/switch-test

run-switch-test: switch-test redis-proxy
	./switch-test

########################################
# Jenkins[file] additions

TARBALL=$(shell git rev-parse HEAD).tar.gz

# See redis-proxy build on Jenkins. Note that this expects AWS configuration
# variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY) in the environment.
.PHONY: upload
upload: redis-proxy
	tar czf $(TARBALL) redis-proxy
	sha512sum $(TARBALL) | tee $(TARBALL).sha512
	s3cmd -c s3cmd.conf put $(TARBALL) $(TARBALL).sha512 s3://codility-dist/redis-proxy/
	echo $(TARBALL) | s3cmd -c s3cmd.conf put - s3://codility-dist/redis-proxy/current
	rm $(TARBALL) $(TARBALL).sha512
