# Benchmark gRPC vs HTTP JSON

Projeto em Go para comparar `gRPC + protobuf` contra `HTTP REST + JSON` no mesmo servidor, com os mesmos cenarios de carga.

## Objetivo

O benchmark foi refatorado para evidenciar melhor as vantagens do gRPC em cenarios onde elas costumam aparecer:

- payload estruturado, com campos aninhados, listas e `bytes`
- multiplos cenarios de tamanho
- execucao concorrente
- comparacao por mediana de varios runs
- sem SQLite no caminho da medicao

## Como funciona

- `processor`: expõe `gRPC` em `:50051` e `HTTP` em `:8080`
- `benchmarker`: executa os mesmos cenarios nos dois protocolos e mede apenas o round-trip do cliente
- `prometheus` e `grafana`: observabilidade em tempo real

O servidor apenas dobra o campo `valor` e devolve o restante do payload intacto.

## Payload

O modelo usado no benchmark fica em [benchmark.proto](/Users/igor-pc/Documents/personal/grpc/proto/benchmark.proto) e já está pré-compilado em:

- [benchmark.pb.go](/Users/igor-pc/Documents/personal/grpc/internal/transport/benchmark.pb.go)
- [benchmark_grpc.pb.go](/Users/igor-pc/Documents/personal/grpc/internal/transport/benchmark_grpc.pb.go)

Ele inclui:

- `uuid`
- `valor`
- `scenario`
- `customer` com `address`
- `items`
- `attributes`
- `counters`
- `notes`
- `blob`

## Cenarios

Por padrao o benchmark roda:

- `medium-structured`
- `large-structured`
- `large-structured-headers`

Cada cenario compara:

- tamanho serializado em `protobuf`
- tamanho serializado em `JSON`
- mediana do tempo total por protocolo
- throughput mediano
- efeito de headers extras quando configurados

## Execucao

Suba a infraestrutura:

```bash
docker compose up -d --build processor prometheus grafana
```

Rode o benchmark:

```bash
docker compose run --rm benchmarker
```

## Relatorios

Ao final da execucao:

- `reports/latest.md`
- `reports/latest.json`

O relatorio traz:

- resumo por cenario
- vencedor por cenario
- tamanho `protobuf` vs `JSON`
- mediana, media, min, max e throughput por protocolo
- detalhe de cada run

## Variaveis importantes

- `TOTAL_REQUESTS`: requests por protocolo em cada run
- `BENCHMARK_RUNS`: quantidade de runs por protocolo
- `CONCURRENCY`: workers simultaneos por protocolo
- `HTTP_MAX_CONNS_PER_HOST`: limite de conexoes HTTP simultaneas
- `SCENARIOS`: lista de cenarios separados por virgula
- `REQUEST_TIMEOUT`: timeout por request

## Observabilidade

- Grafana: `http://localhost:3000`
- Prometheus: `http://localhost:9090`

O dashboard provisionado mostra as metricas por `cenario` e `protocolo`.

## Leitura correta do resultado

Este benchmark e mais fiel do que o modelo anterior porque:

- nao mistura persistencia com medicao
- nao usa payload minimo
- compara o mesmo modelo semantico em `protobuf` e `JSON`
- mostra o ganho de serializacao alem do tempo total

Mesmo assim, ele continua sendo um benchmark local, unary e sem TLS. Isso significa que os numeros valem para este contexto, nao como verdade universal sobre todos os sistemas.
