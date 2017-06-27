STEP_TEMPLATE="\n========================================\nStep: STEPNAME\n========================================\n"
IS_MASTER=
CURRENT_SHA=$(shell git rev-parse HEAD)
MASTER_SHA=$(shell git rev-parse origin/master)
ifeq ($(CURRENT_SHA),$(MASTER_SHA))
	IS_MASTER=1
endif


.PHONY: all
all:
	@echo Targets:
	@echo  - build-test-upload


.PHONY: build-test-upload
build-test-upload: redis-proxy test upload-if-master


.PHONY: redis-proxy
redis-proxy:
	@echo $(subst STEPNAME,make redis-proxy,$(STEP_TEMPLATE))
	make -f Makefile redis-proxy
	ls -l redis-proxy


.PHONY: test
test: redis-proxy
	@echo $(subst STEPNAME,test,$(STEP_TEMPLATE))
	make -f Makefile test


TARBALL=$(shell git rev-parse HEAD).tar.gz

# See redis-proxy build on Jenkins. Note that this expects AWS configuration
# variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY) in the environment.
.PHONY: upload-if-master
upload-if-master: redis-proxy
	@echo $(subst STEPNAME,upload-if-master,$(STEP_TEMPLATE))
ifeq ($(IS_MASTER),1)
	tar czf $(TARBALL) redis-proxy
	sha512sum $(TARBALL) | tee $(TARBALL).sha512
	s3cmd -c s3cmd.conf put $(TARBALL) $(TARBALL).sha512 s3://codility-dist/redis-proxy/
	echo $(CURRENT_SHA) | s3cmd -c s3cmd.conf put - s3://codility-dist/redis-proxy/current
	rm $(TARBALL) $(TARBALL).sha512
else
	@echo Not on master, skipping upload.
endif
