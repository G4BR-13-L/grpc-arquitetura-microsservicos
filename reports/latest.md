# Benchmark gRPC vs HTTP JSON

- Execucao: 23a655e4-6858-43fa-91d4-be0f2cda4dff
- Gerado em UTC: 2026-03-18T00:23:38Z
- Ordem de execucao: http-json,grpc
- Requests por protocolo por run: 15000
- Warmup por protocolo: 100
- Runs por protocolo: 5
- Concorrencia: 256
- HTTP max conns por host: 64
- Timeout por request: 20s

## Resumo de Cenarios

| Cenario | Proto bytes | JSON bytes | Headers bytes | Vencedor | Resumo |
| --- | --- | --- | --- | --- | --- |
| medium-structured | 11507 | 15713 | 0 | gRPC | gRPC foi 3.72x mais rapido na mediana do tempo total. |
| large-structured | 39203 | 52870 | 0 | gRPC | gRPC foi 3.16x mais rapido na mediana do tempo total. |
| large-structured-headers | 39211 | 52878 | 2424 | gRPC | gRPC foi 3.15x mais rapido na mediana do tempo total. |


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
| gRPC | 590.074833ms | 9.927476ms | 1.061667ms | 36.970916ms | 25420.50 |
| HTTP JSON | 2.197355959s | 37.103413ms | 3.52325ms | 107.1935ms | 6826.39 |

### gRPC

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 811.146376ms | 13.754632ms | 576.958µs | 58.341375ms | 18492.35 |
| 2 | 590.074833ms | 9.927476ms | 464.292µs | 27.619916ms | 25420.50 |
| 3 | 511.011834ms | 8.642395ms | 2.505791ms | 24.522ms | 29353.53 |
| 4 | 698.345792ms | 11.717534ms | 2.083125ms | 86.542875ms | 21479.33 |
| 5 | 552.361083ms | 9.279519ms | 1.061667ms | 36.970916ms | 27156.15 |

### HTTP JSON

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 2.197355959s | 37.103413ms | 2.060583ms | 107.1935ms | 6826.39 |
| 2 | 2.193577085s | 37.026566ms | 3.949875ms | 91.128041ms | 6838.15 |
| 3 | 2.660516168s | 44.889315ms | 3.52325ms | 256.198709ms | 5638.00 |
| 4 | 1.984128209s | 33.486792ms | 2.873083ms | 223.647542ms | 7560.00 |
| 5 | 2.245617126s | 37.889136ms | 17.248792ms | 106.003625ms | 6679.68 |

## large-structured

- Payload estruturado grande, sem headers extras.
- protobuf: 39203 bytes
- json: 52870 bytes
- headers extras: 0 bytes
- blob: 32768 bytes | items: 24 | attributes: 48 | counters: 192 | notes: 48

- protobuf ficou 1.35x menor que JSON (39203 bytes vs 52870 bytes).

| Protocolo | Mediana Total | Mediana Media | Mediana Min | Mediana Max | Mediana Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| gRPC | 1.295981959s | 21.742854ms | 7.578ms | 71.942625ms | 11574.24 |
| HTTP JSON | 4.090798211s | 69.198974ms | 5.167583ms | 212.156ms | 3666.77 |

### gRPC

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 2.208586071s | 37.288937ms | 7.578ms | 282.019584ms | 6791.68 |
| 2 | 1.781055709s | 30.093108ms | 12.040833ms | 133.00075ms | 8421.97 |
| 3 | 1.216868459s | 20.324188ms | 3.371875ms | 55.633125ms | 12326.72 |
| 4 | 1.295981959s | 21.742854ms | 5.626792ms | 71.942625ms | 11574.24 |
| 5 | 1.066741042s | 17.78278ms | 9.080875ms | 59.142875ms | 14061.52 |

### HTTP JSON

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 4.21084946s | 71.27144ms | 8.310209ms | 269.954876ms | 3562.23 |
| 2 | 4.346278127s | 73.420477ms | 6.266875ms | 247.636125ms | 3451.23 |
| 3 | 4.090798211s | 69.198974ms | 5.167583ms | 188.780958ms | 3666.77 |
| 4 | 3.375051793s | 57.122101ms | 3.449667ms | 212.156ms | 4444.38 |
| 5 | 3.416576835s | 57.760284ms | 2.461625ms | 159.511834ms | 4390.36 |

## large-structured-headers

- Mesmo payload grande, adicionando headers/metadados extras nos dois protocolos.
- protobuf: 39211 bytes
- json: 52878 bytes
- headers extras: 2424 bytes
- blob: 32768 bytes | items: 24 | attributes: 48 | counters: 192 | notes: 48

- protobuf ficou 1.35x menor que JSON (39211 bytes vs 52878 bytes).

| Protocolo | Mediana Total | Mediana Media | Mediana Min | Mediana Max | Mediana Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| gRPC | 1.11964525s | 18.890278ms | 8.846625ms | 60.414916ms | 13397.10 |
| HTTP JSON | 3.530624668s | 59.694306ms | 4.325875ms | 171.140292ms | 4248.54 |

### gRPC

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 1.241975459s | 20.81284ms | 6.123625ms | 61.350916ms | 12077.53 |
| 2 | 1.11964525s | 18.890278ms | 2.829666ms | 70.449083ms | 13397.10 |
| 3 | 1.078065293s | 18.155521ms | 9.845917ms | 48.203667ms | 13913.81 |
| 4 | 1.096579459s | 18.339607ms | 8.846625ms | 49.43575ms | 13678.90 |
| 5 | 1.216428001s | 20.459485ms | 11.710458ms | 60.414916ms | 12331.19 |

### HTTP JSON

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 3.550525668s | 59.990897ms | 4.468375ms | 232.690167ms | 4224.73 |
| 2 | 3.35775532s | 56.488288ms | 4.325875ms | 147.327375ms | 4467.27 |
| 3 | 3.42953296s | 57.942525ms | 6.782708ms | 151.069083ms | 4373.77 |
| 4 | 3.589533044s | 60.757696ms | 2.303333ms | 188.739667ms | 4178.82 |
| 5 | 3.530624668s | 59.694306ms | 3.171917ms | 171.140292ms | 4248.54 |

