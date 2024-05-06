package main

import (
	"context"
	"flag"
	"log"
	"masquerade-dns/internal/metrics"
	"masquerade-dns/internal/services/dnslimiter"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"masquerade-dns/internal/config"
	"masquerade-dns/internal/pkg/logger"
	"masquerade-dns/internal/services/dnsresolver"
	"masquerade-dns/internal/services/dnsserver"
	"masquerade-dns/internal/services/dnsswitcher"
	"masquerade-dns/internal/services/httpserver"
)

const (
	shutdownTimeout = 5 * time.Second
)

type Flags struct {
	configPath string
}

func parseFlags() *Flags {
	var flags Flags

	flag.StringVar(&flags.configPath, "path", "configs/config.yml", "Config path")
	flag.Parse()

	return &flags
}

func main() {
	flags := parseFlags()

	cfg, err := config.ParseFile(flags.configPath)
	if err != nil {
		log.Fatalln(err)
	}

	logger, err := logger.NewLogger(&cfg.Logger)
	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		_ = logger.Sync()
	}()

	logger.Infow("Start...")

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	metrics := metrics.NewMetrics()

	httpServer := httpserver.NewService(&cfg.HTTPServer, logger)
	httpServer.Start()

	dnsLimiter := dnslimiter.NewService(&cfg.DNSLimiter)
	dnsResolver := dnsresolver.NewService(&cfg.DNSResolver, metrics, logger)

	dnsSwitcher := dnsswitcher.NewService(
		&cfg.DNSSwitcher,
		metrics,
		dnsLimiter,
		logger,
	)

	dnsServer := dnsserver.NewService(
		&cfg.DNSServer,
		metrics,
		dnsResolver,
		dnsSwitcher,
		logger,
	)

	dnsServer.Start()

	<-ctx.Done()

	logger.Infow("Stop...")

	if err := dnsServer.Shutdown(); err != nil {
		logger.Errorw("Can't stop DNS server", zap.Error(err))
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Errorw("Can't stop HTTP server", zap.Error(err))
	}
}
