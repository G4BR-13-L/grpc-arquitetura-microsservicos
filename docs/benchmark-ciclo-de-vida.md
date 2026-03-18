# Ciclo de Vida do Benchmark gRPC vs HTTP JSON

## Objetivo deste documento

Este material explica o fluxo completo do projeto, do contrato `protobuf` ate a geracao de relatorios e dashboards.

Ele foi escrito para apresentacao tecnica. A ideia e mostrar:

- quais servicos existem
- como eles nascem
- como eles se comunicam
- o que exatamente e medido
- onde entram Prometheus, Grafana e os relatorios finais

## Visao geral da arquitetura

O projeto tem quatro pecas principais:

1. `proto/benchmark.proto`
   O contrato de dados e do servico gRPC.
2. `processor`
   O servidor que expoe os dois protocolos sobre a mesma logica de negocio.
3. `benchmarker`
   O cliente que gera carga, mede round-trip e produz relatorios.
4. `prometheus` + `grafana`
   A camada de observabilidade em tempo real.

Em termos práticos:

- o `processor` recebe um `Payload`, dobra o campo `valor` e devolve o mesmo objeto
- o `benchmarker` envia o mesmo cenario tanto por gRPC quanto por HTTP JSON
- o Prometheus raspa metricas dos servicos
- o Grafana exibe os paineis
- o `benchmarker` tambem grava o resultado consolidado em `reports/latest.json` e `reports/latest.md`

## 1. Contrato do sistema: o arquivo `.proto`

O ponto de partida do benchmark e o arquivo [proto/benchmark.proto](/Users/igor-pc/Documents/personal/grpc/proto/benchmark.proto).

Ele define:

- as mensagens `Address`, `Customer`, `Item`, `Attribute` e `Payload`
- o servico `ProcessorService`
- o metodo RPC `Double(Payload) returns (Payload)`

Isso garante que o modelo semantico do teste seja o mesmo para os dois protocolos.

O `Payload` foi desenhado para nao ser trivial. Ele inclui:

- campos escalares
- objeto aninhado
- listas
- mapa representado por lista de atributos
- `bytes`

Esse desenho ajuda a evidenciar o custo de serializacao entre `protobuf` e `JSON`.

## 2. Geracao do codigo Go a partir do `.proto`

O projeto ja versiona os arquivos gerados:

- [internal/transport/benchmark.pb.go](/Users/igor-pc/Documents/personal/grpc/internal/transport/benchmark.pb.go)
- [internal/transport/benchmark_grpc.pb.go](/Users/igor-pc/Documents/personal/grpc/internal/transport/benchmark_grpc.pb.go)

Esses arquivos sao a interface compilada do contrato.

Eles entregam duas coisas diferentes:

- `benchmark.pb.go`
  Estruturas Go para as mensagens protobuf e suporte de serializacao.
- `benchmark_grpc.pb.go`
  Interfaces e stubs do cliente e do servidor gRPC.

Pelos cabecalhos dos arquivos gerados, a versao usada foi:

- `protoc v6.33.0`
- `protoc-gen-go v1.36.8`
- `protoc-gen-go-grpc v1.5.1`

Um comando compativel com a geracao atual seria:

```bash
protoc \
  --proto_path=proto \
  --go_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_out=. \
  --go-grpc_opt=paths=source_relative \
  proto/benchmark.proto
```

Como o projeto guarda os arquivos gerados no repositorio, normalmente nao e preciso recompilar o `.proto` a cada execucao. Essa etapa passa a ser necessaria quando o contrato muda.

## 3. Papel de cada servico

### `processor`

Implementado em [cmd/processor/main.go](/Users/igor-pc/Documents/personal/grpc/cmd/processor/main.go).

Responsabilidades:

- subir um servidor gRPC em `:50051`
- subir um servidor HTTP em `:8080`
- expor `/metrics` e `/healthz` no servidor HTTP
- aplicar a mesma regra de negocio para os dois protocolos
- registrar metricas de processamento

Regra de negocio:

- clonar o payload recebido
- multiplicar `valor` por `2`
- devolver a resposta preservando o restante da estrutura

### `benchmarker`

Implementado em [cmd/benchmarker/main.go](/Users/igor-pc/Documents/personal/grpc/cmd/benchmarker/main.go).

Responsabilidades:

- montar os cenarios de teste
- aquecer o servidor
- executar as rodadas do benchmark
- medir round-trip no cliente
- expor metricas proprias em `:2112`
- consolidar resultados
- gerar relatorios Markdown e JSON

### `prometheus`

Configurado em [observability/prometheus/prometheus.yml](/Users/igor-pc/Documents/personal/grpc/observability/prometheus/prometheus.yml).

Responsabilidades:

- raspar as metricas do `processor`
- raspar as metricas do `benchmarker`
- armazenar series temporais para consulta

### `grafana`

Provisionado a partir de:

- [observability/grafana/provisioning/datasources/prometheus.yml](/Users/igor-pc/Documents/personal/grpc/observability/grafana/provisioning/datasources/prometheus.yml)
- [observability/grafana/provisioning/dashboards/dashboards.yml](/Users/igor-pc/Documents/personal/grpc/observability/grafana/provisioning/dashboards/dashboards.yml)
- [observability/grafana/dashboards/grpc-http-benchmark.json](/Users/igor-pc/Documents/personal/grpc/observability/grafana/dashboards/grpc-http-benchmark.json)

Responsabilidades:

- conectar no Prometheus
- carregar o dashboard provisionado automaticamente
- exibir throughput, latencia e metricas de processamento

## 4. Como os binarios sao construidos

O projeto usa Docker multi-stage.

### Build do `processor`

Arquivo: [processor.Dockerfile](/Users/igor-pc/Documents/personal/grpc/processor.Dockerfile)

Fluxo:

1. usa a imagem `golang:1.24-bookworm` para compilar
2. baixa dependencias com `go mod download`
3. faz `go build -o /out/processor ./cmd/processor`
4. copia o binario para uma imagem final `debian:bookworm-slim`
5. executa `/app/processor`

### Build do `benchmarker`

Arquivo: [benchmarker.Dockerfile](/Users/igor-pc/Documents/personal/grpc/benchmarker.Dockerfile)

Fluxo:

1. usa a imagem `golang:1.24-bookworm` para compilar
2. baixa dependencias com `go mod download`
3. faz `go build -o /out/benchmarker ./cmd/benchmarker`
4. copia o binario para `debian:bookworm-slim`
5. executa `/app/benchmarker`

## 5. Como a infraestrutura sobe com Docker Compose

Arquivo: [docker-compose.yml](/Users/igor-pc/Documents/personal/grpc/docker-compose.yml)

Servicos declarados:

- `processor`
- `benchmarker`
- `prometheus`
- `grafana`

O fluxo recomendado e:

```bash
docker compose up -d --build processor prometheus grafana
docker compose run --rm --service-ports --use-aliases benchmarker
```

### Por que o `benchmarker` roda separado

O `benchmarker` e um executor de carga, nao um servico de longa duracao.

Ele:

- sobe
- expõe metricas enquanto roda
- executa warmup e benchmark
- grava os relatorios
- espera o `METRICS_GRACE_PERIOD`
- encerra

O uso de `--service-ports --use-aliases` e importante para que:

- a porta `2112` fique exposta
- o alias de rede `benchmarker` exista
- o Prometheus consiga raspar `benchmarker:2112` durante a execucao

## 6. Ciclo de vida do `processor`

Quando o `processor` inicia, ele executa a seguinte sequencia:

1. cria um contexto com tratamento de `SIGINT` e `SIGTERM`
2. le `GRPC_ADDR` e `HTTP_ADDR`
3. cria o registro de metricas com `NewProcessorMetrics()`
4. instancia o servico de negocio
5. sobe o servidor gRPC
6. sobe o servidor HTTP
7. expõe:
   - `POST /double`
   - `GET /metrics`
   - `GET /healthz`
8. aguarda sinal de encerramento ou erro fatal
9. faz `GracefulStop()` no gRPC e `Shutdown()` no HTTP

### O que acontece em uma chamada gRPC

Fluxo:

1. o cliente chama `ProcessorService.Double`
2. o servidor identifica a fase via metadata:
   - `warmup`
   - `benchmark`
3. o payload e clonado
4. `valor` e multiplicado por `2`
5. a duracao de processamento e medida
6. a resposta volta ao cliente
7. a metrica `benchmark_processor_processing_seconds` e atualizada

### O que acontece em uma chamada HTTP

Fluxo:

1. o cliente envia `POST /double`
2. o corpo JSON e desserializado em `transport.Payload`
3. o servidor identifica a fase via header `X-Benchmark-Mode`
4. o payload e clonado
5. `valor` e multiplicado por `2`
6. a duracao de processamento e medida
7. a resposta e serializada como JSON
8. as metricas do `processor` sao atualizadas

## 7. Ciclo de vida do `benchmarker`

Quando o `benchmarker` inicia, ele executa uma sequencia mais longa.

### 7.1. Carregamento de configuracao

As configuracoes vem de variaveis de ambiente. As principais sao:

- `PROCESSOR_GRPC_ADDR`
- `PROCESSOR_HTTP_URL`
- `METRICS_ADDR`
- `METRICS_GRACE_PERIOD`
- `REPORTS_DIR`
- `BENCHMARK_ORDER`
- `REQUEST_TIMEOUT`
- `TOTAL_REQUESTS`
- `WARMUP_REQUESTS`
- `BENCHMARK_RUNS`
- `CONCURRENCY`
- `HTTP_MAX_CONNS_PER_HOST`
- `SCENARIOS`

O `benchmarker` tambem gera um `execution_id` unico para identificar a execucao.

### 7.2. Subida do endpoint de metricas

Antes de executar qualquer carga, o `benchmarker` sobe seu servidor HTTP interno:

- `GET /metrics`
- `GET /healthz`

Isso permite que o Prometheus acompanhe:

- total de requests por protocolo
- round-trip do cliente
- resumo do ultimo run finalizado

### 7.3. Preparacao de clientes

Depois disso ele:

1. cria um `http.Client` com limites de conexao configuraveis
2. cria um `grpc.ClientConn`
3. instancia `transport.NewProcessorServiceClient(conn)`

### 7.4. Health check do `processor`

Antes de medir, o `benchmarker` valida se o `processor` esta pronto:

- no gRPC, faz uma chamada real para `Double`
- no HTTP, chama `GET /healthz`

Se o servidor nao estiver saudavel, o benchmark e abortado.

## 8. Como os cenarios sao montados

Os cenarios sao definidos em [cmd/benchmarker/main.go](/Users/igor-pc/Documents/personal/grpc/cmd/benchmarker/main.go).

Atualmente existem tres cenarios built-in:

- `medium-structured`
- `large-structured`
- `large-structured-headers`

Cada cenario define:

- tamanho do `blob`
- quantidade de `items`
- quantidade de `attributes`
- quantidade de `counters`
- quantidade de `notes`
- headers extras opcionais

Durante a criacao do cenario, o codigo tambem calcula:

- tamanho do payload serializado em JSON
- tamanho do payload serializado em protobuf
- quantidade total de bytes extras em headers

Isso e importante porque o relatorio final nao mede apenas tempo. Ele tambem mostra o custo de serializacao de cada formato.

## 9. Warmup

Antes do benchmark principal, o sistema executa warmup.

Objetivo:

- aquecer conexoes
- reduzir ruido de primeira chamada
- estabilizar alocacoes iniciais

Fluxo do warmup:

1. para cada cenario
2. para cada protocolo na ordem configurada
3. envia `WARMUP_REQUESTS`
4. marca a fase como `warmup`
5. registra metricas, mas nao usa esses resultados no relatorio final

No gRPC, a fase e enviada em metadata `benchmark-mode=warmup`.

No HTTP, a fase e enviada no header `X-Benchmark-Mode: warmup`.

## 10. Benchmark principal

Depois do warmup, comeca a medicao real.

### Ordem de execucao

Para cada cenario:

1. o sistema executa `BENCHMARK_RUNS`
2. em cada run, segue `BENCHMARK_ORDER`
3. primeiro roda um protocolo inteiro
4. depois roda o outro protocolo inteiro

Importante: os protocolos nao rodam ao mesmo tempo entre si. O paralelismo acontece dentro de cada protocolo.

### Paralelismo interno

A funcao `runConcurrentBenchmark()` faz o trabalho de carga concorrente.

Ela:

1. calcula quantos workers usar com base em `CONCURRENCY`
2. cria um canal de jobs
3. sobe goroutines consumidoras
4. cada worker executa requests ate acabar a fila
5. consolida:
   - total de requests bem-sucedidas
   - tempo total do run
   - media
   - minimo
   - maximo

### O que exatamente e medido

No benchmark atual, a comparacao principal e baseada no round-trip do cliente.

Isso inclui:

- serializacao no cliente
- envio pela rede local
- processamento no servidor
- desserializacao da resposta

Nao inclui persistencia em banco no caminho principal da medicao.

Esse ponto e importante para a apresentacao: o benchmark foi desenhado para isolar a diferenca entre `gRPC + protobuf` e `HTTP + JSON`, nao para medir SQLite, filas ou logica de negocio pesada.

## 11. Validacao de resposta

Depois de cada chamada, o `benchmarker` valida se a resposta esta correta.

Ele verifica:

- `uuid`
- `scenario`
- `valor` dobrado
- quantidade de `items`
- quantidade de `attributes`
- quantidade de `counters`
- quantidade de `notes`
- tamanho do `blob`

Se a resposta estiver incorreta, o run falha.

Isso evita comparar desempenho em cima de uma implementacao quebrada.

## 12. Geração do relatorio

Ao final de todos os cenarios, o `benchmarker` monta um `benchmarkReport`.

Esse relatorio inclui:

- metadados da execucao
- ordem dos protocolos
- configuracao de concorrencia
- timeout
- resumo por cenario
- vencedor por cenario
- comparacao de tamanhos `protobuf` vs `JSON`
- mediana por protocolo
- detalhe de cada run

Os arquivos gerados sao:

- [reports/latest.json](/Users/igor-pc/Documents/personal/grpc/reports/latest.json)
- [reports/latest.md](/Users/igor-pc/Documents/personal/grpc/reports/latest.md)

### Como o vencedor e escolhido

O vencedor por cenario e definido comparando:

- `MedianTotal` do gRPC
- `MedianTotal` do HTTP JSON

Se o tempo mediano do gRPC for menor, ele vence. Se o do HTTP for menor, ele vence. Se forem iguais, o resultado e empate.

### Por que usar mediana

A mediana reduz o impacto de outliers.

Em benchmark concorrente local, outliers podem acontecer por:

- escalonamento do sistema operacional
- garbage collection
- aquecimento desigual
- variacao momentanea de CPU

## 13. Observabilidade: o que cada metrica representa

As metricas ficam em [internal/observability/metrics.go](/Users/igor-pc/Documents/personal/grpc/internal/observability/metrics.go).

### Metricas do `processor`

Principais series:

- `benchmark_processor_requests_total`
- `benchmark_processor_processing_seconds`

Elas representam:

- volume de requests processadas
- tempo interno de processamento do servidor

Essas metricas ajudam a separar uma pergunta importante:

"O servidor ficou lento, ou o custo maior esta no transporte/serializacao do cliente?"

### Metricas do `benchmarker`

Principais series:

- `benchmark_client_requests_total`
- `benchmark_client_round_trip_seconds`
- `benchmark_client_run_total_seconds`
- `benchmark_client_run_requests_total`
- `benchmark_client_run_completed_timestamp_seconds`

Elas representam:

- total de requests feitas pelo cliente
- latencia de round-trip por request
- resumo do ultimo run finalizado

## 14. Ciclo de observabilidade com Prometheus e Grafana

Fluxo:

1. `processor` expõe `/metrics`
2. `benchmarker` expõe `/metrics` enquanto roda
3. o Prometheus raspa ambos os alvos
4. o Grafana consulta o Prometheus
5. os paineis mostram throughput, latencia e processamento

Ponto importante para apresentacao:

As metricas do `benchmarker` so existem enquanto ele esta em execucao e durante o `METRICS_GRACE_PERIOD` ao final. Por isso o comando com `--service-ports --use-aliases` e relevante para que o Prometheus consiga enxergar o container efemero.

## 15. O que o SQLite representa hoje

Existe codigo em [internal/store/sqlite.go](/Users/igor-pc/Documents/personal/grpc/internal/store/sqlite.go), mas ele nao esta no caminho ativo do benchmark atual.

Isso e importante para a narrativa da apresentacao:

- houve uma fase anterior em que persistencia fazia parte do experimento
- no modelo atual, SQLite foi removido do caminho da medicao principal
- a comparacao ficou mais limpa e focada em transporte, serializacao e round-trip

## 16. Narrativa sugerida para apresentacao

Uma forma objetiva de apresentar o ciclo de vida e:

1. começamos pelo contrato `benchmark.proto`
2. geramos codigo Go para mensagens e stubs gRPC
3. subimos o `processor`, que expoe a mesma operacao em gRPC e HTTP
4. subimos Prometheus e Grafana para observabilidade
5. executamos o `benchmarker`, que cria cenarios estruturados
6. fazemos warmup para estabilizar o ambiente
7. rodamos os benchmarks com concorrencia configuravel
8. medimos round-trip no cliente e processamento no servidor
9. Prometheus coleta as series durante a execucao
10. Grafana mostra o comportamento em tempo real
11. ao final, o `benchmarker` gera relatorios consolidados em JSON e Markdown

## 17. Comandos uteis para demonstracao

### Subir infraestrutura base

```bash
docker compose up -d --build processor prometheus grafana
```

### Rodar benchmark padrao

```bash
docker compose run --rm --service-ports --use-aliases benchmarker
```

### Rodar benchmark com mais pressao concorrente

```bash
docker compose run --rm --service-ports --use-aliases \
  -e CONCURRENCY=256 \
  -e TOTAL_REQUESTS=50000 \
  benchmarker
```

### Inverter a ordem dos protocolos

```bash
docker compose run --rm --service-ports --use-aliases \
  -e BENCHMARK_ORDER=http-json,grpc \
  benchmarker
```

## 18. Resumo executivo

O ciclo de vida do sistema e:

1. definir contrato no `.proto`
2. gerar codigo Go
3. compilar `processor` e `benchmarker`
4. subir `processor`, Prometheus e Grafana
5. executar `benchmarker`
6. validar saude do servidor
7. aquecer o ambiente
8. executar cenarios e runs por protocolo
9. coletar metricas do cliente e do servidor
10. consolidar e publicar relatorios

Esse desenho torna a comparacao mais defensavel porque:

- usa o mesmo payload semantico
- usa a mesma regra de negocio
- separa latencia de cliente e processamento do servidor
- evita misturar banco de dados na medicao principal
- consolida o resultado por mediana e nao por uma unica rodada
