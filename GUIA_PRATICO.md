# Guia Pratico

## Subir a infraestrutura

```bash
docker compose up -d --build processor prometheus grafana
```

## Rodar o benchmark

```bash
docker compose run --rm --service-ports --use-aliases benchmarker
```

## Rodar um cenario especifico

```bash
docker compose run --rm --service-ports --use-aliases -e SCENARIOS=large-structured benchmarker
```

## Rodar com mais carga

```bash
docker compose run --rm --service-ports --use-aliases -e TOTAL_REQUESTS=50000 -e CONCURRENCY=128 benchmarker
```

## Rodar apenas um protocolo

HTTP JSON:

```bash
docker compose run --rm --service-ports --use-aliases -e BENCHMARK_ORDER=http-json benchmarker
```

gRPC:

```bash
docker compose run --rm --service-ports --use-aliases -e BENCHMARK_ORDER=grpc benchmarker
```

## Relatorio gerado

Depois do benchmark:

- `reports/latest.md`
- `reports/latest.json`

## O que olhar no relatorio

1. `Proto bytes` vs `JSON bytes`
2. vencedor por cenario
3. mediana total
4. throughput mediano
5. diferenca entre `large-structured` e `large-structured-headers`

## Grafana

- `http://localhost:3000`
- usuario: `admin`
- senha: `admin`

O dashboard agora mostra series por:

- `protocol`
- `scenario`

## Resetar tudo

```bash
docker compose down -v
docker compose up -d --build processor prometheus grafana
```
