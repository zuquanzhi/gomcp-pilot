package process

import (
	"context"
	"fmt"
	"sync"

	"gomcp-pilot/internal/config"
	"gomcp-pilot/internal/mcp"
	"gomcp-pilot/internal/tui"
)

type Manager struct {
	cfg *config.Config
	ui  *tui.UI

	mu    sync.Mutex
	procs map[string]*mcp.Client
}

func NewManager(cfg *config.Config, ui *tui.UI) *Manager {
	return &Manager{
		cfg:   cfg,
		ui:    ui,
		procs: make(map[string]*mcp.Client),
	}
}

func (m *Manager) StartAll(ctx context.Context) error {
	for _, ups := range m.cfg.Upstreams {
		if err := m.startOne(ctx, ups); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) startOne(ctx context.Context, ups config.Upstream) error {
	if ups.Command == "" {
		return fmt.Errorf("upstream %s missing command", ups.Name)
	}

	client, err := mcp.Start(ctx, ups.Name, ups.Command, ups.Args, ups.Workdir)
	if err != nil {
		return fmt.Errorf("start upstream %s: %w", ups.Name, err)
	}

	m.mu.Lock()
	m.procs[ups.Name] = client
	m.mu.Unlock()

	m.ui.Log(fmt.Sprintf("[upstream:%s] started pid=%d", ups.Name, client.PID()))
	return nil
}

// StopAll 尽最大努力终止所有子进程。
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, cmd := range m.procs {
		if cmd == nil {
			continue
		}
		cmd.Stop()
		m.ui.Log(fmt.Sprintf("[upstream:%s] stopped", name))
	}
}

// Call 向指定 upstream 发送 payload，并返回响应。
func (m *Manager) Call(ctx context.Context, upstream string, payload any) (string, error) {
	m.mu.Lock()
	client := m.procs[upstream]
	m.mu.Unlock()
	if client == nil {
		return "", fmt.Errorf("upstream %s not found", upstream)
	}
	return client.Call(ctx, payload)
}
