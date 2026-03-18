# Curiosidades sobre o `.proto` e como ele entra nas aplicacoes

## Objetivo deste material

Este documento complementa o guia principal e foca especificamente no arquivo `.proto`.

A ideia aqui e responder perguntas comuns em apresentacoes, como:

- o que exatamente um arquivo `.proto` faz
- por que ele existe mesmo quando tambem temos HTTP JSON
- como ele vira codigo Go
- como esse codigo entra no `processor`
- como esse codigo entra no `benchmarker`
- o que acontece em tempo de compilacao e em tempo de execucao

## 1. O que e um arquivo `.proto`

Um arquivo `.proto` e uma definicao de contrato.

No nosso caso, o contrato esta em [proto/benchmark.proto](/Users/igor-pc/Documents/personal/grpc/proto/benchmark.proto).

Ele define duas coisas:

1. o formato dos dados
2. a interface do servico gRPC

Ou seja, ele descreve:

- quais mensagens existem
- quais campos cada mensagem possui
- qual e o tipo de cada campo
- qual servico existe
- quais metodos esse servico expoe

No projeto atual, o trecho mais importante e:

```proto
service ProcessorService {
  rpc Double(Payload) returns (Payload);
}
```

Isso quer dizer:

- existe um servico chamado `ProcessorService`
- ele expoe um metodo remoto chamado `Double`
- esse metodo recebe um `Payload`
- ele devolve um `Payload`

## 2. O `.proto` nao e "codigo de negocio"

Esse ponto costuma gerar confusao.

O `.proto` nao implementa nada. Ele nao executa a regra "dobrar o valor". Ele nao sobe servidor. Ele nao chama rede.

Ele so descreve o contrato.

A implementacao real da regra de negocio esta em [cmd/processor/main.go](/Users/igor-pc/Documents/personal/grpc/cmd/processor/main.go), no metodo `Double`.

Em outras palavras:

- o `.proto` diz "como cliente e servidor devem conversar"
- o `processor` implementa "o que fazer quando a conversa acontecer"

## 3. Como os campos do `.proto` funcionam

Cada campo do protobuf tem:

- um nome
- um tipo
- um numero

Exemplo:

```proto
string uuid = 1;
int64 valor = 2;
string scenario = 3;
```

Esse numero do lado direito e muito importante.

Ele nao e apenas uma ordem visual. Ele faz parte do contrato binario do protobuf.

Na pratica:

- `uuid` usa o identificador `1`
- `valor` usa o identificador `2`
- `scenario` usa o identificador `3`

Isso permite que o protobuf serialize os dados de forma compacta.

### Por que os numeros existem

O protobuf foi desenhado para transmitir dados de forma binaria e eficiente.

Ele nao depende do nome textual do campo em tempo de transmissao. Em vez disso, usa os identificadores numericos.

Esse e um dos motivos pelos quais o payload protobuf costuma ser menor que o JSON.

## 4. Como o `proto3` simplifica o modelo

No topo do arquivo, temos:

```proto
syntax = "proto3";
```

Isso define a versao da linguagem protobuf usada no contrato.

No `proto3`, a modelagem costuma ser mais simples para interoperabilidade:

- campos sao opcionais por padrao
- valores zero sao tratados de forma implicita
- a serializacao tende a ser mais direta

Para o benchmark, isso ajuda porque reduz complexidade acidental e deixa o contrato mais previsivel.

## 5. O que faz o `go_package`

No arquivo `.proto`, existe esta linha:

```proto
option go_package = "grpcbenchmark/internal/transport;transport";
```

Ela diz ao gerador de codigo Go:

- qual caminho de import usar
- qual nome de pacote Go deve ser produzido

No nosso caso, isso resulta no pacote Go `transport`, localizado em:

- [internal/transport/benchmark.pb.go](/Users/igor-pc/Documents/personal/grpc/internal/transport/benchmark.pb.go)
- [internal/transport/benchmark_grpc.pb.go](/Users/igor-pc/Documents/personal/grpc/internal/transport/benchmark_grpc.pb.go)

Por isso o restante da aplicacao importa:

```go
import "grpcbenchmark/internal/transport"
```

## 6. Como o `.proto` foi compilado

Os cabecalhos dos arquivos gerados mostram as ferramentas usadas:

Em [benchmark.pb.go](/Users/igor-pc/Documents/personal/grpc/internal/transport/benchmark.pb.go):

- `protoc-gen-go v1.36.8`
- `protoc v6.33.0`

Em [benchmark_grpc.pb.go](/Users/igor-pc/Documents/personal/grpc/internal/transport/benchmark_grpc.pb.go):

- `protoc-gen-go-grpc v1.5.1`
- `protoc v6.33.0`

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

### O que esse comando faz

Ele executa dois geradores:

1. `protoc-gen-go`
   Gera as estruturas de mensagem e o suporte protobuf.
2. `protoc-gen-go-grpc`
   Gera interfaces e stubs de cliente/servidor gRPC.

## 7. O que nasce de cada arquivo gerado

### `benchmark.pb.go`

Esse arquivo contem:

- structs Go como `Payload`, `Customer`, `Item`
- getters como `GetUuid()`, `GetValor()`
- metadados de reflexao protobuf
- suporte a `proto.Marshal` e `proto.Unmarshal`

Na pratica, ele representa o modelo de dados compilado.

### `benchmark_grpc.pb.go`

Esse arquivo contem:

- a interface `ProcessorServiceClient`
- a interface `ProcessorServiceServer`
- a funcao `RegisterProcessorServiceServer`
- a funcao `NewProcessorServiceClient`
- constantes como `ProcessorService_Double_FullMethodName`

Na pratica, ele representa o contrato RPC compilado.

## 8. Como esse codigo entra no `processor`

O `processor` usa os arquivos gerados em dois niveis.

### 8.1. Como modelo de dados

Em [cmd/processor/main.go](/Users/igor-pc/Documents/personal/grpc/cmd/processor/main.go), o metodo:

```go
func (s *service) Double(ctx context.Context, payload *transport.Payload) (*transport.Payload, error)
```

recebe e devolve `*transport.Payload`.

Ou seja, o tipo do parametro e o tipo de resposta vieram diretamente do `.proto`.

### 8.2. Como contrato do servidor gRPC

O `processor` registra sua implementacao com:

```go
transport.RegisterProcessorServiceServer(grpcServer, processor)
```

Essa funcao foi gerada a partir do `.proto`.

Ela diz ao servidor gRPC:

- qual servico esta sendo registrado
- qual implementacao concreta atendera chamadas RPC

Tambem por isso a struct `service` embute:

```go
transport.UnimplementedProcessorServiceServer
```

Isso segue o contrato esperado pelo codigo gerado.

## 9. Como esse codigo entra no `benchmarker`

O `benchmarker` usa o `.proto` compilado de duas formas principais.

### 9.1. Como cliente gRPC

Em [cmd/benchmarker/main.go](/Users/igor-pc/Documents/personal/grpc/cmd/benchmarker/main.go), ele cria uma conexao:

```go
conn, err := grpc.NewClient(...)
```

e depois constroi o client:

```go
grpcClient := transport.NewProcessorServiceClient(conn)
```

Essa funcao tambem veio do arquivo gerado `benchmark_grpc.pb.go`.

Ela encapsula a chamada remota para:

- serializar a request protobuf
- invocar o metodo remoto
- desserializar a resposta protobuf

### 9.2. Como estrutura de payload

O `benchmarker` cria mensagens usando `transport.Payload`, `transport.Customer`, `transport.Item` e demais tipos gerados.

Ou seja:

- o cenario e montado com structs Go originadas do `.proto`
- o mesmo modelo serve tanto para o caminho gRPC quanto para o caminho HTTP

Isso e uma curiosidade importante do projeto:

mesmo no benchmark HTTP JSON, o modelo base ainda nasce do `.proto`, porque ele foi convertido em structs Go reutilizaveis.

## 10. Como o mesmo `.proto` ajuda ate no caminho HTTP JSON

Pode parecer estranho usar `.proto` em um benchmark que compara gRPC com HTTP JSON, mas isso e intencional.

O `.proto` nao serve apenas ao transporte gRPC. Aqui ele tambem ajuda a manter um unico modelo semantico.

Na pratica:

- gRPC usa o payload protobuf diretamente
- HTTP usa o mesmo struct Go serializado como JSON

Isso reduz um risco comum em benchmarks: comparar dois protocolos usando modelos diferentes sem perceber.

Com essa abordagem:

- os campos sao os mesmos
- a estrutura semantica e a mesma
- o servidor processa a mesma informacao

O que muda e o protocolo e a serializacao, nao a regra de negocio.

## 11. O `.proto` entra em que momento do ciclo

O ciclo completo do `.proto` no projeto e:

1. o desenvolvedor escreve ou altera [proto/benchmark.proto](/Users/igor-pc/Documents/personal/grpc/proto/benchmark.proto)
2. roda `protoc` com os plugins Go
3. os arquivos gerados aparecem em `internal/transport/`
4. o codigo do `processor` e do `benchmarker` importa `transport`
5. `go build` compila tudo junto nos binarios
6. os containers executam esses binarios
7. em runtime, o gRPC usa os stubs e mensagens gerados para falar com seguranca de tipos

O ponto importante e:

o `.proto` em si nao vai para producao como arquivo "interpretado". O que vai para producao sao os binarios Go ja compilados com o codigo gerado embutido.

## 12. O que acontece em tempo de execucao no gRPC

Quando o `benchmarker` chama:

```go
grpcClient.Double(...)
```

acontece, conceitualmente, o seguinte:

1. o client gerado recebe um `transport.Payload`
2. a biblioteca gRPC usa o codec protobuf
3. a mensagem e serializada em binario
4. os bytes sao enviados pela conexao HTTP/2
5. o servidor gRPC despacha a chamada para a implementacao registrada
6. o `processor` executa `Double`
7. a resposta e serializada em protobuf
8. o client reconstrói um `transport.Payload`

Boa parte desse encadeamento so funciona de forma transparente porque o `.proto` foi compilado antes.

## 13. O que acontece em tempo de execucao no HTTP JSON

No HTTP JSON, o `.proto` participa de forma diferente.

Fluxo:

1. o `benchmarker` constroi um `transport.Payload`
2. o Go converte esse struct para JSON com `json.Marshal`
3. o `processor` recebe o JSON em `POST /double`
4. o Go converte o JSON de volta para `transport.Payload`
5. o `processor` processa a request
6. a resposta e serializada novamente em JSON

Ou seja:

- no gRPC, o `.proto` define contrato e serializacao
- no HTTP JSON, o `.proto` continua definindo o modelo, mas a serializacao passa a ser JSON

## 14. Curiosidade importante: compatibilidade e evolucao

Uma das vantagens do protobuf e a evolucao de contrato.

Como cada campo tem numero proprio, e possivel evoluir mensagens com mais seguranca, desde que as mudancas respeitem as regras de compatibilidade.

Em termos gerais:

- adicionar novos campos costuma ser seguro
- reaproveitar numero de campo antigo costuma ser perigoso
- renomear um campo e menos critico que mudar seu numero

Mesmo que o benchmark atual seja simples, isso ajuda na apresentacao para mostrar por que protobuf e popular em sistemas distribuidos.

## 15. Curiosidade importante: o `.proto` nao elimina o HTTP

Outro ponto bom para explicar e:

usar `.proto` nao significa que o sistema e obrigado a ser "so gRPC".

Neste projeto:

- o contrato protobuf e a fonte primaria do modelo
- o gRPC usa esse contrato de forma nativa
- o HTTP JSON reutiliza o mesmo modelo compilado em Go

Entao o `.proto` aqui ajuda a padronizar o dominio, nao apenas a camada RPC.

## 16. Curiosidade importante: por que versionar os arquivos gerados

Este repositorio guarda `benchmark.pb.go` e `benchmark_grpc.pb.go`.

Isso traz algumas vantagens:

- qualquer pessoa consegue compilar o projeto sem depender de `protoc` logo de cara
- o pipeline de build fica mais simples
- a revisao do contrato compilado pode acontecer via Git

Mas tambem traz responsabilidade:

- se o `.proto` mudar, os arquivos gerados precisam ser atualizados junto

## 17. Explicacao curta para apresentar em voz alta

Se quiser uma versao curta para fala, pode usar algo assim:

"O arquivo `.proto` e o contrato central do sistema. Ele define tanto o formato do payload quanto o metodo RPC `Double`. A partir dele, usamos o `protoc` para gerar dois arquivos Go: um com as estruturas de mensagem e outro com os stubs gRPC. Esses arquivos entram no `processor`, que implementa o servidor, e no `benchmarker`, que atua como cliente. No caminho gRPC, esse contrato tambem dirige a serializacao protobuf. No caminho HTTP JSON, ele continua sendo reutilizado como modelo de dados, o que garante que estamos comparando os mesmos objetos nos dois protocolos."

## 18. Resumo final

O `.proto` participa do projeto em quatro camadas:

1. modelagem
   Define mensagens e servicos.
2. geracao de codigo
   Vira structs Go e stubs gRPC.
3. compilacao
   Entra nos binarios do `processor` e do `benchmarker`.
4. execucao
   Sustenta o fluxo gRPC e padroniza o modelo usado tambem no HTTP JSON.

Sem esse contrato unico, a comparacao entre gRPC e HTTP JSON seria menos confiavel.
