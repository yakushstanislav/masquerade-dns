package dnslimiter

import (
	"fmt"
	"github.com/dgraph-io/ristretto"
	"net"
	"time"
)

const (
	cacheNumCounters = 1e6
	cacheMaxCost     = 1 << 28
	cacheBufferItems = 64
)

type Config struct {
	TTL time.Duration `env-required:"true" yaml:"ttl"`
}

type Service struct {
	config *Config

	cache *ristretto.Cache
}

func NewService(config *Config) *Service {
	cache, err := ristretto.NewCache(
		&ristretto.Config{
			NumCounters: cacheNumCounters,
			MaxCost:     cacheMaxCost,
			BufferItems: cacheBufferItems,
		},
	)
	if err != nil {
		panic(err)
	}

	return &Service{
		config: config,
		cache:  cache,
	}
}

func (s *Service) Limit(addr net.IP, source string, maxCount int) bool {
	if maxCount == 0 {
		return false
	}

	key := makeKey(addr, source)

	value, ok := s.cache.Get(key)
	if !ok {
		s.cache.SetWithTTL(key, 0, 1, s.config.TTL)

		return false
	}

	count, ok := value.(int)
	if !ok {
		return false
	}

	if count < maxCount {
		s.cache.SetWithTTL(key, count+1, 1, s.config.TTL)

		return false
	}

	return true
}

func makeKey(addr net.IP, source string) string {
	return fmt.Sprintf("%s:%s", addr.String(), source)
}
