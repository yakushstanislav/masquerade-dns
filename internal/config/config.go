package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/pkg/errors"

	"masquerade-dns/internal/pkg/logger"
	"masquerade-dns/internal/services/dnslimiter"
	"masquerade-dns/internal/services/dnsresolver"
	"masquerade-dns/internal/services/dnsserver"
	"masquerade-dns/internal/services/dnsswitcher"
	"masquerade-dns/internal/services/httpserver"
)

type Config struct {
	Logger      logger.Config      `yaml:"logger"`
	HTTPServer  httpserver.Config  `yaml:"http"`
	DNSServer   dnsserver.Config   `yaml:"dns"`
	DNSSwitcher dnsswitcher.Config `yaml:"switcher"`
	DNSLimiter  dnslimiter.Config  `yaml:"limiter"`
	DNSResolver dnsresolver.Config `yaml:"resolver"`
}

func ParseFile(path string) (*Config, error) {
	var config Config

	if err := cleanenv.ReadConfig(path, &config); err != nil {
		return nil, errors.Wrap(err, "can't parse config")
	}

	return &config, nil
}
