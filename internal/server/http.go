package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"gomcp-pilot/internal/config"
	"gomcp-pilot/internal/process"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Server exposes HTTP endpoints for MCP tool discovery and invocation.
type Server struct {
	cfg       *config.Config
	manager   *process.Manager
	logger    *log.Logger
	mcpServer *mcpserver.MCPServer
}

func New(cfg *config.Config, manager *process.Manager, logger *log.Logger, mcpServer *mcpserver.MCPServer) *Server {
	return &Server{cfg: cfg, manager: manager, logger: logger, mcpServer: mcpServer}
}

// Start runs the HTTP server until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/tools/list", s.handleListTools)
	mux.HandleFunc("/tools/call", s.handleCallTool)

	// Add SSE support
	if s.mcpServer != nil {
		// The endpoint URL that clients should post messages to.
		// Constructing it based on config port.
		endpointURL := fmt.Sprintf("http://localhost:%d/mcp/message", s.cfg.Port)
		sseServer := mcpserver.NewSSEServer(
			s.mcpServer,
			mcpserver.WithMessageEndpoint(endpointURL),
		)

		mux.Handle("/sse", sseServer.SSEHandler())
		mux.Handle("/mcp/message", sseServer.MessageHandler())
		s.logger.Printf("SSE endpoint mounted at /sse (message endpoint: %s)", endpointURL)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: s.authMiddleware(mux),
	}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	s.logger.Printf("HTTP listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	upstream := r.URL.Query().Get("upstream")
	tools, err := s.manager.ListTools(upstream)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, tools)
}

type callPayload struct {
	Upstream  string      `json:"upstream"`
	Tool      string      `json:"tool"`
	Arguments interface{} `json:"arguments,omitempty"`
}

func (s *Server) handleCallTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload callPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}
	if payload.Upstream == "" || payload.Tool == "" {
		http.Error(w, "upstream and tool are required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	result, err := s.manager.CallTool(ctx, process.CallRequest{
		Upstream:  payload.Upstream,
		Tool:      payload.Tool,
		Arguments: payload.Arguments,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("call failed: %v", err), http.StatusBadGateway)
		return
	}

	writeJSON(w, map[string]any{
		"upstream": payload.Upstream,
		"tool":     payload.Tool,
		"result":   result,
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	if s.cfg.AuthToken == "" {
		return next
	}
	expected := "Bearer " + s.cfg.AuthToken
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expected {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}
