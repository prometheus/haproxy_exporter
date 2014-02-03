VERSION  := 0.2.0

SRC      := $(wildcard *.go)
TARGET   := haproxy_exporter

OS   := $(subst Darwin,darwin,$(subst Linux,linux,$(shell uname)))
ARCH := $(subst x86_64,amd64,$(shell uname -m))

GOOS   ?= $(OS)
GOARCH ?= $(ARCH)
GOPKG  := go1.2.$(OS)-$(ARCH).tar.gz
GOROOT ?= $(CURDIR)/.deps/go
GOPATH ?= $(CURDIR)/.deps/gopath
GOCC   := $(GOROOT)/bin/go
GOLIB  := $(GOROOT)/pkg/$(GOOS)_$(GOARCH)
GO     := GOROOT=$(GOROOT) GOPATH=$(GOPATH) $(GOCC)

SUFFIX  := $(GOOS)-$(GOARCH)
BINARY  := bin/$(TARGET)
ARCHIVE := $(TARGET)-$(VERSION).$(SUFFIX).tar.gz

default: build

build: $(BINARY)

.deps/$(GOPKG):
	mkdir -p .deps
	curl -o .deps/$(GOPKG) http://go.googlecode.com/files/$(GOPKG)

$(GOCC): .deps/$(GOPKG)
	tar -C .deps -xzf .deps/$(GOPKG)
	touch $@

$(GOLIB):
	cd .deps/go/src && CGO_ENABLED=0 ./make.bash

dependencies: $(SRC)
	$(GO) get -d

$(BINARY): $(GOCC) $(GOLIB) $(SRC) dependencies
	$(GO) build -o $@

$(ARCHIVE): $(BINARY)
	tar -czf $@ bin/

upload: REMOTE     ?= $(error "can't upload, REMOTE not set")
upload: REMOTE_DIR ?= $(error "can't upload, REMOTE_DIR not set")
upload: $(ARCHIVE)
	scp $(ARCHIVE) $(REMOTE):$(REMOTE_DIR)/$(ARCHIVE)

release: REMOTE     ?= $(error "can't release, REMOTE not set")
release: REMOTE_DIR ?= $(error "can't release, REMOTE_DIR not set")
release:
	GOOS=linux  REMOTE=$(REMOTE) REMOTE_DIR=$(REMOTE_DIR) $(MAKE) upload
	GOOS=darwin REMOTE=$(REMOTE) REMOTE_DIR=$(REMOTE_DIR) $(MAKE) upload

test:
	go test

benchmark:
	go test -bench . -test.benchmem -benchtime=10s

clean:
	rm -rf bin

.PHONY: test tag dependencies clean release upload
