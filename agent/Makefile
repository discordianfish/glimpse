# TODO(alx): Fix extraction of ARCH.
ARCH	:=  amd64
OS		:=	$(subst Darwin,darwin,$(subst Linux,linux,$(shell uname)))

BUILD_DIR := .build
DEPS_DIR 	:= .deps

CONSUL_BIN := $(DEPS_DIR)/consul
CONSUL_VER := 0.4.1
CONSUL_ZIP :=	$(CONSUL_VER)_$(OS)_$(ARCH).zip
CONSUL_URL := "https://dl.bintray.com/mitchellh/consul/$(CONSUL_ZIP)"

default: test

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(DEPS_DIR):
	mkdir -p $(DEPS_DIR)

# TODO(alx): Move to tar base distribution.
$(DEPS_DIR)/$(CONSUL_ZIP): $(DEPS_DIR)
	wget $(CONSUL_URL) -O $@
	touch $@

$(CONSUL_BIN): $(DEPS_DIR)/$(CONSUL_ZIP)
	unzip -d $(DEPS_DIR) $<
	touch $@

.PHONY: acceptance-test
acceptance-test: $(BUILD_DIR) $(CONSUL_BIN)
	BUILD_DIR=$(BUILD_DIR) go test -v -tags acceptance

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -rf $(DEPS_DIR)
	# git clean -fdx

.PHONY: test
test: acceptance-test
