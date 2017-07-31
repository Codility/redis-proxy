package rproxy

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	statDurationsHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "rproxy_request_durations_nanoseconds",
		Help: "Request durations (excluding client communication, including calls to Redis)",
		Buckets: []float64{
			float64(50 * time.Microsecond),
			float64(100 * time.Microsecond),
			float64(250 * time.Microsecond),
			float64(500 * time.Microsecond),
			float64(1000 * time.Microsecond),
			float64(2500 * time.Microsecond),
			float64(5000 * time.Microsecond),
			float64(10000 * time.Microsecond),
		},
	})
	statDelaysHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "rproxy_request_delays_nanoseconds",
		Help: "Request delays introduced by the proxy (excluding client communication, excluding calls to Redis)",
		Buckets: []float64{
			float64(1 * time.Microsecond),
			float64(5 * time.Microsecond),
			float64(10 * time.Microsecond),
			float64(25 * time.Microsecond),
			float64(50 * time.Microsecond),
			float64(100 * time.Microsecond),
			float64(250 * time.Microsecond),
			float64(500 * time.Microsecond),
		},
	})
	statActiveRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "rproxy_active_requests",
		Help: "Number of active requests (those currently executing a call to Redis)",
	})
)

func init() {
	prometheus.MustRegister(
		statDurationsHistogram,
		statDelaysHistogram,
		statActiveRequests,
	)
}

func statRecordRequest(duration, redisDuration time.Duration) {
	statDurationsHistogram.Observe(float64(duration))
	statDelaysHistogram.Observe(float64(duration - redisDuration))
}

func statRecordProxyState(activeRequests int) {
	statActiveRequests.Set(float64(activeRequests))
}
