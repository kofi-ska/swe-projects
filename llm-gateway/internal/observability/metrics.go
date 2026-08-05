package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	Requests                 *prometheus.CounterVec
	RequestDuration          *prometheus.HistogramVec
	TTFT                     prometheus.Histogram
	InterToken               prometheus.Histogram
	Tokens                   prometheus.Counter
	QueueRejections          prometheus.Counter
	Cancelled                prometheus.Counter
	TimedOut                 prometheus.Counter
	QueueDepthGauge          prometheus.Gauge
	QueueCapacity            prometheus.Gauge
	ActiveRequests           prometheus.Gauge
	ActiveWorkers            prometheus.Gauge
	Batches                  prometheus.Counter
	BatchSize                prometheus.Histogram
	BatchWait                prometheus.Histogram
	QueueWaitHistogram       prometheus.Histogram
	BatchProcessingHistogram prometheus.Histogram
}

func NewMetrics(reg prometheus.Registerer, queueCapacity int) *Metrics {
	m := &Metrics{
		Requests:                 prometheus.NewCounterVec(prometheus.CounterOpts{Name: "llm_gateway_requests_total", Help: "Requests by final outcome."}, []string{"method", "route", "outcome"}),
		RequestDuration:          prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "llm_gateway_request_duration_seconds", Help: "End-to-end request duration.", Buckets: prometheus.DefBuckets}, []string{"route", "outcome"}),
		TTFT:                     prometheus.NewHistogram(prometheus.HistogramOpts{Name: "llm_gateway_time_to_first_token_seconds", Help: "Time to first token.", Buckets: prometheus.DefBuckets}),
		InterToken:               prometheus.NewHistogram(prometheus.HistogramOpts{Name: "llm_gateway_inter_token_duration_seconds", Help: "Delay between tokens.", Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1}}),
		Tokens:                   prometheus.NewCounter(prometheus.CounterOpts{Name: "llm_gateway_tokens_generated_total", Help: "Tokens streamed."}),
		QueueDepthGauge:          prometheus.NewGauge(prometheus.GaugeOpts{Name: "llm_gateway_queue_depth", Help: "Queued requests."}),
		QueueCapacity:            prometheus.NewGauge(prometheus.GaugeOpts{Name: "llm_gateway_queue_capacity", Help: "Queue capacity."}),
		QueueWaitHistogram:       prometheus.NewHistogram(prometheus.HistogramOpts{Name: "llm_gateway_queue_wait_seconds", Help: "Queue wait.", Buckets: prometheus.DefBuckets}),
		QueueRejections:          prometheus.NewCounter(prometheus.CounterOpts{Name: "llm_gateway_queue_rejections_total", Help: "Queue-full rejections."}),
		Batches:                  prometheus.NewCounter(prometheus.CounterOpts{Name: "llm_gateway_batches_total", Help: "Formed batches."}),
		BatchSize:                prometheus.NewHistogram(prometheus.HistogramOpts{Name: "llm_gateway_batch_size", Help: "Requests per batch.", Buckets: prometheus.LinearBuckets(1, 1, 16)}),
		BatchWait:                prometheus.NewHistogram(prometheus.HistogramOpts{Name: "llm_gateway_batch_wait_seconds", Help: "Time spent forming a batch.", Buckets: prometheus.DefBuckets}),
		BatchProcessingHistogram: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "llm_gateway_batch_processing_seconds", Help: "Worker batch duration.", Buckets: prometheus.DefBuckets}),
		ActiveRequests:           prometheus.NewGauge(prometheus.GaugeOpts{Name: "llm_gateway_active_requests", Help: "Active HTTP requests."}),
		ActiveWorkers:            prometheus.NewGauge(prometheus.GaugeOpts{Name: "llm_gateway_active_workers", Help: "Busy workers."}),
		Cancelled:                prometheus.NewCounter(prometheus.CounterOpts{Name: "llm_gateway_cancelled_requests_total", Help: "Client cancellations."}),
		TimedOut:                 prometheus.NewCounter(prometheus.CounterOpts{Name: "llm_gateway_timed_out_requests_total", Help: "Service timeouts."}),
	}
	reg.MustRegister(m.Requests, m.RequestDuration, m.TTFT, m.InterToken, m.Tokens, m.QueueDepthGauge, m.QueueCapacity, m.QueueWaitHistogram, m.QueueRejections, m.Batches, m.BatchSize, m.BatchWait, m.BatchProcessingHistogram, m.ActiveRequests, m.ActiveWorkers, m.Cancelled, m.TimedOut)
	m.QueueCapacity.Set(float64(queueCapacity))
	return m
}

func (m *Metrics) QueueDepth(n int)          { m.QueueDepthGauge.Set(float64(n)) }
func (m *Metrics) QueueWait(d time.Duration) { m.QueueWaitHistogram.Observe(d.Seconds()) }
func (m *Metrics) Batch(n int, d time.Duration) {
	m.Batches.Inc()
	m.BatchSize.Observe(float64(n))
	m.BatchWait.Observe(d.Seconds())
}
func (m *Metrics) BatchProcessing(d time.Duration) { m.BatchProcessingHistogram.Observe(d.Seconds()) }
func (m *Metrics) WorkerDelta(n int)               { m.ActiveWorkers.Add(float64(n)) }
