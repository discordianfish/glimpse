ARCH := $(subst x86_64,amd64,$(subst i386,386,$(shell uname -m)))
OS   := $(subst Darwin,darwin,$(subst Linux,linux,$(shell uname)))

GOBIN ?= $(firstword $(subst :, ,$(GOPATH)))/bin
GODEP := $(GOBIN)/godep

DEPS_DIR  := .deps
AGENT_BIN := $(DEPS_DIR)/glimpse-agent

CONSUL_VER ?= 0.4.1
CONSUL_BIN := $(DEPS_DIR)/consul
CONSUL_ZIP := $(CONSUL_VER)_$(OS)_$(ARCH).zip
CONSUL_URL := "https://dl.bintray.com/mitchellh/consul/$(CONSUL_ZIP)"


default: test

build: glimpse-agent

clean:
	git clean -fdx

test: unit-test acceptance-test

unit-test: $(GODEP)
	$(GODEP) go test -v

acceptance-test: $(GODEP) glimpse-agent $(CONSUL_BIN)
	$(GODEP) go test -v -tags acceptance

.PHONY: build clean test unit-test acceptance-test
.SUFFIXES: # Disable default suffixes

glimpse-agent: $(GODEP) *.go
	$(GODEP) go build -o $@

$(GODEP):
	go get github.com/tools/godep

$(DEPS_DIR)/$(CONSUL_ZIP):
	mkdir -p $(DEPS_DIR)
	wget --quiet $(CONSUL_URL) -O $@

$(CONSUL_BIN): $(DEPS_DIR)/$(CONSUL_ZIP)
	unzip -q -d $(DEPS_DIR) $<
	@touch $@

