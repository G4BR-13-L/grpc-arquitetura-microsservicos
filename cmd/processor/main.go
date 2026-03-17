package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"grpcbenchmark/internal/observability"
	"grpcbenchmark/internal/transport"
)

const (
	defaultGRPCAddr = ":50051"
	defaultHTTPAddr = ":8080"
	warmupFlag      = "warmup"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	grpcAddr := getEnv("GRPC_ADDR", defaultGRPCAddr)
	httpAddr := getEnv("HTTP_ADDR", defaultHTTPAddr)

	metrics, err := observability.NewProcessorMetrics()
	if err != nil {
		log.Fatalf("creating processor metrics: %v", err)
	}

	processor := &service{metrics: metrics}

	grpcServer := grpc.NewServer()
	transport.RegisterProcessorServiceServer(grpcServer, processor)

	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("listening gRPC on %s: %v", grpcAddr, err)
	}

	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/double", processor.handleHTTP)
	httpMux.Handle("/metrics", metrics.Handler())
	httpMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           httpMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errs := make(chan error, 2)

	go func() {
		log.Printf("processor gRPC listening on %s", grpcAddr)
		if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			errs <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	go func() {
		log.Printf("processor HTTP listening on %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- fmt.Errorf("http server: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutdown requested")
	case err := <-errs:
		log.Printf("processor exiting after error: %v", err)
	}

	grpcServer.GracefulStop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("stopping http server: %v", err)
	}
}

type service struct {
	transport.UnimplementedProcessorServiceServer
	metrics *observability.ProcessorMetrics
}

func (s *service) Double(ctx context.Context, payload *transport.Payload) (*transport.Payload, error) {
	phase := grpcPhase(ctx)
	startedAt := time.Now()
	response := doublePayload(payload)
	s.metrics.Observe("grpc", payload.GetScenario(), phase, time.Since(startedAt).Seconds(), observability.StatusOK)
	return response, nil
}

func (s *service) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var payload transport.Payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	startedAt := time.Now()
	response := doublePayload(&payload)
	s.metrics.Observe("http-json", payload.GetScenario(), httpPhase(r), time.Since(startedAt).Seconds(), observability.StatusOK)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "encoding response", http.StatusInternalServerError)
		return
	}
}

func doublePayload(payload *transport.Payload) *transport.Payload {
	if payload == nil {
		return &transport.Payload{}
	}

	cloned := protoClone(payload)
	cloned.Valor = payload.GetValor() * 2
	return cloned
}

func protoClone(payload *transport.Payload) *transport.Payload {
	cloned := &transport.Payload{
		Uuid:     payload.GetUuid(),
		Valor:    payload.GetValor(),
		Scenario: payload.GetScenario(),
		Blob:     append([]byte(nil), payload.GetBlob()...),
		Counters: append([]int64(nil), payload.GetCounters()...),
		Notes:    append([]string(nil), payload.GetNotes()...),
	}

	if customer := payload.GetCustomer(); customer != nil {
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

	if len(payload.GetItems()) > 0 {
		cloned.Items = make([]*transport.Item, 0, len(payload.GetItems()))
		for _, item := range payload.GetItems() {
			cloned.Items = append(cloned.Items, &transport.Item{
				Sku:       item.GetSku(),
				Name:      item.GetName(),
				Quantity:  item.GetQuantity(),
				UnitPrice: item.GetUnitPrice(),
				Tags:      append([]string(nil), item.GetTags()...),
			})
		}
	}

	if len(payload.GetAttributes()) > 0 {
		cloned.Attributes = make([]*transport.Attribute, 0, len(payload.GetAttributes()))
		for _, attribute := range payload.GetAttributes() {
			cloned.Attributes = append(cloned.Attributes, &transport.Attribute{
				Key:   attribute.GetKey(),
				Value: attribute.GetValue(),
			})
		}
	}

	return cloned
}

func httpPhase(r *http.Request) string {
	if r.Header.Get("X-Benchmark-Mode") == warmupFlag {
		return observability.PhaseWarmup
	}
	return observability.PhaseBenchmark
}

func grpcPhase(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return observability.PhaseBenchmark
	}
	if len(md.Get("benchmark-mode")) > 0 && md.Get("benchmark-mode")[0] == warmupFlag {
		return observability.PhaseWarmup
	}
	return observability.PhaseBenchmark
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
