package mcpbridge

import (
	"context"
	"encoding/json"
	"fmt"

	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"gomcp-pilot/internal/process"
	"gomcp-pilot/internal/store"
)

// NewServer builds an MCP server that forwards calls to upstream MCP servers via the process manager.
func NewServer(pm *process.Manager) (*server.MCPServer, error) {
	s := server.NewMCPServer(
		"gomcp-pilot",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	tools, err := pm.ListTools("")
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	for _, t := range tools {
		upstreamName := t.Upstream
		toolName := t.Name
		mcpTool := mcp.NewTool(
			fmt.Sprintf("%s/%s", upstreamName, toolName),
			mcp.WithDescription(t.Description),
		)
		// Preserve structured schema; avoid setting RawInputSchema to prevent conflicts.
		if t.InputSchema != nil {
			if b, err := json.Marshal(t.InputSchema); err == nil {
				_ = json.Unmarshal(b, &mcpTool.InputSchema)
			}
		}

		handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			start := time.Now()
			argsBytes, _ := json.Marshal(req.GetRawArguments())
			argsStr := string(argsBytes)

			result, err := pm.CallTool(ctx, process.CallRequest{
				Upstream:  upstreamName,
				Tool:      toolName,
				Arguments: req.GetRawArguments(),
			})

			duration := time.Since(start)
			status := "success"
			errStr := ""
			if err != nil {
				status = "error"
				errStr = err.Error()
			}

			// Async log to not block response too much, or sync for safety?
			// Sync is safer for audit.
			_ = store.RecordCall(upstreamName, toolName, argsStr, status, errStr, duration)

			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return result, nil
		}

		s.AddTool(mcpTool, handler)
	}

	return s, nil
}

// ServeStdio blocks serving MCP over stdio. The server will exit when stdin closes.
func ServeStdio(ctx context.Context, srv *server.MCPServer) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeStdio(srv)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
