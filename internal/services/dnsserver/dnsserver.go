package dnsserver

import (
	"context"
	"masquerade-dns/internal/metrics"
	"net"
	"strconv"
	"time"

	"masquerade-dns/internal/pkg/logger"
	"masquerade-dns/internal/pkg/trace"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const handlerPattern = "."

type dnsResolver interface {
	Lookup(ctx context.Context, req *dns.Msg) *dns.Msg
}

type dnsSwitcher interface {
	Switch(ctx context.Context, addr net.IP, req *dns.Msg) (*dns.Msg, bool)
}

type Config struct {
	Host    string        `env-required:"true" yaml:"host"`
	Port    string        `env-required:"true" yaml:"port"`
	Timeout time.Duration `env-required:"true" yaml:"timeout"`
}

type Service struct {
	config   *Config
	metrics  *metrics.Metrics
	resolver dnsResolver
	switcher dnsSwitcher
	logger   *zap.SugaredLogger

	tcpServer *dns.Server
	udpServer *dns.Server
}

func NewService(
	config *Config,
	metrics *metrics.Metrics,
	resolver dnsResolver,
	switcher dnsSwitcher,
	logger *zap.SugaredLogger,
) *Service {
	return &Service{
		config:   config,
		metrics:  metrics,
		resolver: resolver,
		switcher: switcher,
		logger:   logger,
	}
}

func (s *Service) Start() {
	go func() {
		if err := s.startTCPServer(); err != nil {
			s.logger.Fatalw("Can't start TCP server", logger.Error(err))
		}
	}()

	go func() {
		if err := s.startUDPServer(); err != nil {
			s.logger.Fatalw("Can't start UDP server", logger.Error(err))
		}
	}()
}

func (s *Service) Shutdown() error {
	if err := s.tcpServer.Shutdown(); err != nil {
		return errors.Wrap(err, "can't shutdown TCP server")
	}

	if err := s.udpServer.Shutdown(); err != nil {
		return errors.Wrap(err, "can't shutdown UDP server")
	}

	return nil
}

func (s *Service) startTCPServer() error {
	handler := dns.NewServeMux()
	handler.HandleFunc(handlerPattern, s.handler)

	s.tcpServer = &dns.Server{
		Addr:         net.JoinHostPort(s.config.Host, s.config.Port),
		Net:          "tcp",
		Handler:      handler,
		ReadTimeout:  s.config.Timeout,
		WriteTimeout: s.config.Timeout,
	}

	if err := s.tcpServer.ListenAndServe(); err != nil {
		return errors.Wrap(err, "can't start TCP server")
	}

	return nil
}

func (s *Service) startUDPServer() error {
	handler := dns.NewServeMux()
	handler.HandleFunc(handlerPattern, s.handler)

	s.udpServer = &dns.Server{
		Addr:         net.JoinHostPort(s.config.Host, s.config.Port),
		Net:          "udp",
		Handler:      handler,
		ReadTimeout:  s.config.Timeout,
		WriteTimeout: s.config.Timeout,
	}

	if err := s.udpServer.ListenAndServe(); err != nil {
		return errors.Wrap(err, "can't start UDP server")
	}

	return nil
}

func (s *Service) handler(w dns.ResponseWriter, req *dns.Msg) {
	timer := s.metrics.NewDNSRequestsTimer()
	defer timer.ObserveDuration()

	traceID := trace.NewTraceID()

	ctx := trace.PackTraceID(context.Background(), traceID)

	defer func() {
		if err := w.Close(); err != nil {
			s.logger.Errorw(
				"Can't send DNS response",
				logger.TraceID(traceID),
				logger.Error(err),
			)
		}
	}()

	addr := parseIPAddr(w.RemoteAddr())

	s.logger.Infow(
		"Handle DNS request",
		logger.TraceID(traceID),
		"from", addr,
		"question", formatDNSQuestion(req.Question),
	)

	s.metrics.IncTotalDNSRequests(addr)

	if resp, ok := s.switcher.Switch(ctx, addr, req); ok {
		s.sendResponse(ctx, w, resp)

		return
	}

	resp := s.resolver.Lookup(ctx, req)

	s.sendResponse(ctx, w, resp)
}

func (s *Service) sendResponse(ctx context.Context, w dns.ResponseWriter, resp *dns.Msg) {
	traceID := trace.UnpackTraceID(ctx)

	s.logger.Infow(
		"Send DNS response",
		logger.TraceID(traceID),
		"answer", formatDNSAnswer(resp.Answer),
	)

	if err := w.WriteMsg(resp); err != nil {
		s.logger.Errorw(
			"Can't send DNS response",
			logger.TraceID(traceID),
			logger.Error(err),
		)
	}
}

func formatDNSQuestion(questions []dns.Question) map[string]string {
	names := make(map[string]string, len(questions))

	for _, question := range questions {
		names[formatDNSQType(question.Qtype)] = question.Name
	}

	return names
}

func formatDNSAnswer(answers []dns.RR) []string {
	names := make([]string, 0, len(answers))

	for _, answer := range answers {
		var name string

		switch t := answer.(type) {
		case *dns.A:
			name = t.A.String()

		case *dns.AAAA:
			name = t.AAAA.String()

		case *dns.CNAME:
			name = t.Target

		case *dns.HTTPS:
			name = t.Target

		default:
			name = answer.String()
		}

		names = append(names, name)
	}

	return names
}

func formatDNSQType(qtype uint16) string {
	switch qtype {
	case dns.TypeA:
		return "A"

	case dns.TypeAAAA:
		return "AAAA"

	case dns.TypeCNAME:
		return "CNAME"

	case dns.TypeSOA:
		return "SOA"

	case dns.TypeHTTPS:
		return "HTTPS"
	}

	return strconv.Itoa(int(qtype))
}

func parseIPAddr(addr net.Addr) net.IP {
	switch t := addr.(type) {
	case *net.TCPAddr:
		return t.IP

	case *net.UDPAddr:
		return t.IP

	default:
		panic("address type is not supported")
	}
}
