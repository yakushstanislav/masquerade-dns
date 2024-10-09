package dnsresolver

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/miekg/dns"
	"go.uber.org/zap"

	"masquerade-dns/internal/metrics"
	"masquerade-dns/internal/pkg/logger"
	"masquerade-dns/internal/pkg/trace"
)

const (
	modeRandom     = "random"
	modeRoundRobin = "round-robin"
)

type nameserver struct {
	Address string `env-required:"true" yaml:"address"`
	Network string `yaml:"network"`
}

type Config struct {
	Timeout     time.Duration `env-required:"true" yaml:"timeout"`
	Mode        string        `env-required:"true" yaml:"mode"`
	Nameservers []nameserver  `env-required:"true" yaml:"nameservers"`
}

type Service struct {
	config  *Config
	metrics *metrics.Metrics
	logger  *zap.SugaredLogger

	index int
}

func NewService(
	config *Config,
	metrics *metrics.Metrics,
	logger *zap.SugaredLogger,
) *Service {
	return &Service{
		config:  config,
		metrics: metrics,
		logger:  logger,
	}
}

func (s *Service) Lookup(ctx context.Context, req *dns.Msg) *dns.Msg {
	traceID := trace.UnpackTraceID(ctx)

	nameserver := s.nameserver()

	client := &dns.Client{
		Net:     nameserver.Network,
		Timeout: s.config.Timeout,
	}

	resp, _, err := client.ExchangeContext(ctx, req, nameserver.Address)
	if err != nil {
		s.logger.Errorw(
			"Can't lookup DNS request",
			logger.TraceID(traceID),
			logger.Error(err),
		)

		s.metrics.IncResolvedDNSRequests(metrics.StatusFailed)

		resp := &dns.Msg{}
		resp.SetRcode(req, dns.RcodeServerFailure)

		return resp
	}

	if resp.Rcode != dns.RcodeSuccess {
		s.logger.Warnw("Invalid DNS response", logger.TraceID(traceID))

		s.metrics.IncResolvedDNSRequests(metrics.StatusFailed)

		return resp
	}

	s.metrics.IncResolvedDNSRequests(metrics.StatusSuccess)

	return resp
}

func (s *Service) nameserver() nameserver {
	switch s.config.Mode {
	case modeRandom:
		return s.config.Nameservers[rand.IntN(len(s.config.Nameservers))]

	case modeRoundRobin:
		if s.index >= len(s.config.Nameservers) {
			s.index = 0
		}

		nameserver := s.config.Nameservers[s.index]

		s.index++

		return nameserver

	default:
		panic("resolver mode is not supported")
	}
}
