package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"velotrust/gateway/config"
	"velotrust/gateway/identity"
	gatewayruntime "velotrust/gateway/runtime"
)

type validationSummary struct {
	GatewayID     string `json:"gateway_id"`
	Generation    uint64 `json:"generation"`
	Interface     string `json:"interface"`
	Peers         int    `json:"peers"`
	Routes        int    `json:"routes"`
	Rules         int    `json:"rules"`
	DefaultPolicy string `json:"default_policy"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: gateway-agent <run|validate|keygen> [options]")
		return 2
	}
	switch args[0] {
	case "run":
		return runAgent(args[1:], stdout, stderr)
	case "validate":
		return validateConfig(args[1:], stdout, stderr)
	case "keygen":
		return generateKeys(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, "usage: gateway-agent <run|validate|keygen> [options]")
		fmt.Fprintln(stdout, "run --config PATH [--health-listen ADDRESS] [--log-level debug|info|warn|error]")
		fmt.Fprintln(stdout, "run applies the local gateway config and serves /healthz and /readyz.")
		fmt.Fprintln(stdout, "validate checks the config without exposing its private key.")
		fmt.Fprintln(stdout, "keygen creates one WireGuard key pair, which is also the node's network identity.")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		fmt.Fprintln(stderr, "usage: gateway-agent <run|validate|keygen> [options]")
		return 2
	}
}

func generateKeys(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("keygen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	directory := flags.String("dir", "", "private directory for generated keys")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *directory == "" {
		fmt.Fprintln(stderr, "keygen requires --dir <private-key-directory>")
		return 2
	}
	publicKey, err := identity.Generate(*directory)
	if err != nil {
		fmt.Fprintf(stderr, "generate gateway key: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(publicKey); err != nil {
		fmt.Fprintf(stderr, "write public keys: %v\n", err)
		return 1
	}
	return 0
}

func runAgent(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to a private gateway config file")
	healthAddress := flags.String("health-listen", "127.0.0.1:8765", "address for local health endpoints")
	logLevel := flags.String("log-level", "info", "log level: debug, info, warn, or error")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *configPath == "" {
		fmt.Fprintln(stderr, "run requires --config <private-config.json> [--log-level debug|info|warn|error]")
		return 2
	}
	if err := configureLogger(*logLevel, stderr); err != nil {
		fmt.Fprintf(stderr, "configure gateway logging: %v\n", err)
		return 2
	}
	slog.Info("starting gateway agent", "health_listen", *healthAddress, "log_level", strings.ToLower(*logLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := gatewayruntime.RunFile(ctx, *configPath, *healthAddress); err != nil {
		fmt.Fprintf(stderr, "gateway agent stopped: %v\n", err)
		return 1
	}
	return 0
}

func configureLogger(rawLevel string, output io.Writer) error {
	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(rawLevel)) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return fmt.Errorf("unsupported log level %q", rawLevel)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: level})))
	return nil
}

func validateConfig(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to a private gateway config file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *configPath == "" {
		fmt.Fprintln(stderr, "validate requires --config <private-config.json>")
		return 2
	}
	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "invalid gateway config: %v\n", err)
		return 1
	}
	policy := "default-deny"
	if len(cfg.Rules) > 0 {
		policy = "explicit-rules"
	}
	summary := validationSummary{
		GatewayID:     cfg.GatewayID,
		Generation:    cfg.Generation,
		Interface:     cfg.Tunnel.Interface,
		Peers:         len(cfg.Tunnel.Peers),
		Routes:        len(cfg.Routes),
		Rules:         len(cfg.Rules),
		DefaultPolicy: policy,
	}
	if err := json.NewEncoder(stdout).Encode(summary); err != nil {
		fmt.Fprintf(stderr, "write validation result: %v\n", err)
		return 1
	}
	return 0
}
