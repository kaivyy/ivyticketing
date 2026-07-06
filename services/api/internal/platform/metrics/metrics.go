// Package metrics is the platform's Prometheus instrumentation surface. It owns
// a single registry, the standard process/Go collectors, HTTP request metrics,
// and the domain (business) metrics the war-room dashboard reads: queue,
// checkout, payment, and racepack scan activity.
package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics bundles the registry and every collector the application updates.
type Metrics struct {
	reg *prometheus.Registry

	// HTTP
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	httpInflight prometheus.Gauge

	// Domain gauges (point-in-time snapshots set by background samplers).
	activeQueueUsers prometheus.Gauge
	dbConnsInUse     prometheus.Gauge
	dbConnsIdle      prometheus.Gauge

	// Domain counters (event-driven; rate() gives the per-second view).
	queueReleased    prometheus.Counter
	checkoutStarted  prometheus.Counter
	checkoutSucceeded prometheus.Counter
	checkoutFailed   prometheus.Counter
	paymentSucceeded prometheus.Counter
	paymentFailed    prometheus.Counter
	racepackScans    prometheus.Counter

	// Payment gateway webhook processing delay (received → processed).
	webhookDelay prometheus.Histogram
}

// New builds a Metrics with a fresh registry and all collectors registered.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	m := &Metrics{
		reg: reg,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by method, route and status.",
		}, []string{"method", "route", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"method", "route"}),
		httpInflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "In-flight HTTP requests.",
		}),
		activeQueueUsers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "queue_active_users",
			Help: "Users currently active in the virtual waiting room.",
		}),
		dbConnsInUse: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "db_connections_in_use",
			Help: "Postgres pool connections currently in use.",
		}),
		dbConnsIdle: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Idle Postgres pool connections.",
		}),
		queueReleased: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "queue_released_total",
			Help: "Users released from the waiting room into checkout.",
		}),
		checkoutStarted: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "checkout_started_total",
			Help: "Orders created (checkout started).",
		}),
		checkoutSucceeded: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "checkout_succeeded_total",
			Help: "Orders that reached a paid/confirmed state.",
		}),
		checkoutFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "checkout_failed_total",
			Help: "Orders that expired or failed.",
		}),
		paymentSucceeded: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "payment_succeeded_total",
			Help: "Successful gateway payments.",
		}),
		paymentFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "payment_failed_total",
			Help: "Failed/denied gateway payments.",
		}),
		racepackScans: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "racepack_scans_total",
			Help: "Racepack QR scans processed.",
		}),
		webhookDelay: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "gateway_webhook_delay_seconds",
			Help:    "Delay between a gateway webhook arriving and being processed.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		}),
	}

	reg.MustRegister(
		m.httpRequests, m.httpDuration, m.httpInflight,
		m.activeQueueUsers, m.dbConnsInUse, m.dbConnsIdle,
		m.queueReleased, m.checkoutStarted, m.checkoutSucceeded, m.checkoutFailed,
		m.paymentSucceeded, m.paymentFailed, m.racepackScans, m.webhookDelay,
	)
	return m
}

// Registry exposes the underlying registry for the /metrics handler.
func (m *Metrics) Registry() *prometheus.Registry { return m.reg }

// ObserveHTTP records one completed HTTP request.
func (m *Metrics) ObserveHTTP(method, route string, status int, d time.Duration) {
	if route == "" {
		route = "unmatched"
	}
	m.httpRequests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.httpDuration.WithLabelValues(method, route).Observe(d.Seconds())
}

func (m *Metrics) IncInflight() { m.httpInflight.Inc() }
func (m *Metrics) DecInflight() { m.httpInflight.Dec() }

// --- domain gauge setters (called by background samplers) ---

func (m *Metrics) SetActiveQueueUsers(n float64) { m.activeQueueUsers.Set(n) }
func (m *Metrics) SetDBConns(inUse, idle float64) {
	m.dbConnsInUse.Set(inUse)
	m.dbConnsIdle.Set(idle)
}

// --- domain counters (called at business events) ---

func (m *Metrics) IncQueueReleased(n int) {
	if n > 0 {
		m.queueReleased.Add(float64(n))
	}
}
func (m *Metrics) IncCheckoutStarted()   { m.checkoutStarted.Inc() }
func (m *Metrics) IncCheckoutSucceeded() { m.checkoutSucceeded.Inc() }
func (m *Metrics) IncCheckoutFailed()    { m.checkoutFailed.Inc() }
func (m *Metrics) IncPaymentSucceeded()  { m.paymentSucceeded.Inc() }
func (m *Metrics) IncPaymentFailed()     { m.paymentFailed.Inc() }
func (m *Metrics) IncRacepackScans()     { m.racepackScans.Inc() }

// ObserveWebhookDelay records how long a gateway webhook waited before we
// processed it.
func (m *Metrics) ObserveWebhookDelay(d time.Duration) {
	m.webhookDelay.Observe(d.Seconds())
}
