# Benchmark gRPC vs HTTP JSON

- Execucao: acd787c8-a027-4922-973d-d550272f0710
- Gerado em UTC: 2026-03-18T00:08:22Z
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
| medium-structured | 11507 | 15713 | 0 | gRPC | gRPC foi 3.60x mais rapido na mediana do tempo total. |
| large-structured | 39203 | 52870 | 0 | gRPC | gRPC foi 3.82x mais rapido na mediana do tempo total. |
| large-structured-headers | 39211 | 52878 | 2424 | gRPC | gRPC foi 3.34x mais rapido na mediana do tempo total. |


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
| gRPC | 915.912875ms | 2.320211ms | 72.792µs | 14.8585ms | 27295.17 |
| HTTP JSON | 3.30108096s | 8.375253ms | 326.542µs | 63.743334ms | 7573.28 |

### gRPC

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 1.153271542s | 2.925791ms | 91.542µs | 15.30275ms | 21677.46 |
| 2 | 967.674626ms | 2.442813ms | 59.75µs | 14.8585ms | 25835.13 |
| 3 | 915.912875ms | 2.320211ms | 73.834µs | 15.558625ms | 27295.17 |
| 4 | 881.534376ms | 2.230205ms | 62.208µs | 10.691792ms | 28359.64 |
| 5 | 865.929876ms | 2.194685ms | 72.792µs | 10.89875ms | 28870.70 |

### HTTP JSON

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 4.296416946s | 10.897739ms | 373.75µs | 96.921667ms | 5818.80 |
| 2 | 3.48828946s | 8.858196ms | 317.541µs | 88.200875ms | 7166.84 |
| 3 | 3.25597321s | 8.264889ms | 330µs | 47.731042ms | 7678.20 |
| 4 | 3.20712821s | 8.138519ms | 283.833µs | 45.029208ms | 7795.14 |
| 5 | 3.30108096s | 8.375253ms | 326.542µs | 63.743334ms | 7573.28 |

## large-structured

- Payload estruturado grande, sem headers extras.
- protobuf: 39203 bytes
- json: 52870 bytes
- headers extras: 0 bytes
- blob: 32768 bytes | items: 24 | attributes: 48 | counters: 192 | notes: 48

- protobuf ficou 1.35x menor que JSON (39203 bytes vs 52870 bytes).

| Protocolo | Mediana Total | Mediana Media | Mediana Min | Mediana Max | Mediana Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| gRPC | 1.739463959s | 4.396661ms | 236.709µs | 24.524584ms | 14372.24 |
| HTTP JSON | 6.649550753s | 16.921986ms | 705µs | 126.916708ms | 3759.65 |

### gRPC

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 1.725695792s | 4.369698ms | 212.667µs | 22.880208ms | 14486.91 |
| 2 | 1.663320834s | 4.217538ms | 236.709µs | 20.201542ms | 15030.17 |
| 3 | 1.739463959s | 4.396661ms | 296.292µs | 24.524584ms | 14372.24 |
| 4 | 1.7599s | 4.438708ms | 214.5µs | 48.106083ms | 14205.35 |
| 5 | 1.753342167s | 4.443717ms | 305.959µs | 26.512375ms | 14258.48 |

### HTTP JSON

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 6.170822212s | 15.693101ms | 705µs | 138.638833ms | 4051.32 |
| 2 | 6.306598238s | 16.04843ms | 739.041µs | 94.655ms | 3964.10 |
| 3 | 7.206638587s | 18.334507ms | 684.625µs | 261.169709ms | 3469.02 |
| 4 | 6.649550753s | 16.921986ms | 690.917µs | 116.227917ms | 3759.65 |
| 5 | 7.062897462s | 17.97192ms | 767.167µs | 126.916708ms | 3539.62 |

## large-structured-headers

- Mesmo payload grande, adicionando headers/metadados extras nos dois protocolos.
- protobuf: 39211 bytes
- json: 52878 bytes
- headers extras: 2424 bytes
- blob: 32768 bytes | items: 24 | attributes: 48 | counters: 192 | notes: 48

- protobuf ficou 1.35x menor que JSON (39211 bytes vs 52878 bytes).

| Protocolo | Mediana Total | Mediana Media | Mediana Min | Mediana Max | Mediana Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| gRPC | 2.276874126s | 5.776753ms | 347.417µs | 33.176916ms | 10979.97 |
| HTTP JSON | 7.609663129s | 19.357365ms | 922.292µs | 152.542292ms | 3285.30 |

### gRPC

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 2.247106237s | 5.691109ms | 273.083µs | 27.164917ms | 11125.42 |
| 2 | 2.263497168s | 5.747816ms | 336.458µs | 27.754625ms | 11044.86 |
| 3 | 2.276874126s | 5.776753ms | 347.417µs | 40.598375ms | 10979.97 |
| 4 | 2.598052403s | 6.594903ms | 493.625µs | 46.270083ms | 9622.59 |
| 5 | 2.50535996s | 6.346459ms | 503.666µs | 33.176916ms | 9978.61 |

### HTTP JSON

| Run | Total | Media | Min | Max | Throughput req/s |
| --- | --- | --- | --- | --- | --- |
| 1 | 7.609663129s | 19.357365ms | 763.125µs | 139.837541ms | 3285.30 |
| 2 | 7.762210712s | 19.723851ms | 922.458µs | 177.211959ms | 3220.73 |
| 3 | 8.136717337s | 20.703751ms | 934µs | 159.405625ms | 3072.49 |
| 4 | 7.29779692s | 18.560296ms | 922.292µs | 152.542292ms | 3425.69 |
| 5 | 7.47302092s | 19.007605ms | 894.709µs | 123.025042ms | 3345.37 |

