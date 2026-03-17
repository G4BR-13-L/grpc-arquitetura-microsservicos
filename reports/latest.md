# Benchmark gRPC vs HTTP JSON

- Execucao: d3554873-72fd-4638-aca2-9291ae25a511
- Gerado em UTC: 2026-03-17T02:02:12Z
- Ordem de execucao: grpc,http-json
- Requests por protocolo por run: 25000
- Warmup por protocolo: 100
- Runs por protocolo: 5
- Concorrencia: 64
- HTTP max conns por host: 64
- Timeout por request: 20s

## Resumo de Cenarios

| Cenario | Proto bytes | JSON bytes | Headers bytes | Vencedor | Resumo |
| --- | --- | --- | --- | --- | --- |
| medium-structured | 11507 | 15713 | 0 | gRPC | gRPC foi 3.46x mais rapido na mediana do tempo total. |
| large-structured | 39203 | 52870 | 0 | gRPC | gRPC foi 3.59x mais rapido na mediana do tempo total. |
| large-structured-headers | 39211 | 52878 | 2424 | gRPC | gRPC foi 2.81x mais rapido na mediana do tempo total. |


## Notas

- Comparacao principal baseada no round-trip do cliente por mediana.
- O processor apenas dobra o valor e devolve o mesmo payload para reduzir logica de negocio no meio da medicao.
- gRPC usa protobuf precompilado e HTTP usa JSON sobre o mesmo modelo semantico.


## medium-structured

- Payload estruturado medio, sem headers extras.
- protobuf: 11507 bytes
- json: 15713 bytes
- headers extras: 0 bytes
- blob: 8192 bytes | items: 12 | attributes: 24 | counters: 96 | notes: 24

- protobuf ficou 1.37x menor que JSON (11507 bytes vs 15713 bytes).

| Protocolo | Mediana Total | Mediana Media | Mediana Min | Mediana Max | Mediana Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| gRPC | 998.402236ms | 2.529742ms | 64.292µs | 19.779208ms | 25040.01 |
| HTTP JSON | 3.456618502s | 8.780031ms | 335.792µs | 74.873667ms | 7232.50 |

### gRPC

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 3.785858252s | 9.660912ms | 1.472375ms | 65.492292ms | 6603.52 |
| 2 | 1.264735417s | 3.222241ms | 132.958µs | 19.193875ms | 19766.98 |
| 3 | 998.402236ms | 2.529742ms | 64.292µs | 49.242042ms | 25040.01 |
| 4 | 910.609001ms | 2.311151ms | 56.416µs | 19.779208ms | 27454.15 |
| 5 | 856.947584ms | 2.172398ms | 62.125µs | 8.945292ms | 29173.31 |

### HTTP JSON

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 3.948698168s | 10.032969ms | 360.333µs | 85.160083ms | 6331.20 |
| 2 | 3.472226168s | 8.807954ms | 335.792µs | 83.51875ms | 7199.99 |
| 3 | 3.456618502s | 8.780031ms | 320.333µs | 74.873667ms | 7232.50 |
| 4 | 3.370612127s | 8.562174ms | 337.542µs | 66.151625ms | 7417.05 |
| 5 | 3.295810918s | 8.37226ms | 316.625µs | 52.643333ms | 7585.39 |

## large-structured

- Payload estruturado grande, sem headers extras.
- protobuf: 39203 bytes
- json: 52870 bytes
- headers extras: 0 bytes
- blob: 32768 bytes | items: 24 | attributes: 48 | counters: 192 | notes: 48

- protobuf ficou 1.35x menor que JSON (39203 bytes vs 52870 bytes).

| Protocolo | Mediana Total | Mediana Media | Mediana Min | Mediana Max | Mediana Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| gRPC | 1.800829126s | 4.571226ms | 262.584µs | 29.27625ms | 13882.49 |
| HTTP JSON | 6.464085003s | 16.444196ms | 773.125µs | 114.678042ms | 3867.52 |

### gRPC

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 1.800829126s | 4.571226ms | 262.584µs | 42.8335ms | 13882.49 |
| 2 | 1.634559626s | 4.136251ms | 201µs | 29.27625ms | 15294.64 |
| 3 | 1.630924945s | 4.142271ms | 237.417µs | 23.446625ms | 15328.73 |
| 4 | 1.818468251s | 4.605072ms | 293.459µs | 28.8835ms | 13747.83 |
| 5 | 2.535890709s | 6.44802ms | 300.667µs | 80.454833ms | 9858.47 |

### HTTP JSON

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 6.401517169s | 16.284047ms | 620.541µs | 114.678042ms | 3905.32 |
| 2 | 6.280409503s | 15.980021ms | 695.125µs | 104.763584ms | 3980.63 |
| 3 | 6.464085003s | 16.444196ms | 773.125µs | 108.46125ms | 3867.52 |
| 4 | 6.875085837s | 17.489799ms | 795.333µs | 116.450375ms | 3636.32 |
| 5 | 6.86664317s | 17.465011ms | 793.208µs | 150.543041ms | 3640.79 |

## large-structured-headers

- Mesmo payload grande, adicionando headers/metadados extras nos dois protocolos.
- protobuf: 39211 bytes
- json: 52878 bytes
- headers extras: 2424 bytes
- blob: 32768 bytes | items: 24 | attributes: 48 | counters: 192 | notes: 48

- protobuf ficou 1.35x menor que JSON (39211 bytes vs 52878 bytes).

| Protocolo | Mediana Total | Mediana Media | Mediana Min | Mediana Max | Mediana Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| gRPC | 2.443821751s | 6.187607ms | 277.166µs | 37.307042ms | 10229.88 |
| HTTP JSON | 6.868494114s | 17.485577ms | 828.792µs | 121.757417ms | 3639.81 |

### gRPC

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 2.976138335s | 7.559001ms | 746.875µs | 37.307042ms | 8400.15 |
| 2 | 2.544251335s | 6.451884ms | 277.166µs | 27.085542ms | 9826.07 |
| 3 | 2.443821751s | 6.187607ms | 325.792µs | 41.487334ms | 10229.88 |
| 4 | 2.294938501s | 5.820864ms | 242.375µs | 26.042958ms | 10893.54 |
| 5 | 2.230188584s | 5.661418ms | 269.25µs | 38.345791ms | 11209.81 |

### HTTP JSON

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 7.64325499s | 19.439878ms | 817.459µs | 262.496125ms | 3270.86 |
| 2 | 6.679940711s | 16.980132ms | 850.875µs | 101.417167ms | 3742.55 |
| 3 | 6.607730128s | 16.807979ms | 796.709µs | 120.828083ms | 3783.45 |
| 4 | 6.868494114s | 17.485577ms | 828.792µs | 121.757417ms | 3639.81 |
| 5 | 7.891749504s | 20.068341ms | 952.458µs | 150.117375ms | 3167.87 |

