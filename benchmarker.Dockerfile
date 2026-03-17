FROM golang:1.24-bookworm AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/benchmarker ./cmd/benchmarker

FROM debian:bookworm-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/benchmarker /app/benchmarker

ENV PROCESSOR_GRPC_ADDR=processor:50051
ENV PROCESSOR_HTTP_URL=http://processor:8080/double
ENV TOTAL_REQUESTS=25000
ENV WARMUP_REQUESTS=100
ENV BENCHMARK_RUNS=5
ENV CONCURRENCY=64
ENV HTTP_MAX_CONNS_PER_HOST=64
ENV SCENARIOS=medium-structured,large-structured,large-structured-headers

CMD ["/app/benchmarker"]
