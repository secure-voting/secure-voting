package httpserver

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Server wraps http.Server to provide graceful shutdown.
type Server struct {
	httpServer *http.Server
}

// New builds an HTTP server.
func New(addr string, handler http.Handler) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

// Run starts serving HTTP requests.
func (s *Server) Run() error {
	err := s.httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown stops the server gracefully.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
