package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	managementapp "velotrust/management/app"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "bootstrap" {
		if err := bootstrap(os.Args[2:]); err != nil {
			slog.Error("bootstrap administrator failed", "error", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := migrate(os.Args[2:]); err != nil {
			slog.Error("Management database migration failed", "error", err)
			os.Exit(1)
		}
		return
	}
	if err := run(os.Args[1:]); err != nil {
		slog.Error("Management stopped", "error", err)
		os.Exit(1)
	}
}

func migrate(args []string) error {
	flags := flag.NewFlagSet("management migrate", flag.ContinueOnError)
	configPath := flags.String("config", "", "Management TOML configuration file or directory")
	baseline := flags.Bool("baseline-existing-iam", false, "record the existing IAM schema as the initial migration after verifying it")
	if err := flags.Parse(args); err != nil {
		return err
	}
	conf, err := managementapp.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load Management config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	applied, err := managementapp.Migrate(ctx, conf, *baseline)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		slog.Info("Management database schema is up to date")
		return nil
	}
	slog.Info("Management database migrations applied", "migrations", applied)
	return nil
}

func run(args []string) error {
	flags := flag.NewFlagSet("management", flag.ContinueOnError)
	configPath := flags.String("config", "", "Management TOML configuration file or directory")
	listenAddr := flags.String("listen", "", "Management JRPC listen address (overrides config)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	conf, err := managementapp.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load Management config: %w", err)
	}
	address := *listenAddr
	if address == "" {
		address = conf.HTTP.ListenAddr
	}
	if address == "" {
		port := conf.HTTP.Port
		if port == 0 {
			port = 8080
		}
		address = fmt.Sprintf("127.0.0.1:%d", port)
	}
	if conf.App.Version != "" && version == "dev" {
		version = conf.App.Version
	}
	server, db, closeResources, err := managementapp.Build(conf, address, version)
	if err != nil {
		return fmt.Errorf("initialize Management: %w", err)
	}
	defer db.Close()
	defer closeResources()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	slog.Info("starting Management", "listen_addr", server.ListenAddr(), "jrpc_endpoint", "/jrpc")
	if err := server.Serve(ctx); err != nil {
		return err
	}
	slog.Info("Management stopped")
	return nil
}

func bootstrap(args []string) error {
	flags := flag.NewFlagSet("management bootstrap", flag.ContinueOnError)
	configPath := flags.String("config", "", "Management TOML configuration file or directory")
	username := flags.String("username", "", "Initial administrator username")
	displayName := flags.String("display-name", "", "Initial administrator display name")
	if err := flags.Parse(args); err != nil {
		return err
	}
	password := os.Getenv("VELOTRUST_BOOTSTRAP_PASSWORD")
	_ = os.Unsetenv("VELOTRUST_BOOTSTRAP_PASSWORD")
	if password == "" {
		return fmt.Errorf("set VELOTRUST_BOOTSTRAP_PASSWORD to a protected secret of at least 8 characters")
	}
	conf, err := managementapp.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load Management config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := managementapp.BootstrapAdmin(ctx, conf, *username, *displayName, password); err != nil {
		return err
	}
	slog.Info("initial platform administrator created", "username", *username)
	return nil
}
