GO ?= go
DOCKER ?= docker
DIST_DIR ?= dist
IMAGE ?= netprob

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
IMAGE_TAG ?= $(VERSION)

# Freeze auto-detected metadata once so every artifact from one invocation is identical.
VERSION := $(VERSION)
COMMIT := $(COMMIT)
BUILD_DATE := $(BUILD_DATE)
IMAGE_TAG := $(IMAGE_TAG)

VERSION_LDFLAGS = -X netprob/internal/buildinfo.Version=$(VERSION) -X netprob/internal/buildinfo.Commit=$(COMMIT) -X netprob/internal/buildinfo.Date=$(BUILD_DATE)
RELEASE_LDFLAGS = -s -w $(VERSION_LDFLAGS)

.PHONY: build frontend release release-binaries agent-linux agent-linux-amd64 agent-linux-arm64 docker-image verify-agent-linux-amd64 clean-dist version

version:
	@echo $(VERSION)

frontend:
	cd web && npm ci && npm run lint && npm run build
	rsync -a --delete web/dist/ internal/webui/dist/

build:
	mkdir -p bin
	$(GO) build -trimpath -ldflags "$(VERSION_LDFLAGS)" -o bin/netprob ./cmd/netprob

release: release-binaries docker-image

release-binaries: frontend agent-linux
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "$(RELEASE_LDFLAGS)" -o $(DIST_DIR)/netprob-server-linux-amd64 ./cmd/netprob

agent-linux: agent-linux-amd64 agent-linux-arm64

agent-linux-amd64:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -tags agentonly -trimpath -ldflags "$(RELEASE_LDFLAGS)" -o $(DIST_DIR)/netprob-agent-linux-amd64 ./cmd/netprob

agent-linux-arm64:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -tags agentonly -trimpath -ldflags "$(RELEASE_LDFLAGS)" -o $(DIST_DIR)/netprob-agent-linux-arm64 ./cmd/netprob

docker-image:
	$(DOCKER) build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--tag $(IMAGE):$(IMAGE_TAG) .

verify-agent-linux-amd64: agent-linux-amd64
	file $(DIST_DIR)/netprob-agent-linux-amd64
	@if ldd $(DIST_DIR)/netprob-agent-linux-amd64 >/dev/null 2>&1; then \
		echo "error: agent artifact unexpectedly uses shared libraries"; \
		exit 1; \
	else \
		echo "agent artifact is static (no host glibc dependency)"; \
	fi
	$(DIST_DIR)/netprob-agent-linux-amd64 -version

clean-dist:
	rm -f $(DIST_DIR)/netprob-server-linux-amd64 $(DIST_DIR)/netprob-agent-linux-amd64 $(DIST_DIR)/netprob-agent-linux-arm64
