package dnsswitcher

import (
	"context"
	"net"
	"regexp"
	"strings"

	"github.com/miekg/dns"
	"go.uber.org/zap"

	"masquerade-dns/internal/metrics"
	"masquerade-dns/internal/pkg/logger"
	"masquerade-dns/internal/pkg/trace"
)

type dnsLimiter interface {
	Limit(addr net.IP, source string, maxCount int) bool
}

type dnsHTTPSAnswer struct {
	Priority uint16   `yaml:"priority"`
	Target   string   `env-required:"true" yaml:"target"`
	ALPN     []string `yaml:"alpn"`
	IPv4Hint []net.IP `yaml:"ipv4hint"`
	IPv6Hint []net.IP `yaml:"ipv6hint"`
}

type dnsAnswer struct {
	A     net.IP          `yaml:"a"`
	AAAA  net.IP          `yaml:"aaaa"`
	CNAME string          `yaml:"cname"`
	HTTPS *dnsHTTPSAnswer `yaml:"https"`
}

type switchConfig struct {
	Source      string     `env-required:"true" yaml:"source"`
	Destination string     `yaml:"destination"`
	Answer      *dnsAnswer `yaml:"answer"`
	MaxCount    int        `env-required:"true" yaml:"maxCount"`
	TTL         uint32     `env-required:"true" yaml:"ttl"`
}

type Config struct {
	Settings []switchConfig `yaml:"settings"`
}

type Service struct {
	config  *Config
	metrics *metrics.Metrics
	limiter dnsLimiter
	logger  *zap.SugaredLogger
}

func NewService(
	config *Config,
	metrics *metrics.Metrics,
	limiter dnsLimiter,
	logger *zap.SugaredLogger,
) *Service {
	return &Service{
		config:  config,
		metrics: metrics,
		limiter: limiter,
		logger:  logger,
	}
}

func (s *Service) Switch(ctx context.Context, addr net.IP, req *dns.Msg) (*dns.Msg, bool) {
	traceID := trace.UnpackTraceID(ctx)

	if len(req.Question) == 0 {
		s.logger.Warnw("Handle DNS request without question", logger.TraceID(traceID))

		return nil, false
	}

	question := req.Question[0]

	for _, config := range s.config.Settings {
		if strings.HasPrefix(config.Source, "/") && strings.HasSuffix(config.Source, "/") {
			source := strings.Trim(config.Source, "/")

			r, err := regexp.Compile(source)
			if err != nil {
				s.logger.Warnw(
					"Can't compile regular expression",
					logger.TraceID(traceID),
					logger.Error(err),
				)

				continue
			}

			if !r.MatchString(question.Name) {
				continue
			}
		} else if dns.Fqdn(config.Source) != question.Name {
			continue
		}

		if s.limiter.Limit(addr, config.Source, config.MaxCount) {
			s.logger.Infow("Limit DNS request", logger.TraceID(traceID))

			s.metrics.IncLimitedDNSRequests()

			return nil, false
		}

		s.logger.Infow("Switch DNS request", logger.TraceID(traceID))

		s.metrics.IncSwitchedDNSRequests(addr)

		resp := &dns.Msg{}
		resp.SetReply(req)

		var answer []dns.RR

		if config.Destination != "" {
			answer = parseDNSAnswer(question.Name, config.Destination, config.TTL)
		} else {
			answer = makeDNSAnswer(question.Name, question.Qtype, config.Answer, config.TTL)
		}

		resp.Answer = append(resp.Answer, answer...)

		return resp, true
	}

	return nil, false
}

func parseDNSAnswer(name, destination string, ttl uint32) []dns.RR {
	var answers []dns.RR

	if addr := net.ParseIP(destination); addr != nil {
		switch len(addr) {
		case net.IPv4len:
			answers = append(answers,
				makeDNSAnswerA(name, addr, ttl),
			)

		case net.IPv6len:
			answers = append(answers,
				makeDNSAnswerAAAA(name, addr, ttl),
			)
		}
	} else {
		answers = append(answers,
			makeDNSAnswerCNAME(name, destination, ttl),
		)
	}

	return answers
}

func makeDNSAnswer(name string, qtype uint16, config *dnsAnswer, ttl uint32) []dns.RR {
	var answers []dns.RR

	switch qtype {
	case dns.TypeA:
		if config.A != nil {
			answers = append(answers,
				makeDNSAnswerA(name, config.A, ttl),
			)
		}

	case dns.TypeAAAA:
		if config.AAAA != nil {
			answers = append(answers,
				makeDNSAnswerAAAA(name, config.AAAA, ttl),
			)
		}

	case dns.TypeCNAME:
		if config.CNAME != "" {
			answers = append(answers,
				makeDNSAnswerCNAME(name, config.CNAME, ttl),
			)
		}
	case dns.TypeHTTPS:
		if config.HTTPS != nil {
			answers = append(answers,
				makeDNSAnswerHTTPS(name, config.HTTPS, ttl),
			)
		}
	}

	return answers
}

func makeDNSAnswerA(name string, addr net.IP, ttl uint32) *dns.A {
	const rdLength = 4

	return &dns.A{
		Hdr: dns.RR_Header{
			Name:     dns.Fqdn(name),
			Rrtype:   dns.TypeA,
			Class:    dns.ClassINET,
			Ttl:      ttl,
			Rdlength: rdLength,
		},
		A: addr,
	}
}

func makeDNSAnswerAAAA(name string, addr net.IP, ttl uint32) *dns.AAAA {
	const rdLength = 16

	return &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:     dns.Fqdn(name),
			Rrtype:   dns.TypeAAAA,
			Class:    dns.ClassINET,
			Ttl:      ttl,
			Rdlength: rdLength,
		},
		AAAA: addr,
	}
}

func makeDNSAnswerCNAME(name string, target string, ttl uint32) *dns.CNAME {
	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Target: dns.Fqdn(target),
	}
}

func makeDNSAnswerHTTPS(name string, config *dnsHTTPSAnswer, ttl uint32) *dns.HTTPS {
	answer := &dns.HTTPS{
		SVCB: dns.SVCB{
			Hdr: dns.RR_Header{
				Name:   dns.Fqdn(name),
				Rrtype: dns.TypeHTTPS,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			Priority: config.Priority,
			Target:   dns.Fqdn(config.Target),
		},
	}

	if config.ALPN != nil {
		answer.Value = append(answer.Value,
			&dns.SVCBAlpn{Alpn: config.ALPN},
		)
	}

	if config.IPv4Hint != nil {
		answer.Value = append(answer.Value,
			&dns.SVCBIPv4Hint{Hint: config.IPv4Hint},
		)
	}

	if config.IPv6Hint != nil {
		answer.Value = append(answer.Value,
			&dns.SVCBIPv6Hint{Hint: config.IPv6Hint},
		)
	}

	return answer
}
