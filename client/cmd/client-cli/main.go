// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	clientconfig "velotrust/client/internal/config"
	clienttunnel "velotrust/client/internal/tunnel/wireguard"
	winplatform "velotrust/client/system/windows"
)

var version = "dev"

type versionReport struct {
	Version      string `json:"version"`
	OperatingSys string `json:"os"`
	Architecture string `json:"architecture"`
}

type StatusReport struct {
	Platform        string                         `json:"platform"`
	Architecture    string                         `json:"architecture"`
	Elevated        bool                           `json:"elevated"`
	AgentService    winplatform.AgentServiceStatus `json:"agent_service"`
	TunnelStatus    string                         `json:"tunnel_status"`
	TunnelStatusMsg string                         `json:"tunnel_status_message"`
}

type configReport struct {
	Valid           bool   `json:"valid"`
	Path            string `json:"path"`
	SchemaVersion   int    `json:"schema_version,omitempty"`
	InstanceID      string `json:"instance_id,omitempty"`
	ControlPlaneURL string `json:"control_plane_url,omitempty"`
	LogLevel        string `json:"log_level,omitempty"`
	Error           string `json:"error,omitempty"`
}

type diagnoseReport struct {
	StatusReport
	Config configReport `json:"config"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	case "diagnose":
		return runDiagnose(args[1:], stdout, stderr)
	case "keygen":
		return runKeygen(args[1:], stdout, stderr)
	case "connect":
		return runConnect(args[1:], stdout, stderr)
	case "disconnect":
		return runDisconnect(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	flags := newFlags("version", stderr)
	jsonOutput := flags.Bool("json", false, "write machine-readable JSON")
	if err := parseFlags(flags, args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	report := versionReport{Version: version, OperatingSys: runtime.GOOS, Architecture: runtime.GOARCH}
	if *jsonOutput {
		return writeJSON(stdout, report, stderr)
	}
	fmt.Fprintf(stdout, "VeloTrust Client CLI %s (%s/%s)\n", report.Version, report.OperatingSys, report.Architecture)
	return 0
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	flags := newFlags("status", stderr)
	jsonOutput := flags.Bool("json", false, "write machine-readable JSON")
	if err := parseFlags(flags, args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	report, err := inspectStatus()
	if err != nil {
		fmt.Fprintf(stderr, "inspect client status: %v\n", err)
		return 1
	}
	if *jsonOutput {
		return writeJSON(stdout, report, stderr)
	}
	fmt.Fprintf(stdout, "Platform: %s/%s\nElevated: %t\nAgent service: %s\nTunnel: %s\n", report.Platform, report.Architecture, report.Elevated, report.AgentService.State, report.TunnelStatusMsg)
	return 0
}

func runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "validate" {
		fmt.Fprintln(stderr, "usage: client-cli config validate [--file PATH] [--json]")
		return 2
	}
	flags := newFlags("config validate", stderr)
	path := flags.String("file", "", "client config file (default: platform config path)")
	jsonOutput := flags.Bool("json", false, "write machine-readable JSON")
	if err := parseFlags(flags, args[1:]); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	configPath, err := resolveConfigPath(*path)
	if err != nil {
		fmt.Fprintf(stderr, "resolve client config path: %v\n", err)
		return 1
	}
	report := validateConfig(configPath)
	if *jsonOutput {
		if exitCode := writeJSON(stdout, report, stderr); exitCode != 0 {
			return exitCode
		}
	} else if report.Valid {
		fmt.Fprintf(stdout, "valid client config: %s (instance=%s, log_level=%s)\n", report.Path, report.InstanceID, report.LogLevel)
	} else {
		fmt.Fprintf(stderr, "invalid client config %s: %s\n", report.Path, report.Error)
	}
	if !report.Valid {
		return 1
	}
	return 0
}

func runDiagnose(args []string, stdout, stderr io.Writer) int {
	flags := newFlags("diagnose", stderr)
	path := flags.String("config", "", "client config file (default: platform config path)")
	jsonOutput := flags.Bool("json", false, "write machine-readable JSON")
	if err := parseFlags(flags, args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	configPath, err := resolveConfigPath(*path)
	if err != nil {
		fmt.Fprintf(stderr, "resolve client config path: %v\n", err)
		return 1
	}
	status, err := inspectStatus()
	if err != nil {
		fmt.Fprintf(stderr, "inspect client status: %v\n", err)
		return 1
	}
	report := diagnoseReport{StatusReport: status, Config: validateConfig(configPath)}
	if *jsonOutput {
		if exitCode := writeJSON(stdout, report, stderr); exitCode != 0 {
			return exitCode
		}
	} else {
		fmt.Fprintf(stdout, "Platform: %s/%s\nElevated: %t\nAgent service: %s\nTunnel: %s\nConfig file: %s\n", report.Platform, report.Architecture, report.Elevated, report.AgentService.State, report.TunnelStatusMsg, report.Config.Path)
		if report.Config.Valid {
			fmt.Fprintf(stdout, "Config validation: valid (instance=%s, log_level=%s)\n", report.Config.InstanceID, report.Config.LogLevel)
		} else {
			fmt.Fprintf(stdout, "Config validation: %s\n", report.Config.Error)
		}
	}
	if !report.Config.Valid {
		return 1
	}
	return 0
}

func runKeygen(args []string, stdout, stderr io.Writer) int {
	flags := newFlags("keygen", stderr)
	directory := flags.String("dir", "", "private directory for the WireGuard key file")
	if err := parseFlags(flags, args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *directory == "" {
		fmt.Fprintln(stderr, "keygen requires --dir <private-directory>")
		return 2
	}
	publicKey, err := winplatform.GenerateWireGuardKey(*directory)
	if err != nil {
		fmt.Fprintf(stderr, "generate client WireGuard key: %v\n", err)
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "public_key=%s\nprivate_key_file=%s\n", publicKey, filepath.Join(*directory, "client.key")); err != nil {
		fmt.Fprintf(stderr, "write key output: %v\n", err)
		return 1
	}
	return 0
}

func runConnect(args []string, stdout, stderr io.Writer) int {
	flags := newFlags("connect", stderr)
	path := flags.String("config", "", "client config file containing a static gateway tunnel profile")
	logLevel := flags.String("log-level", "", "log level: debug, info, warn, or error (overrides config)")
	if err := parseFlags(flags, args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	configPath, err := resolveConfigPath(*path)
	if err != nil {
		fmt.Fprintf(stderr, "resolve client config path: %v\n", err)
		return 1
	}
	cfg, err := clientconfig.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load client config: %v\n", err)
		return 1
	}
	if cfg.Tunnel == nil {
		fmt.Fprintln(stderr, "client config has no tunnel profile; add provisioned tunnel.peers and an assigned address")
		return 1
	}
	effectiveLogLevel := cfg.EffectiveLogLevel()
	if *logLevel != "" {
		effectiveLogLevel = *logLevel
	}
	if err := configureLogger(effectiveLogLevel, stderr); err != nil {
		fmt.Fprintf(stderr, "configure client logging: %v\n", err)
		return 2
	}
	slog.Info("starting client tunnel", "instance_id", cfg.InstanceID, "interface", cfg.Tunnel.InterfaceName, "log_level", strings.ToLower(effectiveLogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := clienttunnel.Connect(ctx, cfg.InstanceID, configPath, *cfg.Tunnel, stdout); err != nil {
		fmt.Fprintf(stderr, "client connection stopped: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "Client tunnel disconnected; configured routes and address were removed.")
	return 0
}

func runDisconnect(args []string, stdout, stderr io.Writer) int {
	flags := newFlags("disconnect", stderr)
	path := flags.String("config", "", "client config file containing the tunnel interface name")
	if err := parseFlags(flags, args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	configPath, err := resolveConfigPath(*path)
	if err != nil {
		fmt.Fprintf(stderr, "resolve client config path: %v\n", err)
		return 1
	}
	cfg, err := clientconfig.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load client config: %v\n", err)
		return 1
	}
	if cfg.Tunnel == nil {
		fmt.Fprintln(stderr, "client config has no tunnel profile")
		return 1
	}
	if !winplatform.IsElevated() {
		fmt.Fprintln(stderr, "disconnect requires an elevated PowerShell session to remove tunnel routes")
		return 1
	}
	releaseTunnelLock, err := winplatform.AcquireTunnelLock()
	if err != nil {
		fmt.Fprintf(stderr, "acquire client tunnel lock: %v\n", err)
		return 1
	}
	defer func() { _ = releaseTunnelLock() }()
	interfaceName := cfg.Tunnel.InterfaceName
	if interfaceName == "" {
		interfaceName = "VeloTrust"
	}
	cleanupErr := errors.Join(
		wrapCommandError("restore Windows DNS policy", winplatform.CleanupDNSCapture(cfg.InstanceID)),
		wrapCommandError("clean up client tunnel networking", winplatform.CleanupTunnelNetwork(interfaceName)),
	)
	if cleanupErr != nil {
		fmt.Fprintf(stderr, "disconnect cleanup failed: %v\n", cleanupErr)
		return 1
	}
	fmt.Fprintln(stdout, "Client DNS policy, routes, and manually assigned addresses were restored.")
	return 0
}

func wrapCommandError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func inspectStatus() (StatusReport, error) {
	service, err := winplatform.InspectAgentService()
	if err != nil {
		return StatusReport{}, err
	}
	return StatusReport{
		Platform:        runtime.GOOS,
		Architecture:    runtime.GOARCH,
		Elevated:        winplatform.IsElevated(),
		AgentService:    service,
		TunnelStatus:    "unknown",
		TunnelStatusMsg: "tunnel status is not reported by the local agent",
	}, nil
}

func validateConfig(path string) configReport {
	report := configReport{Path: filepath.Clean(path)}
	cfg, err := clientconfig.Load(path)
	if err != nil {
		report.Error = err.Error()
		return report
	}
	report.Valid = true
	report.SchemaVersion = cfg.SchemaVersion
	report.InstanceID = cfg.InstanceID
	report.ControlPlaneURL = cfg.ControlPlaneURL
	report.LogLevel = cfg.EffectiveLogLevel()
	return report
}

func resolveConfigPath(path string) (string, error) {
	if path != "" {
		return filepath.Abs(path)
	}
	if runtime.GOOS == "windows" {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			return "", errors.New("ProgramData is unset; pass --file explicitly")
		}
		return filepath.Join(programData, "VeloTrust", "Client", "client.json"), nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(configDir, "velotrust", "client.json"), nil
}

func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags
}

func parseFlags(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	return nil
}

func writeJSON(stdout io.Writer, value any, stderr io.Writer) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintf(stderr, "write JSON output: %v\n", err)
		return 1
	}
	return 0
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "VeloTrust Client CLI")
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  client-cli version [--json]")
	fmt.Fprintln(writer, "  client-cli status [--json]")
	fmt.Fprintln(writer, "  client-cli config validate [--file PATH] [--json]")
	fmt.Fprintln(writer, "  client-cli diagnose [--config PATH] [--json]")
	fmt.Fprintln(writer, "  client-cli keygen --dir PRIVATE_DIR")
	fmt.Fprintln(writer, "  client-cli connect --config PATH [--log-level debug|info|warn|error]")
	fmt.Fprintln(writer, "  client-cli disconnect --config PATH")
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
