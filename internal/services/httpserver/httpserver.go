package httpserver

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"masquerade-dns/internal/pkg/logger"
)

const httpTimeout = 5 * time.Second

type Config struct {
	Host string `env-required:"true" yaml:"host"`
	Port string `env-required:"true" yaml:"port"`
}

type Service struct {
	config *Config
	logger *zap.SugaredLogger

	server *http.Server
}

func NewService(
	config *Config,
	logger *zap.SugaredLogger,
) *Service {
	return &Service{
		config: config,
		logger: logger,
	}
}

func (s *Service) Start() {
	go func() {
		if err := s.startHTTPServer(); err != nil {
			s.logger.Fatalw("Can't start HTTP server", logger.Error(err))
		}
	}()
}

func (s *Service) Shutdown(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "can't shutdown HTTP server")
	}

	return nil
}

func (s *Service) startHTTPServer() error {
	handler := http.NewServeMux()
	handler.Handle("/metrics", promhttp.Handler())

	s.server = &http.Server{
		Addr:         net.JoinHostPort(s.config.Host, s.config.Port),
		Handler:      handler,
		ReadTimeout:  httpTimeout,
		WriteTimeout: httpTimeout,
		IdleTimeout:  httpTimeout,
	}

	if err := s.server.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.Wrap(err, "can't start HTTP server")
		}
	}

	return nil
}
