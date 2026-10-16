package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	management "velotrust/management/runtime"
)

var version = "dev"

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:8080", "Management JRPC listen address")
	flag.Parse()

	server, err := management.New(management.Config{ListenAddr: *listenAddr, Version: version})
	if err != nil {
		slog.Error("initialize Management", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("starting Management", "listen_addr", server.ListenAddr(), "jrpc_endpoint", "/jrpc")
	if err := server.Serve(ctx); err != nil {
		slog.Error("Management stopped", "error", err)
		os.Exit(1)
	}
	slog.Info("Management stopped")
}
