package metrics

import (
	"net"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

const namespace = "masquerade"

type Metrics struct {
	totalDNSRequests    *prometheus.CounterVec
	resolvedDNSRequests *prometheus.CounterVec
	switchedDNSRequests *prometheus.CounterVec

	limitedDNSRequests prometheus.Counter

	durationDNSRequests prometheus.Histogram
}

func NewMetrics() *Metrics {
	m := &Metrics{}

	m.init()

	return m
}

func (m *Metrics) init() {
	m.totalDNSRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "dns_requests_processed_total",
			Help:      "Total number of processed DNS requests.",
			Namespace: namespace,
		},
		[]string{"remote_ip"},
	)

	m.resolvedDNSRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "dns_requests_resolved_total",
			Help:      "Total number of resolved DNS requests.",
			Namespace: namespace,
		},
		[]string{"status"},
	)

	m.switchedDNSRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "dns_requests_switched_total",
			Help:      "Total number of switched DNS requests.",
			Namespace: namespace,
		},
		[]string{"remote_ip"},
	)

	m.limitedDNSRequests = promauto.NewCounter(
		prometheus.CounterOpts{
			Name:      "dns_requests_limited_total",
			Help:      "Total number of limited DNS requests.",
			Namespace: namespace,
		},
	)

	m.durationDNSRequests = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:      "dns_requests_duration",
			Help:      "Histogram of DNS request durations.",
			Namespace: namespace,
		},
	)
}

func (m *Metrics) IncTotalDNSRequests(addr net.IP) {
	m.totalDNSRequests.WithLabelValues(addr.String()).Inc()
}

func (m *Metrics) IncResolvedDNSRequests(status string) {
	m.resolvedDNSRequests.WithLabelValues(status).Inc()
}

func (m *Metrics) IncSwitchedDNSRequests(addr net.IP) {
	m.switchedDNSRequests.WithLabelValues(addr.String()).Inc()
}

func (m *Metrics) IncLimitedDNSRequests() {
	m.limitedDNSRequests.Inc()
}

func (m *Metrics) NewDNSRequestsTimer() *prometheus.Timer {
	return prometheus.NewTimer(m.durationDNSRequests)
}
