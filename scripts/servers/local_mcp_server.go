package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// A tiny standalone MCP server exposing a few utility tools:
// - echo:       return the same text
// - time_now:   current time in RFC3339
// - upper:      uppercase a string
//
// Run directly (for manual test):
//
//	go run ./scripts/local_mcp_server.go
//
// Or reference it from config.yaml as an upstream command:
//
//	command: "go"
//	args: ["run", "./scripts/local_mcp_server.go"]
func main() {
	s := server.NewMCPServer(
		"local-utils",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	echoTool := mcp.NewTool("echo",
		mcp.WithDescription("Echo the input text"),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to echo back")),
	)
	s.AddTool(echoTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text, _ := req.RequireString("text")
		return mcp.NewToolResultText(text), nil
	})

	timeTool := mcp.NewTool("time_now",
		mcp.WithDescription("Return current time in RFC3339"),
	)
	s.AddTool(timeTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(time.Now().Format(time.RFC3339)), nil
	})

	upperTool := mcp.NewTool("upper",
		mcp.WithDescription("Uppercase a string"),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to uppercase")),
	)
	s.AddTool(upperTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text, _ := req.RequireString("text")
		return mcp.NewToolResultText(strings.ToUpper(text)), nil
	})

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
