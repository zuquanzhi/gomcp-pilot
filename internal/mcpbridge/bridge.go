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

// NewServer 创建一个 MCP 服务器，通过进程管理器将请求转发给上游。
func NewServer(pm *process.Manager) (*server.MCPServer, error) {
	s := server.NewMCPServer(
		"gomcp-pilot",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
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
		// 保留结构化模板；避免设置 RawInputSchema 以防冲突。
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

			// 记录调用历史用于审计
			_ = store.RecordCall(upstreamName, toolName, argsStr, status, errStr, duration)

			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return result, nil
		}

		s.AddTool(mcpTool, handler)
	}

	// 注册资源处理器
	resources, err := pm.ListResources("")
	if err != nil {
		// Log warning but proceed, as resources are optional
		// We can't log easily here as we don't have logger passed in, but we can print or ignore
		// Default to ignoring error and just registering what we have
	}

	for _, r := range resources {
		resource := mcp.Resource{
			URI:         r.Uri,
			Name:        r.Name,
			Description: r.Description,
			MIMEType:    r.MimeType,
		}

		// Explicitly type the handler to get better error messages if signature mismatches
		var handler server.ResourceHandlerFunc = func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			res, err := pm.ReadResource(ctx, req.Params.URI)
			if err != nil {
				return nil, err
			}
			return res.Contents, nil
		}

		s.AddResource(resource, handler)
	}

	return s, nil
}

// ServeStdio 阻塞式通过 stdio 提供 MCP 服务。当 stdin 关闭时退出。
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
