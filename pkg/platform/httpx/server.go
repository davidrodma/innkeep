package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Server é um servidor HTTP com timeouts seguros e desligamento gracioso.
type Server struct {
	server *http.Server
	logger *slog.Logger
}

// NewServer cria o servidor. Os timeouts não são opcionais: um servidor Go sem
// timeouts mantém conexões lentas abertas para sempre (ataque "slowloris").
func NewServer(addr string, handler http.Handler, logger *slog.Logger) *Server {
	return &Server{
		logger: logger,
		server: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
			MaxHeaderBytes:    64 << 10,
		},
	}
}

// Run atende requisições até o contexto ser cancelado e, então, espera as
// requisições em andamento terminarem (desligamento gracioso). Em Kubernetes
// isso evita erros durante um deploy: o pod recebe SIGTERM, para de aceitar
// conexões novas e conclui as que já estavam em curso.
func (s *Server) Run(ctx context.Context) error {
	serveErr := make(chan error, 1)
	go func() {
		s.logger.Info("servidor HTTP ouvindo", slog.String("addr", s.server.Addr))
		serveErr <- s.server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("httpx: servidor encerrou: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("httpx: desligamento gracioso: %w", err)
		}
		return nil
	}
}
