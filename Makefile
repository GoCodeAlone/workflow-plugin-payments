.PHONY: build test install cross-build clean

BINARY_NAME = workflow-plugin-payments
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"
INSTALL_DIR ?= data/plugins/$(BINARY_NAME)
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

build:
	GOPRIVATE=github.com/GoCodeAlone/* go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

test:
	GOPRIVATE=github.com/GoCodeAlone/* go test ./... -v -race

install: build
	mkdir -p $(DESTDIR)/$(INSTALL_DIR)
	cp bin/$(BINARY_NAME) $(DESTDIR)/$(INSTALL_DIR)/
	cp plugin.json $(DESTDIR)/$(INSTALL_DIR)/

cross-build:
	@mkdir -p bin
	@for platform in $(PLATFORMS); do \
		os=$${platform%%/*}; \
		arch=$${platform##*/}; \
		output=bin/$(BINARY_NAME)-$${os}-$${arch}; \
		echo "Building $${output}..."; \
		CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} GOPRIVATE=github.com/GoCodeAlone/* \
			go build $(LDFLAGS) -o $${output} ./cmd/$(BINARY_NAME); \
	done

clean:
	rm -rf bin/
