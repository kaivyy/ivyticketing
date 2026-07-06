package metrics

import (
	"net/http"
	"time"

	dto "github.com/prometheus/client_model/go"
)

// Snapshot is a point-in-time view of the war-room metrics, derived from the
// registry. It powers the super-admin dashboard's JSON endpoint so the page
// works without a full Prometheus/Grafana deployment.
type Snapshot struct {
	CapturedAt string `json:"capturedAt"`

	// Gauges (current values).
	ActiveQueueUsers float64 `json:"activeQueueUsers"`
	DBConnsInUse     float64 `json:"dbConnsInUse"`
	DBConnsIdle      float64 `json:"dbConnsIdle"`
	HTTPInFlight     float64 `json:"httpInFlight"`

	// Counters (cumulative totals since start).
	QueueReleased     float64 `json:"queueReleased"`
	CheckoutStarted   float64 `json:"checkoutStarted"`
	CheckoutSucceeded float64 `json:"checkoutSucceeded"`
	CheckoutFailed    float64 `json:"checkoutFailed"`
	PaymentSucceeded  float64 `json:"paymentSucceeded"`
	PaymentFailed     float64 `json:"paymentFailed"`
	RacepackScans     float64 `json:"racepackScans"`

	// Derived rates/health.
	HTTPRequests      float64 `json:"httpRequests"`
	HTTPErrors        float64 `json:"httpErrors"`
	ErrorRate         float64 `json:"errorRate"`         // 5xx / total, 0..1
	CheckoutSuccessRate float64 `json:"checkoutSuccessRate"` // succeeded / (succeeded+failed)
	PaymentSuccessRate  float64 `json:"paymentSuccessRate"`  // succeeded / (succeeded+failed)
	HTTPP95Seconds      float64 `json:"httpP95Seconds"`      // approx p95 across all routes
}

// Snapshot gathers the current metric values from the registry into a Snapshot.
func (m *Metrics) Snapshot() Snapshot {
	s := Snapshot{CapturedAt: time.Now().UTC().Format(time.RFC3339)}
	families, err := m.reg.Gather()
	if err != nil {
		return s
	}

	var durSum, durCount float64
	var durBuckets []*dto.Bucket

	for _, mf := range families {
		name := mf.GetName()
		switch name {
		case "queue_active_users":
			s.ActiveQueueUsers = gaugeValue(mf)
		case "db_connections_in_use":
			s.DBConnsInUse = gaugeValue(mf)
		case "db_connections_idle":
			s.DBConnsIdle = gaugeValue(mf)
		case "http_requests_in_flight":
			s.HTTPInFlight = gaugeValue(mf)
		case "queue_released_total":
			s.QueueReleased = counterValue(mf)
		case "checkout_started_total":
			s.CheckoutStarted = counterValue(mf)
		case "checkout_succeeded_total":
			s.CheckoutSucceeded = counterValue(mf)
		case "checkout_failed_total":
			s.CheckoutFailed = counterValue(mf)
		case "payment_succeeded_total":
			s.PaymentSucceeded = counterValue(mf)
		case "payment_failed_total":
			s.PaymentFailed = counterValue(mf)
		case "racepack_scans_total":
			s.RacepackScans = counterValue(mf)
		case "http_requests_total":
			for _, mm := range mf.GetMetric() {
				v := mm.GetCounter().GetValue()
				s.HTTPRequests += v
				if isServerError(mm) {
					s.HTTPErrors += v
				}
			}
		case "http_request_duration_seconds":
			for _, mm := range mf.GetMetric() {
				h := mm.GetHistogram()
				durSum += h.GetSampleSum()
				durCount += float64(h.GetSampleCount())
				durBuckets = mergeBuckets(durBuckets, h.GetBucket())
			}
		}
	}

	if s.HTTPRequests > 0 {
		s.ErrorRate = s.HTTPErrors / s.HTTPRequests
	}
	if t := s.CheckoutSucceeded + s.CheckoutFailed; t > 0 {
		s.CheckoutSuccessRate = s.CheckoutSucceeded / t
	}
	if t := s.PaymentSucceeded + s.PaymentFailed; t > 0 {
		s.PaymentSuccessRate = s.PaymentSucceeded / t
	}
	s.HTTPP95Seconds = approxQuantile(durBuckets, durCount, 0.95)
	return s
}

func gaugeValue(mf *dto.MetricFamily) float64 {
	ms := mf.GetMetric()
	if len(ms) == 0 {
		return 0
	}
	return ms[0].GetGauge().GetValue()
}

func counterValue(mf *dto.MetricFamily) float64 {
	var total float64
	for _, mm := range mf.GetMetric() {
		total += mm.GetCounter().GetValue()
	}
	return total
}

func isServerError(mm *dto.Metric) bool {
	for _, l := range mm.GetLabel() {
		if l.GetName() == "status" {
			v := l.GetValue()
			return len(v) == 3 && v[0] == '5'
		}
	}
	return false
}

// mergeBuckets sums per-bucket cumulative counts across label sets, keyed by
// upper bound, so the p95 estimate spans all routes.
func mergeBuckets(acc []*dto.Bucket, in []*dto.Bucket) []*dto.Bucket {
	if len(acc) == 0 {
		out := make([]*dto.Bucket, len(in))
		for i, b := range in {
			ub := b.GetUpperBound()
			c := b.GetCumulativeCount()
			out[i] = &dto.Bucket{UpperBound: &ub, CumulativeCount: &c}
		}
		return out
	}
	for i := range acc {
		if i < len(in) {
			c := acc[i].GetCumulativeCount() + in[i].GetCumulativeCount()
			acc[i].CumulativeCount = &c
		}
	}
	return acc
}

// approxQuantile estimates a quantile from cumulative histogram buckets using
// linear interpolation within the target bucket.
func approxQuantile(buckets []*dto.Bucket, count float64, q float64) float64 {
	if count == 0 || len(buckets) == 0 {
		return 0
	}
	target := q * count
	var prevCount float64
	var prevBound float64
	for _, b := range buckets {
		cum := float64(b.GetCumulativeCount())
		if cum >= target {
			ub := b.GetUpperBound()
			if ub > 1e18 { // +Inf bucket
				return prevBound
			}
			span := cum - prevCount
			if span <= 0 {
				return ub
			}
			frac := (target - prevCount) / span
			return prevBound + frac*(ub-prevBound)
		}
		prevCount = cum
		prevBound = b.GetUpperBound()
	}
	return prevBound
}

// SnapshotHandler serves the war-room JSON snapshot.
func (m *Metrics) SnapshotHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, m.Snapshot())
}
