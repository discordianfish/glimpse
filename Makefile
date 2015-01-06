.PHONY: setup
setup:
	ln -sf ../../misc/hooks/pre-commit .git/hooks/

.PHONY: test
test: fmt vet
	$(MAKE) -C agent test

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...
