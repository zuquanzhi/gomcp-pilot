package process

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"

	"gomcp-pilot/internal/config"
	"gomcp-pilot/internal/logger"
)

// CallRequest represents a tool invocation against a specific upstream.
type CallRequest struct {
	Upstream  string
	Tool      string
	Arguments any
}

// ToolDescriptor is returned to HTTP clients when listing tools.
type ToolDescriptor struct {
	Upstream    string `json:"upstream"`
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema,omitempty"`
}

// Manager owns the lifecycle of all upstream MCP clients.
type Manager struct {
	mu          sync.RWMutex
	upstreams   map[string]*upstreamClient
	interceptor func(upstream, tool, args string) bool // Returns true if allowed
}

type upstreamClient struct {
	cfg    config.Upstream
	client *client.Client
	tools  []mcp.Tool
}

// NewManager builds an empty manager. Call StartAll before serving traffic.
func NewManager() *Manager {
	return &Manager{
		upstreams: make(map[string]*upstreamClient),
	}
}

func (m *Manager) SetInterceptor(fn func(upstream, tool, args string) bool) {
	m.interceptor = fn
}

// StartAll spawns and initializes every configured upstream.
func (m *Manager) StartAll(ctx context.Context, cfg *config.Config) error {
	for _, ups := range cfg.Upstreams {
		if err := m.startOne(ctx, ups); err != nil {
			return err
		}
	}
	return nil
}

// StopAll tears down every upstream client.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, ups := range m.upstreams {
		_ = ups.client.Close()
		delete(m.upstreams, name)
	}
}

// ListTools aggregates tools across upstreams. If upstreamFilter is non-empty, only
// that upstream is returned.
func (m *Manager) ListTools(upstreamFilter string) ([]ToolDescriptor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []ToolDescriptor
	for name, ups := range m.upstreams {
		if upstreamFilter != "" && name != upstreamFilter {
			continue
		}
		for _, t := range ups.tools {
			title := t.Annotations.Title
			if title == "" {
				title = t.Name
			}
			result = append(result, ToolDescriptor{
				Upstream:    name,
				Name:        t.Name,
				Title:       title,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}
	if upstreamFilter != "" && len(result) == 0 {
		return nil, fmt.Errorf("upstream %s not found", upstreamFilter)
	}
	return result, nil
}

// CallTool forwards a tool invocation to the specified upstream.
func (m *Manager) CallTool(ctx context.Context, req CallRequest) (*mcp.CallToolResult, error) {
	m.mu.RLock()
	ups := m.upstreams[req.Upstream]
	m.mu.RUnlock()
	logger.Global.Info("Processing tool call",
		zap.String("upstream", req.Upstream),
		zap.String("tool", req.Tool))

	if ups == nil {
		return nil, fmt.Errorf("upstream %s not found", req.Upstream)
	}

	callReq := mcp.CallToolRequest{
		Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
		Params: mcp.CallToolParams{
			Name:      req.Tool,
			Arguments: req.Arguments,
		},
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Interception Logic
	argBytes, _ := json.Marshal(req.Arguments)
	argStr := string(argBytes)

	if !ups.cfg.AutoApprove && m.interceptor != nil {
		if !m.interceptor(req.Upstream, req.Tool, argStr) {
			logger.Global.Warn("Tool call intercepted and denied",
				zap.String("upstream", req.Upstream),
				zap.String("tool", req.Tool))
			return nil, fmt.Errorf("operation denied by user")
		}
	}

	logger.Global.Info(fmt.Sprintf(">> Calling MCP: %s/%s %s", req.Upstream, req.Tool, argStr))

	start := time.Now()
	res, err := ups.client.CallTool(ctx, callReq)
	duration := time.Since(start)

	if err != nil {
		logger.Global.Error(fmt.Sprintf("<< Error: %v", err),
			zap.String("upstream", req.Upstream),
			zap.String("tool", req.Tool))
		return nil, err
	}

	// Try to get a string representation of the result content for logging
	var resStr string
	if len(res.Content) > 0 {
		// Just take the first bit of textcontent or image info
		if txt, ok := res.Content[0].(mcp.TextContent); ok {
			resStr = txt.Text
			if len(resStr) > 100 {
				resStr = resStr[:100] + "..."
			}
		} else {
			resStr = "[Binary/Image Content]"
		}
	} else {
		resStr = "[No Content]"
	}

	logger.Global.Info(fmt.Sprintf("<< Result: %s", resStr),
		zap.String("upstream", req.Upstream),
		zap.String("tool", req.Tool),
		zap.Duration("duration", duration))

	return res, nil

}

func (m *Manager) startOne(ctx context.Context, ups config.Upstream) error {
	commandFunc := func(ctx context.Context, cmd string, env []string, args []string) (*exec.Cmd, error) {
		c := exec.CommandContext(ctx, cmd, args...)
		if ups.Workdir != "" {
			c.Dir = ups.Workdir
		}
		c.Env = append(c.Env, env...)
		return c, nil
	}

	stdio := transport.NewStdioWithOptions(
		ups.Command,
		ups.Env,
		ups.Args,
		transport.WithCommandFunc(commandFunc),
	)

	cl := client.NewClient(stdio)

	// Start transport
	if err := cl.Start(ctx); err != nil {
		return fmt.Errorf("start stdio client for %s: %w", ups.Name, err)
	}

	// Initialize handshake
	initReq := mcp.InitializeRequest{
		Request: mcp.Request{Method: string(mcp.MethodInitialize)},
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo: mcp.Implementation{
				Name:    "gomcp-pilot",
				Version: "0.1.0",
			},
		},
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if _, err := cl.Initialize(ctx, initReq); err != nil {
		_ = cl.Close()
		return fmt.Errorf("initialize %s: %w", ups.Name, err)
	}

	tools, err := cl.ListTools(context.Background(), mcp.ListToolsRequest{
		PaginatedRequest: mcp.PaginatedRequest{
			Request: mcp.Request{Method: string(mcp.MethodToolsList)},
			Params:  mcp.PaginatedParams{},
		},
	})
	if err != nil {
		_ = cl.Close()
		return fmt.Errorf("list tools for %s: %w", ups.Name, err)
	}

	m.mu.Lock()
	m.upstreams[ups.Name] = &upstreamClient{
		cfg:    ups,
		client: cl,
		tools:  tools.Tools,
	}
	m.mu.Unlock()
	return nil
}
