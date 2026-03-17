package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	PhaseWarmup    = "warmup"
	PhaseBenchmark = "benchmark"
	StatusOK       = "ok"
	StatusError    = "error"
)

var latencyBuckets = []float64{
	0.00005,
	0.0001,
	0.00025,
	0.0005,
	0.001,
	0.0025,
	0.005,
	0.01,
	0.025,
	0.05,
}

type ProcessorMetrics struct {
	registry           *prometheus.Registry
	requestsTotal      *prometheus.CounterVec
	processingDuration *prometheus.HistogramVec
}

func NewProcessorMetrics() (*ProcessorMetrics, error) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "benchmark",
			Subsystem: "processor",
			Name:      "requests_total",
			Help:      "Total de requests processadas pelo servico.",
		},
		[]string{"protocol", "scenario", "phase", "status"},
	)
	processingDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "benchmark",
			Subsystem: "processor",
			Name:      "processing_seconds",
			Help:      "Tempo de processamento do servico em segundos.",
			Buckets:   latencyBuckets,
		},
		[]string{"protocol", "scenario", "phase"},
	)

	if err := registry.Register(requestsTotal); err != nil {
		return nil, err
	}
	if err := registry.Register(processingDuration); err != nil {
		return nil, err
	}

	return &ProcessorMetrics{
		registry:           registry,
		requestsTotal:      requestsTotal,
		processingDuration: processingDuration,
	}, nil
}

func (m *ProcessorMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *ProcessorMetrics) Observe(protocol string, scenario string, phase string, durationSeconds float64, status string) {
	m.requestsTotal.WithLabelValues(protocol, scenario, phase, status).Inc()
	if status == StatusOK {
		m.processingDuration.WithLabelValues(protocol, scenario, phase).Observe(durationSeconds)
	}
}

type BenchmarkerMetrics struct {
	registry             *prometheus.Registry
	requestsTotal        *prometheus.CounterVec
	roundTripDuration    *prometheus.HistogramVec
	runTotalDuration     *prometheus.GaugeVec
	runRequestsTotal     *prometheus.GaugeVec
	runCompletedUnixTime *prometheus.GaugeVec
}

func NewBenchmarkerMetrics() (*BenchmarkerMetrics, error) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "benchmark",
			Subsystem: "client",
			Name:      "requests_total",
			Help:      "Total de requests realizadas pelo benchmarker.",
		},
		[]string{"protocol", "scenario", "phase", "status"},
	)
	roundTripDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "benchmark",
			Subsystem: "client",
			Name:      "round_trip_seconds",
			Help:      "Tempo de ida e volta medido pelo benchmarker em segundos.",
			Buckets:   latencyBuckets,
		},
		[]string{"protocol", "scenario", "phase"},
	)
	runTotalDuration := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "benchmark",
			Subsystem: "client",
			Name:      "run_total_seconds",
			Help:      "Tempo total do ultimo benchmark finalizado por protocolo em segundos.",
		},
		[]string{"protocol", "scenario"},
	)
	runRequestsTotal := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "benchmark",
			Subsystem: "client",
			Name:      "run_requests_total",
			Help:      "Quantidade de requests do ultimo benchmark finalizado por protocolo.",
		},
		[]string{"protocol", "scenario"},
	)
	runCompletedUnixTime := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "benchmark",
			Subsystem: "client",
			Name:      "run_completed_timestamp_seconds",
			Help:      "Timestamp Unix da ultima execucao finalizada por protocolo.",
		},
		[]string{"protocol", "scenario"},
	)

	collectorsToRegister := []prometheus.Collector{
		requestsTotal,
		roundTripDuration,
		runTotalDuration,
		runRequestsTotal,
		runCompletedUnixTime,
	}
	for _, collector := range collectorsToRegister {
		if err := registry.Register(collector); err != nil {
			return nil, err
		}
	}

	return &BenchmarkerMetrics{
		registry:             registry,
		requestsTotal:        requestsTotal,
		roundTripDuration:    roundTripDuration,
		runTotalDuration:     runTotalDuration,
		runRequestsTotal:     runRequestsTotal,
		runCompletedUnixTime: runCompletedUnixTime,
	}, nil
}

func (m *BenchmarkerMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *BenchmarkerMetrics) ObserveRequest(protocol string, scenario string, phase string, durationSeconds float64, status string) {
	m.requestsTotal.WithLabelValues(protocol, scenario, phase, status).Inc()
	if status == StatusOK {
		m.roundTripDuration.WithLabelValues(protocol, scenario, phase).Observe(durationSeconds)
	}
}

func (m *BenchmarkerMetrics) SetRunSummary(protocol string, scenario string, totalSeconds float64, totalRequests int, completedUnix float64) {
	m.runTotalDuration.WithLabelValues(protocol, scenario).Set(totalSeconds)
	m.runRequestsTotal.WithLabelValues(protocol, scenario).Set(float64(totalRequests))
	m.runCompletedUnixTime.WithLabelValues(protocol, scenario).Set(completedUnix)
}
