package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"time"
)

type Server struct {
	http  *http.Server
	ready atomic.Bool
}

func New(address string, applicationService http.Handler) *Server {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.readiness)
	if applicationService != nil {
		mux.Handle("/transactions/", applicationService)
		mux.Handle("/_matrix/app/", applicationService)
	}
	s.http = &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return s
}

func (s *Server) SetReady(ready bool) { s.ready.Store(ready) }

func (s *Server) ListenAndServe() error {
	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, "ok")
}

func (s *Server) readiness(w http.ResponseWriter, _ *http.Request) {
	if !s.ready.Load() {
		respond(w, http.StatusServiceUnavailable, "not ready")
		return
	}
	respond(w, http.StatusOK, "ready")
}

func respond(w http.ResponseWriter, status int, state string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": state})
}
