# TODO(alx): Fix extraction of ARCH.
ARCH := amd64
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

$(GODEP):
	go get github.com/tools/godep

$(DEPS_DIR):
	mkdir -p $(DEPS_DIR)

# TODO(alx): Move to tar base distribution.
$(DEPS_DIR)/$(CONSUL_ZIP):
	wget --quiet $(CONSUL_URL) -O $@

$(AGENT_BIN): $(GODEP) *.go
	$(GODEP) go build -o $@

$(CONSUL_BIN): $(DEPS_DIR)/$(CONSUL_ZIP)
	unzip -q -d $(DEPS_DIR) $<
	@touch $@

# TODO(alx): Fix dependencies.
.PHONY: acceptance-test
acceptance-test: $(GODEP) $(DEPS_DIR) $(AGENT_BIN) $(CONSUL_BIN)
	$(GODEP) go test -v -tags acceptance

.PHONY: unit-test
unit-test: $(GODEP)
	$(GODEP) go test -v

.PHONY: clean
clean:
	git clean -fdx

.PHONY: test
test: unit-test acceptance-test

.SUFFIXES: # Disable default suffixes
