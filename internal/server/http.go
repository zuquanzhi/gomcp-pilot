package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"gomcp-pilot/internal/config"
	"gomcp-pilot/internal/interceptor"
	"gomcp-pilot/internal/process"
	"gomcp-pilot/internal/tui"
)

type Server struct {
	cfg         *config.Config
	pm          *process.Manager
	interceptor *interceptor.Interceptor
	ui          *tui.UI
}

func New(cfg *config.Config, pm *process.Manager, ic *interceptor.Interceptor, ui *tui.UI) *Server {
	return &Server{
		cfg:         cfg,
		pm:          pm,
		interceptor: ic,
		ui:          ui,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/tools/list", s.handleList)
	mux.HandleFunc("/tools/call", s.handleCall)
	mux.HandleFunc("/events", s.handleEvents)

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: s.authMiddleware(mux),
	}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	s.ui.Status(fmt.Sprintf("listening on %s", addr))
	s.ui.Log(fmt.Sprintf("[http] listening on %s", addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	type tool struct {
		Name        string `json:"name"`
		AutoApprove bool   `json:"auto_approve"`
	}
	resp := make([]tool, 0, len(s.cfg.Upstreams))
	for _, ups := range s.cfg.Upstreams {
		resp = append(resp, tool{Name: ups.Name, AutoApprove: ups.AutoApprove})
	}
	writeJSON(w, resp)
}

func (s *Server) handleCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var call interceptor.Call
	if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	reqID := s.ui.BeginRequest(call.Tool, call.Action, call.Target)
	decision := s.interceptor.Evaluate(r.Context(), call)
	if !decision.Allowed {
		s.ui.ResolveRequest(reqID, "blocked", decision.Reason)
		http.Error(w, decision.Reason, http.StatusForbidden)
		return
	}
	s.ui.ResolveRequest(reqID, "allowed", decision.Reason)

	resp, err := s.pm.Call(r.Context(), call.Tool, call)
	if err != nil {
		s.ui.ResolveRequest(reqID, "error", err.Error())
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		return
	}

	s.ui.ResolveRequest(reqID, "done", "ok")
	writeJSON(w, map[string]any{
		"status":   "accepted",
		"reason":   decision.Reason,
		"response": resp,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	fmt.Fprintf(w, "data: %s\n\n", "gomcp-pilot online")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, "data: %s\n\n", "ping")
			flusher.Flush()
		}
	}
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	// 仅做轻量校验，防范 CSRF。
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AuthToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		token := r.Header.Get("Authorization")
		expected := "Bearer " + s.cfg.AuthToken
		if token != expected {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
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
