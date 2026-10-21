package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"velotrust/hub/app"
	"velotrust/hub/config"
)

func main() {
	configPath := flag.String("config", "./hub/configs/hub.example.json", "path to HUB JSON config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load HUB config", "error", err)
		os.Exit(1)
	}
	server, cleanup, err := app.Build(cfg)
	if err != nil {
		slog.Error("initialize HUB", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("starting HUB", "instance_id", server.InstanceID(), "listen_addr", server.ListenAddr())
	if err := server.Serve(ctx); err != nil {
		slog.Error("HUB stopped", "error", err)
		os.Exit(1)
	}
	slog.Info("HUB stopped", "instance_id", server.InstanceID())
}
