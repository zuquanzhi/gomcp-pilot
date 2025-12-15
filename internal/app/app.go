package app

import (
	"context"
	"fmt"

	"gomcp-pilot/internal/config"
	"gomcp-pilot/internal/interceptor"
	"gomcp-pilot/internal/process"
	"gomcp-pilot/internal/server"
	"gomcp-pilot/internal/tui"
)

// App 负责组装各核心组件。
type App struct {
	cfg         *config.Config
	pm          *process.Manager
	interceptor *interceptor.Interceptor
	server      *server.Server
	ui          *tui.UI
}

func New(cfg *config.Config) (*App, error) {
	ui := tui.New()
	pm := process.NewManager(cfg, ui)
	ic := interceptor.New(cfg, ui)
	srv := server.New(cfg, pm, ic, ui)

	return &App{
		cfg:         cfg,
		pm:          pm,
		interceptor: ic,
		server:      srv,
		ui:          ui,
	}, nil
}

// Run 按顺序启动进程管理、UI 和 HTTP 服务。
func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// TUI 可以在独立 goroutine 中运行，方便接收事件。
	go a.ui.Run(ctx)

	if err := a.pm.StartAll(ctx); err != nil {
		return fmt.Errorf("start upstreams: %w", err)
	}
	defer a.pm.StopAll()

	if err := a.server.Start(ctx); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	return nil
}
