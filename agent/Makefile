ARCH := $(subst x86_64,amd64,$(subst i386,386,$(shell uname -m)))
OS   := $(subst Darwin,darwin,$(subst Linux,linux,$(shell uname)))

GOBIN ?= $(firstword $(subst :, ,$(GOPATH)))/bin
GODEP := $(GOBIN)/godep

DEPS_DIR  := .deps

CONSUL_BIN := consul
CONSUL_VER ?= 0.5.0
CONSUL_ZIP := $(CONSUL_VER)_$(OS)_$(ARCH).zip
CONSUL_TAR := consul-$(CONSUL_VER)_$(OS)_$(ARCH).tar.gz
CONSUL_URL := "https://dl.bintray.com/mitchellh/consul/$(CONSUL_ZIP)"
CONSUL_DST ?= http://ent/iss/consul

AGENT_BIN := glimpse-agent
AGENT_VER := 0.5.0
AGENT_TAR := $(AGENT_BIN)-$(AGENT_VER)_$(OS)_$(ARCH).tar.gz
AGENT_DST ?= http://ent/iss/glimpse
AGENT_FLG := -ldflags " -X main.version $(AGENT_VER)"


default: test

build: $(AGENT_BIN)

try: build
	@./$(AGENT_BIN)

clean:
	rm -rf $(AGENT_BIN) $(DEPS_DIR)

test: unit-test acceptance-test

unit-test: $(GODEP)
	$(GODEP) go test

acceptance-test: $(GODEP) $(AGENT_BIN) $(DEPS_DIR)/$(CONSUL_BIN)
	$(GODEP) go test -tags acceptance -run TestAgent

release: $(AGENT_TAR)
	curl -X POST --data-binary @$< $(AGENT_DST)/$<

repackage: $(DEPS_DIR)/$(CONSUL_TAR)
	curl -X POST --data-binary @$< $(CONSUL_DST)/$(CONSUL_TAR)

.PHONY: build try clean test unit-test acceptance-test release
.SUFFIXES: # Disable default suffixes

$(DEPS_DIR):
	mkdir -p $(DEPS_DIR)

$(AGENT_BIN): $(GODEP) *.go Makefile
	$(GODEP) go build $(AGENT_FLG) -o $@

$(AGENT_TAR): $(AGENT_BIN)
	tar czf $@ $<

$(GODEP):
	go get github.com/tools/godep

$(DEPS_DIR)/$(CONSUL_ZIP): | $(DEPS_DIR)
	wget --quiet $(CONSUL_URL) -O $@

$(DEPS_DIR)/$(CONSUL_TAR): $(DEPS_DIR)/$(CONSUL_BIN)
	tar -C $(DEPS_DIR) -czf $@ $(CONSUL_BIN)

$(DEPS_DIR)/$(CONSUL_BIN): $(DEPS_DIR)/$(CONSUL_ZIP)
	unzip -q -d $(DEPS_DIR) $<
	@touch $@
