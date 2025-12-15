package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gomcp-pilot/internal/config"
	"gomcp-pilot/internal/mcpbridge"
	"gomcp-pilot/internal/process"
	"gomcp-pilot/internal/server"
)

// Run boots the upstream manager and HTTP server, blocking until context cancellation.
func Run(ctx context.Context, cfg *config.Config) error {
	logger := log.New(os.Stdout, "[gomcp] ", log.LstdFlags|log.Lmicroseconds)

	manager := process.NewManager()
	if err := manager.StartAll(ctx, cfg); err != nil {
		return err
	}
	defer manager.StopAll()

	srv := server.New(cfg, manager, logger)
	return srv.Start(ctx)
}

// RunMCP starts upstreams and serves an MCP server over stdio.
func RunMCP(ctx context.Context, cfg *config.Config) error {
	logger := log.New(os.Stdout, "[gomcp] ", log.LstdFlags|log.Lmicroseconds)

	manager := process.NewManager()
	if err := manager.StartAll(ctx, cfg); err != nil {
		return err
	}
	defer manager.StopAll()

	srv, err := mcpbridge.NewServer(manager)
	if err != nil {
		return err
	}
	logger.Println("stdio MCP server ready (connect with MCP-compatible client)")
	return mcpbridge.ServeStdio(ctx, srv)
}

// WithSignals wraps a context with SIGINT/SIGTERM cancellation.
func WithSignals() (context.Context, context.CancelFunc) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	return ctx, cancel
}
