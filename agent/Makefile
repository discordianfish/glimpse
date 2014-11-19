ARCH := $(subst x86_64,amd64,$(subst i386,386,$(shell uname -m)))
OS   := $(subst Darwin,darwin,$(subst Linux,linux,$(shell uname)))

GOBIN ?= $(firstword $(subst :, ,$(GOPATH)))/bin
GODEP := $(GOBIN)/godep

DEPS_DIR := .deps

AGENT_BIN := $(DEPS_DIR)/glimpse-agent

CONSUL_BIN := $(DEPS_DIR)/consul
CONSUL_VER := 0.4.1
CONSUL_ZIP := $(CONSUL_VER)_$(OS)_$(ARCH).zip
CONSUL_URL := "https://dl.bintray.com/mitchellh/consul/$(CONSUL_ZIP)"

default: test

.PHONY: build
build: glimpse-agent

.PHONY: acceptance-test
acceptance-test: build $(CONSUL_BIN) $(GODEP)
	$(GODEP) go test -v -tags acceptance

.PHONY: unit-test
unit-test: $(GODEP)
	$(GODEP) go test -v

.PHONY: clean
clean:
	git clean -fdx

.PHONY: test
test: unit-test acceptance-test

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

.SUFFIXES: # Disable default suffixes
