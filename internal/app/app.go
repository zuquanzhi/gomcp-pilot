package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"

	"gomcp-pilot/internal/config"
	"gomcp-pilot/internal/logger"
	"gomcp-pilot/internal/mcpbridge"
	"gomcp-pilot/internal/process"
	"gomcp-pilot/internal/server"
	"gomcp-pilot/internal/store"
	"gomcp-pilot/internal/tui"
)

// RunTUI boots the upstream manager and HTTP server in TUI mode.
func RunTUI(ctx context.Context, cfg *config.Config) error {
	// 1. Initialize Infrastructure
	if err := logger.InitLogger(); err != nil {
		return err
	}
	if err := store.InitStore(); err != nil {
		return err
	}
	defer store.Close()

	// Redirect standard logger to TUI
	log.SetOutput(&logWriter{})
	stdLogger := log.New(&logWriter{}, "[gomcp] ", log.LstdFlags)

	// 2. Initialize Process Manager with TUI Interceptor
	manager := process.NewManager()
	manager.SetInterceptor(func(upstream, tool, args string) bool {
		// Send request to TUI
		respChan := make(chan bool)
		tui.InterceptChan <- tui.InterceptRequest{
			Upstream:     upstream,
			Tool:         tool,
			Args:         args,
			ResponseChan: respChan,
		}
		// Block waiting for user decision
		logger.Global.Info("Waiting for user approval",
			zap.String("upstream", upstream),
			zap.String("tool", tool))

		allowed := <-respChan

		if allowed {
			logger.Global.Info("Request approved", zap.String("tool", tool))
		} else {
			logger.Global.Warn("Request denied", zap.String("tool", tool))
		}
		return allowed
	})

	if err := manager.StartAll(ctx, cfg); err != nil {
		return err
	}
	defer manager.StopAll()

	// 3. Start HTTP Server
	mcpSrv, err := mcpbridge.NewServer(manager)
	if err != nil {
		return err
	}
	srv := server.New(cfg, manager, stdLogger, mcpSrv)
	go func() {
		if err := srv.Start(ctx); err != nil {
			logger.Global.Error("HTTP server failed to start", zap.String("error", err.Error()))
		}
	}()

	// Tool Fetcher closure
	toolFetcher := func(upstream string) ([]tui.ToolInfo, error) {
		logger.Global.Info("Fetching tools for upstream", zap.String("upstream", upstream))
		tools, err := manager.ListTools(upstream)
		if err != nil {
			logger.Global.Error("Failed to list tools", zap.String("upstream", upstream), zap.Error(err))
			return nil, err
		}

		var infos []tui.ToolInfo
		for _, t := range tools {
			infos = append(infos, tui.ToolInfo{
				Name:        t.Name,
				Description: t.Description,
			})
		}
		return infos, nil
	}

	// 4. Start TUI (Blocks until quit)
	p := tea.NewProgram(tui.InitialModel(cfg, toolFetcher), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

// RunHeadless boots the manager and HTTP server without TUI, blocking until signal.
func RunHeadless(ctx context.Context, cfg *config.Config) error {
	// 1. Initialize Infrastructure
	if err := logger.InitLogger(); err != nil {
		return err
	}
	if err := store.InitStore(); err != nil {
		return err
	}
	defer store.Close()

	// Standard logger
	stdLogger := log.New(os.Stdout, "[gomcp] ", log.LstdFlags)

	// 2. Initialize Process Manager with simple logging interceptor
	manager := process.NewManager()
	manager.SetInterceptor(func(upstream, tool, args string) bool {
		logger.Global.Info("Auto-approving tool call (Headless mode)",
			zap.String("upstream", upstream),
			zap.String("tool", tool))
		return true // Auto-approve in headless mode for now
	})

	if err := manager.StartAll(ctx, cfg); err != nil {
		return err
	}
	defer manager.StopAll()

	// 3. Start HTTP Server
	mcpSrv, err := mcpbridge.NewServer(manager)
	if err != nil {
		return err
	}
	srv := server.New(cfg, manager, stdLogger, mcpSrv)

	// Run server in foreground (blocking) since we don't have TUI to block
	logger.Global.Info("Running in Headless Mode. Press Ctrl+C to stop.")
	return srv.Start(ctx)
}

type logWriter struct{}

func (w *logWriter) Write(p []byte) (n int, err error) {
	// basic bridge to our structured logger channel
	// Remove trailing newline
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	logger.LogChan <- logger.LogEntry{
		Level:     "INFO",
		Message:   msg,
		Timestamp: time.Now(),
	}
	return len(p), nil
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
