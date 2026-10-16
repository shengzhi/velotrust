GO ?= go
CGO_ENABLED ?= 0
BIN_DIR ?= bin
GATEWAY_AGENT := $(BIN_DIR)/gateway-agent
CLIENT_CLI_WINDOWS := $(BIN_DIR)/client-cli-windows-amd64.exe
VERSION ?= dev
WINTUN_DLL ?=

.PHONY: help build-gateway-agent build-client-cli-windows

help:
	@printf '%s\n' 'VeloTrust monorepo' '  make build-gateway-agent      Build a static gateway-agent into $(GATEWAY_AGENT)' '  make build-client-cli-windows Build Windows amd64 client CLI into $(CLIENT_CLI_WINDOWS)' '    Optional: WINTUN_DLL=/path/to/wintun.dll copies the runtime beside the CLI.' 'See docs/ARCHITECTURE.md for package boundaries.'

build-gateway-agent:
	@mkdir -p "$(BIN_DIR)"
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -o "$(GATEWAY_AGENT)" ./cmd/gateway-agent

build-client-cli-windows:
	@mkdir -p "$(BIN_DIR)"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags "-X main.version=$(VERSION)" -o "$(CLIENT_CLI_WINDOWS)" ./client/cmd/client-cli
	@if [ -n "$(WINTUN_DLL)" ]; then cp "$(WINTUN_DLL)" "$(BIN_DIR)/wintun.dll"; fi
	@if [ -z "$(WINTUN_DLL)" ]; then printf '%s\n' 'Runtime requirement: place x64 wintun.dll beside the executable.'; fi
