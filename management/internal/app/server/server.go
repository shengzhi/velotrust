package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type IServer interface {
	Name() string
	ListenAndServe() error
	ListenAddr() string
	Shutdown(context.Context) error
}

type Server struct {
	servers []IServer
}

func NewServer(servers ...IServer) *Server {
	return &Server{servers: servers}
}

func (as *Server) Add(server IServer) *Server {
	as.servers = append(as.servers, server)
	return as
}

func (as *Server) Start(finalizeFunc func()) {
	for _, server := range as.servers {
		go func(_svr IServer) {
			slog.Info("[server]%s is listening on %s", _svr.Name(), _svr.ListenAddr())
			if err := _svr.ListenAndServe(); err != nil {
				slog.Error(fmt.Sprintf("Server Shutdown error %v", err))
			}
		}(server)
	}

	as.gracefulShutdown(func() error {
		defer func() {
			if finalizeFunc != nil {
				finalizeFunc()
			}
		}()
		var errs []string
		for _, server := range as.servers {
			if err := server.Shutdown(context.Background()); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) == 0 {
			return nil
		}
		return fmt.Errorf("%s", strings.Join(errs, ";"))
	})
}

func (as *Server) gracefulShutdown(f func() error) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	<-quit
	slog.Info("Graceful Shutdown Server ...")
	if err := f(); err != nil {
		slog.Warn(fmt.Sprintf("Server Shutdown error %v", err))
	}
	slog.Info("Server exiting")
}
