VERSION := v1.0.0
COMMIT := $(shell git rev-parse HEAD)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: help build

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the cccli binary
	mkdir -p dist; \
	cd src; \
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o ../dist/cccli-linux-arm ./cmd; \
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o ../dist/cccli-linux-amd ./cmd; \
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o ../dist/cccli-windows-amd ./cmd; \
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o ../dist/cccli-darwin ./cmd;
