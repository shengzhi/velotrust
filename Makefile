GO ?= go
CGO_ENABLED ?= 0
BIN_DIR ?= bin
GATEWAY_AGENT := $(BIN_DIR)/gateway-agent
CLIENT_CLI_WINDOWS := $(BIN_DIR)/client-cli-windows-amd64.exe
HUB := $(BIN_DIR)/hub
MANAGEMENT := $(BIN_DIR)/management
VERSION ?= dev
WINTUN_DLL ?=
PROTOC ?= protoc

.PHONY: help build build-gateway-agent build-client-cli-windows build-hub build-management generate-hub-proto

help:
	@printf '%s\n' 'VeloTrust monorepo' '  make build                   Build HUB, Gateway Agent, and Management' '  make build-gateway-agent      Build a static gateway-agent into $(GATEWAY_AGENT)' '  make build-client-cli-windows Build Windows amd64 client CLI into $(CLIENT_CLI_WINDOWS)' '    Optional: WINTUN_DLL=/path/to/wintun.dll copies the runtime beside the CLI.' '  make generate-hub-proto       Generate HUB gRPC Go contracts' '  make build-hub               Build the HUB server into $(HUB)' '  make build-management        Build the Management JRPC server into $(MANAGEMENT)' 'See docs/ARCHITECTURE.md for package boundaries.'

build: build-hub build-gateway-agent build-management

build-gateway-agent:
	@mkdir -p "$(BIN_DIR)"
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -ldflags "-X velotrust/gateway/internal/buildinfo.Version=$(VERSION)" -o "$(GATEWAY_AGENT)" ./cmd/gateway-agent

build-client-cli-windows:
	@mkdir -p "$(BIN_DIR)"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags "-X main.version=$(VERSION)" -o "$(CLIENT_CLI_WINDOWS)" ./client/cmd/client-cli
	@if [ -n "$(WINTUN_DLL)" ]; then cp "$(WINTUN_DLL)" "$(BIN_DIR)/wintun.dll"; fi
	@if [ -z "$(WINTUN_DLL)" ]; then printf '%s\n' 'Runtime requirement: place x64 wintun.dll beside the executable.'; fi

generate-hub-proto:
	$(PROTOC) --proto_path=. --go_out=. --go_opt=module=velotrust --go-grpc_out=. --go-grpc_opt=module=velotrust shared/proto/v1/hub.proto

build-hub:
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -o "$(HUB)" ./cmd/hub

build-management:
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -ldflags "-X main.version=$(VERSION)" -o "$(MANAGEMENT)" ./cmd/management
