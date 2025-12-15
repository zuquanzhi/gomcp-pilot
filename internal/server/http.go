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
)

// Server exposes HTTP endpoints for MCP tool discovery and invocation.
type Server struct {
	cfg     *config.Config
	manager *process.Manager
	logger  *log.Logger
}

func New(cfg *config.Config, manager *process.Manager, logger *log.Logger) *Server {
	return &Server{cfg: cfg, manager: manager, logger: logger}
}

// Start runs the HTTP server until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/tools/list", s.handleListTools)
	mux.HandleFunc("/tools/call", s.handleCallTool)

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
