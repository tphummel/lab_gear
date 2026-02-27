package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lab_gear_http_requests_total",
			Help: "Total number of HTTP requests by method, route, and status code.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lab_gear_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds by method and route.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "lab_gear_http_requests_in_flight",
		Help: "Current number of HTTP requests being processed.",
	})
)

// MachineDB is the subset of db.DB needed to collect machine metrics.
type MachineDB interface {
	CountByKind() (map[string]int, error)
}

// machineCollector is a custom Prometheus collector that queries the database
// on each scrape to report machine counts broken down by kind.
type machineCollector struct {
	db          MachineDB
	machinesDesc *prometheus.Desc
}

func (c *machineCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.machinesDesc
}

func (c *machineCollector) Collect(ch chan<- prometheus.Metric) {
	counts, err := c.db.CountByKind()
	if err != nil {
		ch <- prometheus.NewInvalidMetric(c.machinesDesc, err)
		return
	}
	for kind, n := range counts {
		ch <- prometheus.MustNewConstMetric(
			c.machinesDesc,
			prometheus.GaugeValue,
			float64(n),
			kind,
		)
	}
}

// Register registers all metrics with the default Prometheus registry.
// Call once at startup after the database is initialised.
func Register(db MachineDB) {
	prometheus.MustRegister(
		// Standard Go runtime and process metrics
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),

		// HTTP service metrics
		httpRequestsTotal,
		httpRequestDuration,
		httpRequestsInFlight,

		// Application metrics
		&machineCollector{
			db: db,
			machinesDesc: prometheus.NewDesc(
				"lab_gear_machines_total",
				"Number of machines managed, partitioned by kind.",
				[]string{"kind"},
				nil,
			),
		},
	)
}

// Handler returns the Prometheus HTTP handler for the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}

// responseWriter wraps http.ResponseWriter to capture the response status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Middleware wraps an http.Handler to record HTTP metrics.
// pattern should be the route pattern string (e.g. "/api/v1/machines/{id}")
// so the path label has bounded cardinality.
func Middleware(pattern string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		httpRequestsInFlight.Inc()

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			httpRequestsInFlight.Dec()
			status := strconv.Itoa(rw.status)
			httpRequestsTotal.WithLabelValues(r.Method, pattern, status).Inc()
			httpRequestDuration.WithLabelValues(r.Method, pattern).Observe(time.Since(start).Seconds())
		}()

		next.ServeHTTP(rw, r)
	})
}
