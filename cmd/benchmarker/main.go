package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"grpcbenchmark/internal/observability"
	"grpcbenchmark/internal/transport"
)

const (
	defaultGRPCProcessor        = "processor:50051"
	defaultHTTPProcessor        = "http://processor:8080/double"
	defaultTotalRequests        = 25000
	defaultWarmupRequests       = 100
	defaultBenchmarkRuns        = 5
	defaultConcurrency          = 64
	defaultHTTPMaxConns         = 64
	defaultRequestValue   int64 = 10
	warmupFlag                  = "warmup"
	defaultMetricsAddr          = ":2112"
	defaultMetricsGrace         = 30 * time.Second
	defaultReportsDir           = "/reports"
	defaultBenchmarkOrder       = "grpc,http-json"
	defaultScenarios            = "medium-structured,large-structured,large-structured-headers"
	defaultRequestTimeout       = 20 * time.Second
)

type config struct {
	executionID         string
	grpcAddr            string
	httpURL             string
	metricsAddr         string
	metricsGracePeriod  time.Duration
	reportsDir          string
	benchmarkOrder      []string
	requestTimeout      time.Duration
	totalRequests       int
	warmupRequests      int
	benchmarkRuns       int
	concurrency         int
	httpMaxConnsPerHost int
	scenarios           []scenarioConfig
}

type scenarioConfig struct {
	Name            string
	Description     string
	HeaderPairs     map[string]string
	JSONBytes       int
	ProtoBytes      int
	HeaderBytes     int
	BlobBytes       int
	ItemCount       int
	AttributeCount  int
	CounterCount    int
	NoteCount       int
	templatePayload *transport.Payload
}

type benchmarkResult struct {
	protocol      string
	scenario      string
	runNumber     int
	totalRequests int
	totalElapsed  time.Duration
	minElapsed    time.Duration
	maxElapsed    time.Duration
	avgElapsed    time.Duration
	startedAtUTC  time.Time
	finishedAtUTC time.Time
}

type protocolRunReport struct {
	RunNumber     int       `json:"run_number"`
	TotalRequests int       `json:"total_requests"`
	TotalElapsed  string    `json:"total_elapsed"`
	Average       string    `json:"average"`
	Min           string    `json:"min"`
	Max           string    `json:"max"`
	ThroughputRPS float64   `json:"throughput_rps"`
	StartedAtUTC  time.Time `json:"started_at_utc"`
	FinishedAtUTC time.Time `json:"finished_at_utc"`
}

type protocolSummaryReport struct {
	Protocol            string              `json:"protocol"`
	BenchmarkRuns       int                 `json:"benchmark_runs"`
	TotalRequestsPerRun int                 `json:"total_requests_per_run"`
	MedianTotal         string              `json:"median_total"`
	MedianAverage       string              `json:"median_average"`
	MedianMin           string              `json:"median_min"`
	MedianMax           string              `json:"median_max"`
	MedianThroughputRPS float64             `json:"median_throughput_rps"`
	MedianTotalSeconds  float64             `json:"median_total_seconds"`
	Runs                []protocolRunReport `json:"runs"`
}

type scenarioReport struct {
	Name               string                           `json:"name"`
	Description        string                           `json:"description"`
	JSONBytes          int                              `json:"json_bytes"`
	ProtoBytes         int                              `json:"proto_bytes"`
	HeaderBytes        int                              `json:"header_bytes"`
	BlobBytes          int                              `json:"blob_bytes"`
	ItemCount          int                              `json:"item_count"`
	AttributeCount     int                              `json:"attribute_count"`
	CounterCount       int                              `json:"counter_count"`
	NoteCount          int                              `json:"note_count"`
	ClientResults      map[string]protocolSummaryReport `json:"client_results"`
	Winner             string                           `json:"winner,omitempty"`
	WinnerSummary      string                           `json:"winner_summary,omitempty"`
	SerializationDelta string                           `json:"serialization_delta"`
}

type benchmarkReport struct {
	ExecutionID         string           `json:"execution_id"`
	GeneratedAtUTC      time.Time        `json:"generated_at_utc"`
	BenchmarkOrder      string           `json:"benchmark_order"`
	TotalRequests       int              `json:"total_requests_per_protocol_per_run"`
	WarmupRequests      int              `json:"warmup_requests_per_protocol"`
	BenchmarkRuns       int              `json:"benchmark_runs"`
	Concurrency         int              `json:"concurrency"`
	HTTPMaxConnsPerHost int              `json:"http_max_conns_per_host"`
	RequestTimeout      string           `json:"request_timeout"`
	Scenarios           []scenarioReport `json:"scenarios"`
	Notes               []string         `json:"notes"`
}

type reportPaths struct {
	MarkdownPath string
	JSONPath     string
}

type requestOutcome struct {
	elapsed time.Duration
	err     error
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := loadConfig()
	log.Printf(
		"benchmarker config: order=%v scenarios=%d total_requests=%d warmup=%d runs=%d concurrency=%d http_max_conns=%d request_timeout=%s",
		cfg.benchmarkOrder,
		len(cfg.scenarios),
		cfg.totalRequests,
		cfg.warmupRequests,
		cfg.benchmarkRuns,
		cfg.concurrency,
		cfg.httpMaxConnsPerHost,
		cfg.requestTimeout,
	)

	metrics, err := observability.NewBenchmarkerMetrics()
	if err != nil {
		log.Fatalf("creating benchmarker metrics: %v", err)
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metrics.Handler())
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	metricsServer := &http.Server{
		Addr:              cfg.metricsAddr,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("benchmarker metrics listening on %s", cfg.metricsAddr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("benchmarker metrics server stopped: %v", err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsServer.Shutdown(shutdownCtx)
	}()

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        cfg.httpMaxConnsPerHost,
			MaxIdleConnsPerHost: cfg.httpMaxConnsPerHost,
			MaxConnsPerHost:     cfg.httpMaxConnsPerHost,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	conn, err := grpc.NewClient(
		cfg.grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("creating gRPC client: %v", err)
	}
	defer conn.Close()

	grpcClient := transport.NewProcessorServiceClient(conn)

	if err := waitForProcessor(ctx, httpClient, grpcClient, cfg); err != nil {
		log.Fatalf("processor unavailable: %v", err)
	}

	if cfg.warmupRequests > 0 {
		if err := runWarmup(ctx, httpClient, grpcClient, cfg, metrics); err != nil {
			log.Fatalf("warmup failed: %v", err)
		}
	}

	report, err := runBenchmarkScenarios(ctx, httpClient, grpcClient, metrics, cfg)
	if err != nil {
		log.Fatalf("running benchmark scenarios: %v", err)
	}

	reportPaths, err := writeReports(cfg.reportsDir, report)
	if err != nil {
		log.Fatalf("writing reports: %v", err)
	}

	printSummary(report)
	fmt.Printf("Relatorios -> markdown: %s | json: %s\n", reportPaths.MarkdownPath, reportPaths.JSONPath)

	if cfg.metricsGracePeriod > 0 {
		log.Printf("keeping metrics endpoint available for %s", cfg.metricsGracePeriod)
		select {
		case <-ctx.Done():
		case <-time.After(cfg.metricsGracePeriod):
		}
	}
}

func loadConfig() config {
	concurrency := maxInt(1, getEnvInt("CONCURRENCY", defaultConcurrency))
	scenarios := getSelectedScenarios(getEnv("SCENARIOS", defaultScenarios))

	return config{
		executionID:         uuid.NewString(),
		grpcAddr:            getEnv("PROCESSOR_GRPC_ADDR", defaultGRPCProcessor),
		httpURL:             getEnv("PROCESSOR_HTTP_URL", defaultHTTPProcessor),
		metricsAddr:         getEnv("METRICS_ADDR", defaultMetricsAddr),
		metricsGracePeriod:  getEnvDuration("METRICS_GRACE_PERIOD", defaultMetricsGrace),
		reportsDir:          getEnv("REPORTS_DIR", defaultReportsDir),
		benchmarkOrder:      getBenchmarkOrder(getEnv("BENCHMARK_ORDER", defaultBenchmarkOrder)),
		requestTimeout:      getEnvDuration("REQUEST_TIMEOUT", defaultRequestTimeout),
		totalRequests:       getEnvInt("TOTAL_REQUESTS", defaultTotalRequests),
		warmupRequests:      getEnvInt("WARMUP_REQUESTS", defaultWarmupRequests),
		benchmarkRuns:       maxInt(1, getEnvInt("BENCHMARK_RUNS", defaultBenchmarkRuns)),
		concurrency:         concurrency,
		httpMaxConnsPerHost: maxInt(1, getEnvInt("HTTP_MAX_CONNS_PER_HOST", defaultHTTPMaxConns)),
		scenarios:           scenarios,
	}
}

func runBenchmarkScenarios(
	ctx context.Context,
	httpClient *http.Client,
	grpcClient transport.ProcessorServiceClient,
	metrics *observability.BenchmarkerMetrics,
	cfg config,
) (benchmarkReport, error) {
	report := benchmarkReport{
		ExecutionID:         cfg.executionID,
		GeneratedAtUTC:      time.Now().UTC(),
		BenchmarkOrder:      strings.Join(cfg.benchmarkOrder, ","),
		TotalRequests:       cfg.totalRequests,
		WarmupRequests:      cfg.warmupRequests,
		BenchmarkRuns:       cfg.benchmarkRuns,
		Concurrency:         cfg.concurrency,
		HTTPMaxConnsPerHost: cfg.httpMaxConnsPerHost,
		RequestTimeout:      cfg.requestTimeout.String(),
		Scenarios:           make([]scenarioReport, 0, len(cfg.scenarios)),
		Notes: []string{
			"Comparacao principal baseada no round-trip do cliente por mediana.",
			"O processor apenas dobra o valor e devolve o mesmo payload para reduzir logica de negocio no meio da medicao.",
			"gRPC usa protobuf precompilado e HTTP usa JSON sobre o mesmo modelo semantico.",
		},
	}

	for _, scenario := range cfg.scenarios {
		log.Printf("running scenario %s", scenario.Name)
		results := make(map[string][]benchmarkResult, len(cfg.benchmarkOrder))

		for runNumber := 1; runNumber <= cfg.benchmarkRuns; runNumber++ {
			log.Printf("scenario=%s run=%d/%d order=%v", scenario.Name, runNumber, cfg.benchmarkRuns, cfg.benchmarkOrder)
			for _, protocol := range cfg.benchmarkOrder {
				switch protocol {
				case "grpc":
					result, err := runGRPCBenchmark(ctx, grpcClient, metrics, scenario, runNumber, cfg.totalRequests, cfg.requestTimeout, cfg.concurrency)
					if err != nil {
						return benchmarkReport{}, err
					}
					results[protocol] = append(results[protocol], result)
				case "http-json":
					result, err := runHTTPBenchmark(ctx, httpClient, metrics, scenario, cfg.httpURL, runNumber, cfg.totalRequests, cfg.requestTimeout, cfg.concurrency)
					if err != nil {
						return benchmarkReport{}, err
					}
					results[protocol] = append(results[protocol], result)
				default:
					return benchmarkReport{}, fmt.Errorf("unsupported protocol: %s", protocol)
				}
			}
		}

		scenarioReport := buildScenarioReport(scenario, results)
		report.Scenarios = append(report.Scenarios, scenarioReport)
		for protocol, summary := range scenarioReport.ClientResults {
			metrics.SetRunSummary(protocol, scenario.Name, summary.MedianTotalSeconds, summary.TotalRequestsPerRun, float64(lastFinishedAt(summary).Unix()))
		}
		for _, protocol := range []string{"grpc", "http-json"} {
			if _, ok := scenarioReport.ClientResults[protocol]; !ok {
				metrics.SetRunSummary(protocol, scenario.Name, 0, 0, 0)
			}
		}
	}

	return report, nil
}

func runWarmup(ctx context.Context, httpClient *http.Client, grpcClient transport.ProcessorServiceClient, cfg config, metrics *observability.BenchmarkerMetrics) error {
	log.Printf("running %d warmup requests per scenario/protocol", cfg.warmupRequests)

	for _, scenario := range cfg.scenarios {
		for _, protocol := range cfg.benchmarkOrder {
			for range cfg.warmupRequests {
				payload := buildPayload(scenario, uuid.NewString())
				switch protocol {
				case "grpc":
					startedAt := time.Now()
					callCtx, cancel := context.WithTimeout(ctx, cfg.requestTimeout)
					md := metadata.New(buildScenarioHeaders(scenario, true))
					_, err := grpcClient.Double(metadata.NewOutgoingContext(callCtx, md), payload)
					cancel()
					if err != nil {
						metrics.ObserveRequest(protocol, scenario.Name, observability.PhaseWarmup, 0, observability.StatusError)
						return fmt.Errorf("grpc warmup for %s: %w", scenario.Name, err)
					}
					metrics.ObserveRequest(protocol, scenario.Name, observability.PhaseWarmup, time.Since(startedAt).Seconds(), observability.StatusOK)
				case "http-json":
					startedAt := time.Now()
					if _, err := sendHTTP(ctx, httpClient, cfg.httpURL, payload, buildScenarioHeaderHTTP(scenario, true), cfg.requestTimeout); err != nil {
						metrics.ObserveRequest(protocol, scenario.Name, observability.PhaseWarmup, 0, observability.StatusError)
						return fmt.Errorf("http warmup for %s: %w", scenario.Name, err)
					}
					metrics.ObserveRequest(protocol, scenario.Name, observability.PhaseWarmup, time.Since(startedAt).Seconds(), observability.StatusOK)
				}
			}
		}
	}

	return nil
}

func runGRPCBenchmark(ctx context.Context, client transport.ProcessorServiceClient, metrics *observability.BenchmarkerMetrics, scenario scenarioConfig, runNumber int, total int, timeout time.Duration, concurrency int) (benchmarkResult, error) {
	return runConcurrentBenchmark(ctx, "grpc", scenario.Name, total, concurrency, func(runCtx context.Context) requestOutcome {
		payload := buildPayload(scenario, uuid.NewString())
		headers := buildScenarioHeaders(scenario, false)
		requestStarted := time.Now()

		callCtx, cancel := context.WithTimeout(runCtx, timeout)
		response, err := client.Double(metadata.NewOutgoingContext(callCtx, metadata.New(headers)), payload)
		cancel()
		if err != nil {
			metrics.ObserveRequest("grpc", scenario.Name, observability.PhaseBenchmark, 0, observability.StatusError)
			return requestOutcome{err: err}
		}

		elapsed := time.Since(requestStarted)
		if err := validateResponse(payload, response); err != nil {
			return requestOutcome{err: err}
		}
		metrics.ObserveRequest("grpc", scenario.Name, observability.PhaseBenchmark, elapsed.Seconds(), observability.StatusOK)
		return requestOutcome{elapsed: elapsed}
	}, runNumber)
}

func runHTTPBenchmark(ctx context.Context, httpClient *http.Client, metrics *observability.BenchmarkerMetrics, scenario scenarioConfig, url string, runNumber int, total int, timeout time.Duration, concurrency int) (benchmarkResult, error) {
	return runConcurrentBenchmark(ctx, "http-json", scenario.Name, total, concurrency, func(runCtx context.Context) requestOutcome {
		payload := buildPayload(scenario, uuid.NewString())
		headers := buildScenarioHeaderHTTP(scenario, false)
		requestStarted := time.Now()

		response, err := sendHTTP(runCtx, httpClient, url, payload, headers, timeout)
		if err != nil {
			metrics.ObserveRequest("http-json", scenario.Name, observability.PhaseBenchmark, 0, observability.StatusError)
			return requestOutcome{err: err}
		}

		elapsed := time.Since(requestStarted)
		if err := validateResponse(payload, response); err != nil {
			return requestOutcome{err: err}
		}
		metrics.ObserveRequest("http-json", scenario.Name, observability.PhaseBenchmark, elapsed.Seconds(), observability.StatusOK)
		return requestOutcome{elapsed: elapsed}
	}, runNumber)
}

func runConcurrentBenchmark(ctx context.Context, protocol string, scenarioName string, total int, concurrency int, requestFn func(context.Context) requestOutcome, runNumber int) (benchmarkResult, error) {
	startedAt := time.Now().UTC()
	minElapsed := time.Duration(1<<63 - 1)
	maxElapsed := time.Duration(0)
	totalElapsed := time.Duration(0)
	workers := minInt(maxInt(1, concurrency), maxInt(1, total))

	benchmarkCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var successCount int
	var firstErr error

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				if benchmarkCtx.Err() != nil {
					continue
				}

				outcome := requestFn(benchmarkCtx)
				if outcome.err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = outcome.err
						cancel()
					}
					mu.Unlock()
					continue
				}

				mu.Lock()
				successCount++
				totalElapsed += outcome.elapsed
				if outcome.elapsed < minElapsed {
					minElapsed = outcome.elapsed
				}
				if outcome.elapsed > maxElapsed {
					maxElapsed = outcome.elapsed
				}
				mu.Unlock()
			}
		}()
	}

dispatch:
	for range total {
		select {
		case <-benchmarkCtx.Done():
			break dispatch
		case jobs <- struct{}{}:
		}
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return benchmarkResult{}, firstErr
	}
	if successCount == 0 {
		return benchmarkResult{}, fmt.Errorf("%s benchmark for %s completed without successful requests", protocol, scenarioName)
	}

	finishedAt := time.Now().UTC()
	return benchmarkResult{
		protocol:      protocol,
		scenario:      scenarioName,
		runNumber:     runNumber,
		totalRequests: successCount,
		totalElapsed:  finishedAt.Sub(startedAt),
		minElapsed:    minElapsed,
		maxElapsed:    maxElapsed,
		avgElapsed:    totalElapsed / time.Duration(successCount),
		startedAtUTC:  startedAt,
		finishedAtUTC: finishedAt,
	}, nil
}

func sendHTTP(ctx context.Context, client *http.Client, url string, payload *transport.Payload, headers http.Header, timeout time.Duration) (*transport.Payload, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if len(body) == 0 {
			return nil, fmt.Errorf("http request returned %s", resp.Status)
		}
		return nil, fmt.Errorf("http request returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var response transport.Payload
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	return &response, nil
}

func waitForProcessor(ctx context.Context, httpClient *http.Client, grpcClient transport.ProcessorServiceClient, cfg config) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if processorReady(ctx, httpClient, grpcClient, cfg) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("processor did not become healthy within 30s")
}

func processorReady(ctx context.Context, httpClient *http.Client, grpcClient transport.ProcessorServiceClient, cfg config) bool {
	for _, protocol := range cfg.benchmarkOrder {
		switch protocol {
		case "grpc":
			if err := pingGRPC(ctx, grpcClient, cfg.requestTimeout); err != nil {
				return false
			}
		case "http-json":
			if err := pingHTTP(ctx, httpClient, cfg.httpURL); err != nil {
				return false
			}
		}
	}
	return true
}

func pingHTTP(ctx context.Context, client *http.Client, requestURL string) error {
	healthURL := strings.TrimSuffix(requestURL, "/double") + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http health returned %s", resp.Status)
	}
	return nil
}

func pingGRPC(ctx context.Context, client transport.ProcessorServiceClient, timeout time.Duration) error {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload := buildPayload(builtInScenarios()["medium-structured"], uuid.NewString())
	md := metadata.New(buildScenarioHeaders(builtInScenarios()["medium-structured"], true))
	response, err := client.Double(metadata.NewOutgoingContext(callCtx, md), payload)
	if err != nil {
		return err
	}
	return validateResponse(payload, response)
}

func validateResponse(request *transport.Payload, response *transport.Payload) error {
	switch {
	case response.GetUuid() != request.GetUuid():
		return fmt.Errorf("uuid mismatch")
	case response.GetScenario() != request.GetScenario():
		return fmt.Errorf("scenario mismatch")
	case response.GetValor() != request.GetValor()*2:
		return fmt.Errorf("valor mismatch")
	case len(response.GetItems()) != len(request.GetItems()):
		return fmt.Errorf("items mismatch")
	case len(response.GetAttributes()) != len(request.GetAttributes()):
		return fmt.Errorf("attributes mismatch")
	case len(response.GetCounters()) != len(request.GetCounters()):
		return fmt.Errorf("counters mismatch")
	case len(response.GetNotes()) != len(request.GetNotes()):
		return fmt.Errorf("notes mismatch")
	case len(response.GetBlob()) != len(request.GetBlob()):
		return fmt.Errorf("blob mismatch")
	}
	return nil
}

func buildScenarioReport(scenario scenarioConfig, results map[string][]benchmarkResult) scenarioReport {
	clientResults := make(map[string]protocolSummaryReport, len(results))
	for protocol, runs := range results {
		clientResults[protocol] = newProtocolSummaryReport(protocol, runs)
	}

	winner, winnerSummary := buildWinnerSummary(clientResults)
	return scenarioReport{
		Name:               scenario.Name,
		Description:        scenario.Description,
		JSONBytes:          scenario.JSONBytes,
		ProtoBytes:         scenario.ProtoBytes,
		HeaderBytes:        scenario.HeaderBytes,
		BlobBytes:          scenario.BlobBytes,
		ItemCount:          scenario.ItemCount,
		AttributeCount:     scenario.AttributeCount,
		CounterCount:       scenario.CounterCount,
		NoteCount:          scenario.NoteCount,
		ClientResults:      clientResults,
		Winner:             winner,
		WinnerSummary:      winnerSummary,
		SerializationDelta: serializationDeltaText(scenario.JSONBytes, scenario.ProtoBytes),
	}
}

func buildWinnerSummary(results map[string]protocolSummaryReport) (string, string) {
	grpcResult, grpcOK := results["grpc"]
	httpResult, httpOK := results["http-json"]
	if !grpcOK || !httpOK {
		return "", ""
	}

	grpcDuration, err := time.ParseDuration(grpcResult.MedianTotal)
	if err != nil {
		return "", ""
	}
	httpDuration, err := time.ParseDuration(httpResult.MedianTotal)
	if err != nil {
		return "", ""
	}
	if grpcDuration == httpDuration {
		return "tie", "gRPC e HTTP JSON tiveram a mesma mediana de tempo total."
	}
	if grpcDuration < httpDuration {
		return "grpc", fmt.Sprintf("gRPC foi %.2fx mais rapido na mediana do tempo total.", httpDuration.Seconds()/grpcDuration.Seconds())
	}
	return "http-json", fmt.Sprintf("HTTP JSON foi %.2fx mais rapido na mediana do tempo total.", grpcDuration.Seconds()/httpDuration.Seconds())
}

func serializationDeltaText(jsonBytes int, protoBytes int) string {
	if jsonBytes <= 0 || protoBytes <= 0 {
		return ""
	}
	if protoBytes == jsonBytes {
		return fmt.Sprintf("JSON e protobuf tiveram o mesmo tamanho serializado (%d bytes).", jsonBytes)
	}
	if protoBytes < jsonBytes {
		return fmt.Sprintf("protobuf ficou %.2fx menor que JSON (%d bytes vs %d bytes).", float64(jsonBytes)/float64(protoBytes), protoBytes, jsonBytes)
	}
	return fmt.Sprintf("JSON ficou %.2fx menor que protobuf (%d bytes vs %d bytes).", float64(protoBytes)/float64(jsonBytes), jsonBytes, protoBytes)
}

func newProtocolSummaryReport(protocol string, runs []benchmarkResult) protocolSummaryReport {
	runReports := make([]protocolRunReport, 0, len(runs))
	totalDurations := make([]time.Duration, 0, len(runs))
	avgDurations := make([]time.Duration, 0, len(runs))
	minDurations := make([]time.Duration, 0, len(runs))
	maxDurations := make([]time.Duration, 0, len(runs))

	for _, run := range runs {
		runReports = append(runReports, protocolRunReport{
			RunNumber:     run.runNumber,
			TotalRequests: run.totalRequests,
			TotalElapsed:  run.totalElapsed.String(),
			Average:       run.avgElapsed.String(),
			Min:           run.minElapsed.String(),
			Max:           run.maxElapsed.String(),
			ThroughputRPS: throughputFor(run.totalRequests, run.totalElapsed),
			StartedAtUTC:  run.startedAtUTC,
			FinishedAtUTC: run.finishedAtUTC,
		})
		totalDurations = append(totalDurations, run.totalElapsed)
		avgDurations = append(avgDurations, run.avgElapsed)
		minDurations = append(minDurations, run.minElapsed)
		maxDurations = append(maxDurations, run.maxElapsed)
	}

	medianTotal := medianDuration(totalDurations)
	return protocolSummaryReport{
		Protocol:            protocol,
		BenchmarkRuns:       len(runs),
		TotalRequestsPerRun: runs[0].totalRequests,
		MedianTotal:         medianTotal.String(),
		MedianAverage:       medianDuration(avgDurations).String(),
		MedianMin:           medianDuration(minDurations).String(),
		MedianMax:           medianDuration(maxDurations).String(),
		MedianThroughputRPS: throughputFor(runs[0].totalRequests, medianTotal),
		MedianTotalSeconds:  medianTotal.Seconds(),
		Runs:                runReports,
	}
}

func writeReports(dir string, report benchmarkReport) (reportPaths, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return reportPaths{}, err
	}
	if err := cleanupOldReports(dir); err != nil {
		return reportPaths{}, err
	}

	jsonPath := filepath.Join(dir, "latest.json")
	mdPath := filepath.Join(dir, "latest.md")

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return reportPaths{}, err
	}
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		return reportPaths{}, err
	}

	mdBytes := []byte(renderMarkdownReport(report))
	if err := os.WriteFile(mdPath, mdBytes, 0o644); err != nil {
		return reportPaths{}, err
	}

	return reportPaths{MarkdownPath: mdPath, JSONPath: jsonPath}, nil
}

func renderMarkdownReport(report benchmarkReport) string {
	var scenarioRows strings.Builder
	var scenarioSections strings.Builder

	for _, scenario := range report.Scenarios {
		fmt.Fprintf(&scenarioRows, "| %s | %d | %d | %d | %s | %s |\n",
			scenario.Name,
			scenario.ProtoBytes,
			scenario.JSONBytes,
			scenario.HeaderBytes,
			displayWinner(scenario.Winner),
			scenario.WinnerSummary,
		)

		fmt.Fprintf(&scenarioSections, "## %s\n\n", scenario.Name)
		fmt.Fprintf(&scenarioSections, "- %s\n", scenario.Description)
		fmt.Fprintf(&scenarioSections, "- protobuf: %d bytes\n", scenario.ProtoBytes)
		fmt.Fprintf(&scenarioSections, "- json: %d bytes\n", scenario.JSONBytes)
		fmt.Fprintf(&scenarioSections, "- headers extras: %d bytes\n", scenario.HeaderBytes)
		fmt.Fprintf(&scenarioSections, "- blob: %d bytes | items: %d | attributes: %d | counters: %d | notes: %d\n\n",
			scenario.BlobBytes,
			scenario.ItemCount,
			scenario.AttributeCount,
			scenario.CounterCount,
			scenario.NoteCount,
		)
		if scenario.SerializationDelta != "" {
			fmt.Fprintf(&scenarioSections, "- %s\n\n", scenario.SerializationDelta)
		}

		scenarioSections.WriteString("| Protocolo | Mediana Total | Mediana Media | Mediana Min | Mediana Max | Mediana Throughput req/s |\n")
		scenarioSections.WriteString("| --- | --- | --- | --- | --- | --- |\n")
		for _, protocol := range []string{"grpc", "http-json"} {
			result, ok := scenario.ClientResults[protocol]
			if !ok {
				continue
			}
			fmt.Fprintf(&scenarioSections, "| %s | %s | %s | %s | %s | %.2f |\n",
				displayProtocol(protocol),
				result.MedianTotal,
				result.MedianAverage,
				result.MedianMin,
				result.MedianMax,
				result.MedianThroughputRPS,
			)
		}
		scenarioSections.WriteString("\n")

		for _, protocol := range []string{"grpc", "http-json"} {
			result, ok := scenario.ClientResults[protocol]
			if !ok {
				continue
			}
			fmt.Fprintf(&scenarioSections, "### %s\n\n", displayProtocol(protocol))
			scenarioSections.WriteString("| Run | Total | Media | Min | Max | Throughput req/s |\n")
			scenarioSections.WriteString("| --- | --- | --- | --- | --- | --- |\n")
			for _, run := range result.Runs {
				fmt.Fprintf(&scenarioSections, "| %d | %s | %s | %s | %s | %.2f |\n",
					run.RunNumber,
					run.TotalElapsed,
					run.Average,
					run.Min,
					run.Max,
					run.ThroughputRPS,
				)
			}
			scenarioSections.WriteString("\n")
		}
	}

	return fmt.Sprintf(`# Benchmark gRPC vs HTTP JSON

- Execucao: %s
- Gerado em UTC: %s
- Ordem de execucao: %s
- Requests por protocolo por run: %d
- Warmup por protocolo: %d
- Runs por protocolo: %d
- Concorrencia: %d
- HTTP max conns por host: %d
- Timeout por request: %s

## Resumo de Cenarios

| Cenario | Proto bytes | JSON bytes | Headers bytes | Vencedor | Resumo |
| --- | --- | --- | --- | --- | --- |
%s

## Notas

%s

%s`,
		report.ExecutionID,
		report.GeneratedAtUTC.Format(time.RFC3339),
		report.BenchmarkOrder,
		report.TotalRequests,
		report.WarmupRequests,
		report.BenchmarkRuns,
		report.Concurrency,
		report.HTTPMaxConnsPerHost,
		report.RequestTimeout,
		scenarioRows.String(),
		renderNotes(report.Notes),
		scenarioSections.String(),
	)
}

func printSummary(report benchmarkReport) {
	fmt.Println("Benchmark finalizado")
	fmt.Println()
	for _, scenario := range report.Scenarios {
		fmt.Printf("%s -> vencedor: %s", scenario.Name, displayWinner(scenario.Winner))
		if scenario.WinnerSummary != "" {
			fmt.Printf(" | %s", scenario.WinnerSummary)
		}
		fmt.Println()
	}
}

func buildPayload(scenario scenarioConfig, requestID string) *transport.Payload {
	payload := cloneTemplatePayload(scenario.templatePayload)
	payload.Uuid = requestID
	payload.Valor = defaultRequestValue
	return payload
}

func cloneTemplatePayload(template *transport.Payload) *transport.Payload {
	if template == nil {
		return &transport.Payload{}
	}

	cloned := &transport.Payload{
		Uuid:     template.GetUuid(),
		Valor:    template.GetValor(),
		Scenario: template.GetScenario(),
		Blob:     append([]byte(nil), template.GetBlob()...),
		Counters: append([]int64(nil), template.GetCounters()...),
		Notes:    append([]string(nil), template.GetNotes()...),
	}

	if customer := template.GetCustomer(); customer != nil {
		cloned.Customer = &transport.Customer{
			CustomerId: customer.GetCustomerId(),
			Segment:    customer.GetSegment(),
			Email:      customer.GetEmail(),
			Phones:     append([]string(nil), customer.GetPhones()...),
		}
		if address := customer.GetAddress(); address != nil {
			cloned.Customer.Address = &transport.Address{
				Street:     address.GetStreet(),
				City:       address.GetCity(),
				State:      address.GetState(),
				PostalCode: address.GetPostalCode(),
				Country:    address.GetCountry(),
			}
		}
	}

	if len(template.GetItems()) > 0 {
		cloned.Items = make([]*transport.Item, 0, len(template.GetItems()))
		for _, item := range template.GetItems() {
			cloned.Items = append(cloned.Items, &transport.Item{
				Sku:       item.GetSku(),
				Name:      item.GetName(),
				Quantity:  item.GetQuantity(),
				UnitPrice: item.GetUnitPrice(),
				Tags:      append([]string(nil), item.GetTags()...),
			})
		}
	}

	if len(template.GetAttributes()) > 0 {
		cloned.Attributes = make([]*transport.Attribute, 0, len(template.GetAttributes()))
		for _, attribute := range template.GetAttributes() {
			cloned.Attributes = append(cloned.Attributes, &transport.Attribute{
				Key:   attribute.GetKey(),
				Value: attribute.GetValue(),
			})
		}
	}

	return cloned
}

func buildScenarioHeaders(scenario scenarioConfig, warmup bool) map[string]string {
	headers := make(map[string]string, len(scenario.HeaderPairs)+2)
	for key, value := range scenario.HeaderPairs {
		headers[key] = value
	}
	headers["benchmark-scenario"] = scenario.Name
	if warmup {
		headers["benchmark-mode"] = warmupFlag
	}
	return headers
}

func buildScenarioHeaderHTTP(scenario scenarioConfig, warmup bool) http.Header {
	headers := make(http.Header, len(scenario.HeaderPairs)+2)
	for key, value := range scenario.HeaderPairs {
		headers.Set(key, value)
	}
	headers.Set("X-Benchmark-Scenario", scenario.Name)
	if warmup {
		headers.Set("X-Benchmark-Mode", warmupFlag)
	}
	return headers
}

func getSelectedScenarios(value string) []scenarioConfig {
	available := builtInScenarios()
	names := strings.Split(value, ",")
	selected := make([]scenarioConfig, 0, len(names))
	for _, name := range names {
		key := strings.TrimSpace(name)
		if key == "" {
			continue
		}
		scenario, ok := available[key]
		if !ok {
			continue
		}
		selected = append(selected, scenario)
	}
	if len(selected) == 0 {
		return []scenarioConfig{available["medium-structured"], available["large-structured"], available["large-structured-headers"]}
	}
	return selected
}

func builtInScenarios() map[string]scenarioConfig {
	return map[string]scenarioConfig{
		"medium-structured":        makeScenario("medium-structured", "Payload estruturado medio, sem headers extras.", 8*1024, 12, 24, 96, 24, 0, 0),
		"large-structured":         makeScenario("large-structured", "Payload estruturado grande, sem headers extras.", 32*1024, 24, 48, 192, 48, 0, 0),
		"large-structured-headers": makeScenario("large-structured-headers", "Mesmo payload grande, adicionando headers/metadados extras nos dois protocolos.", 32*1024, 24, 48, 192, 48, 12, 192),
	}
}

func makeScenario(name string, description string, blobBytes int, itemCount int, attributeCount int, counterCount int, noteCount int, headerCount int, headerValueBytes int) scenarioConfig {
	headerPairs := make(map[string]string, headerCount)
	headerBytes := 0
	headerValue := strings.Repeat("h", headerValueBytes)
	for idx := 0; idx < headerCount; idx++ {
		key := fmt.Sprintf("x-bench-%02d", idx)
		headerPairs[key] = headerValue
		headerBytes += len(key) + len(headerValue)
	}

	payload := &transport.Payload{
		Uuid:     "template",
		Valor:    defaultRequestValue,
		Scenario: name,
		Customer: &transport.Customer{
			CustomerId: "CUST-001",
			Segment:    "enterprise",
			Email:      "benchmark@example.com",
			Phones:     []string{"+55-11-5555-0101", "+55-11-5555-0102", "+55-11-5555-0103"},
			Address: &transport.Address{
				Street:     "Avenida Benchmark 1000",
				City:       "Sao Paulo",
				State:      "SP",
				PostalCode: "01000-000",
				Country:    "BR",
			},
		},
		Items:      make([]*transport.Item, 0, itemCount),
		Attributes: make([]*transport.Attribute, 0, attributeCount),
		Counters:   make([]int64, 0, counterCount),
		Notes:      make([]string, 0, noteCount),
		Blob:       bytes.Repeat([]byte("b"), blobBytes),
	}

	for idx := 0; idx < itemCount; idx++ {
		payload.Items = append(payload.Items, &transport.Item{
			Sku:       fmt.Sprintf("SKU-%04d", idx),
			Name:      fmt.Sprintf("Produto benchmark %02d", idx),
			Quantity:  int32((idx % 5) + 1),
			UnitPrice: float64((idx%20)+1) * 3.75,
			Tags: []string{
				"bench",
				"grpc-vs-http",
				fmt.Sprintf("tag-%02d", idx%10),
				fmt.Sprintf("group-%02d", idx%6),
			},
		})
	}

	for idx := 0; idx < attributeCount; idx++ {
		payload.Attributes = append(payload.Attributes, &transport.Attribute{
			Key:   fmt.Sprintf("attribute_%02d", idx),
			Value: strings.Repeat(fmt.Sprintf("value%02d", idx%10), 4),
		})
	}

	for idx := 0; idx < counterCount; idx++ {
		payload.Counters = append(payload.Counters, int64(1000+idx*13))
	}

	for idx := 0; idx < noteCount; idx++ {
		payload.Notes = append(payload.Notes, fmt.Sprintf("nota-%02d-%s", idx, strings.Repeat("n", 24)))
	}

	jsonBytes, _ := json.Marshal(payload)
	protoBytes, _ := proto.Marshal(payload)

	return scenarioConfig{
		Name:            name,
		Description:     description,
		HeaderPairs:     headerPairs,
		JSONBytes:       len(jsonBytes),
		ProtoBytes:      len(protoBytes),
		HeaderBytes:     headerBytes,
		BlobBytes:       blobBytes,
		ItemCount:       itemCount,
		AttributeCount:  attributeCount,
		CounterCount:    counterCount,
		NoteCount:       noteCount,
		templatePayload: payload,
	}
}

func cleanupOldReports(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "latest") || strings.HasPrefix(name, "benchmark_") {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderNotes(notes []string) string {
	if len(notes) == 0 {
		return "- Nenhuma nota.\n"
	}
	var builder strings.Builder
	for _, note := range notes {
		builder.WriteString("- ")
		builder.WriteString(note)
		builder.WriteString("\n")
	}
	return builder.String()
}

func displayProtocol(protocol string) string {
	switch protocol {
	case "grpc":
		return "gRPC"
	case "http-json":
		return "HTTP JSON"
	default:
		return protocol
	}
}

func displayWinner(winner string) string {
	switch winner {
	case "grpc":
		return "gRPC"
	case "http-json":
		return "HTTP JSON"
	case "tie":
		return "Empate"
	default:
		return "-"
	}
}

func lastFinishedAt(report protocolSummaryReport) time.Time {
	var latest time.Time
	for _, run := range report.Runs {
		if run.FinishedAtUTC.After(latest) {
			latest = run.FinishedAtUTC
		}
	}
	return latest
}

func throughputFor(totalRequests int, totalElapsed time.Duration) float64 {
	if totalElapsed <= 0 {
		return 0
	}
	return float64(totalRequests) / totalElapsed.Seconds()
}

func medianDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i] < sorted[j]
	})
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

func getBenchmarkOrder(value string) []string {
	parts := strings.Split(value, ",")
	order := make([]string, 0, len(parts))
	for _, item := range parts {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		order = append(order, normalized)
	}
	if len(order) == 0 {
		return []string{"grpc", "http-json"}
	}

	validOrder := make([]string, 0, len(order))
	for _, protocol := range order {
		if protocol != "grpc" && protocol != "http-json" {
			continue
		}
		if slices.Contains(validOrder, protocol) {
			continue
		}
		validOrder = append(validOrder, protocol)
	}
	if len(validOrder) == 0 {
		return []string{"grpc", "http-json"}
	}
	return validOrder
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
