VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GOFLAGS := -trimpath -ldflags="-s -w -X main.version=$(VERSION)"
BINARY := trusttunnel-manager
WEB_DIR := web
DIST_DIR := web/dist
BUILD_DIR := build

ARCHS := mipsle mips arm64 arm
GOARM_VAL := 7

.PHONY: all dev build build-all ipk clean web go

all: build

# --- Development ---

dev:
	@echo "Start Vite dev server and Go in parallel"
	cd $(WEB_DIR) && npm run dev &
	go run ./cmd/trusttunnel-manager -dev

# --- Web ---

web:
	cd $(WEB_DIR) && npm ci && npm run build

# --- Go (host platform) ---

go: web
	go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/trusttunnel-manager

build: go

# --- Cross-compilation ---

build-all: web
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-mipsel ./cmd/trusttunnel-manager
	GOOS=linux GOARCH=mips GOMIPS=softfloat go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-mips ./cmd/trusttunnel-manager
	GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-aarch64 ./cmd/trusttunnel-manager
	GOOS=linux GOARCH=arm GOARM=$(GOARM_VAL) go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-armv7 ./cmd/trusttunnel-manager

# --- IPK packaging ---

ipk: build-all
	@for arch in mipsel mips aarch64 armv7; do \
		./packaging/build-ipk.sh $$arch $(VERSION); \
	done

# --- Clean ---

clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR)
	cd $(WEB_DIR) && rm -rf node_modules dist
