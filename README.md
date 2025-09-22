## Cadastro de Empresas — Go + MongoDB + RabbitMQ + WebSocket

Este projeto consiste em um serviço RESTfull em GoLang desenvolvido para fins demonstrativos. O projeto possui cadastro de empresas, com persistência em MongoDB, publicação de eventos em RabbitMQ e um segundo serviço que consome esses eventos e os retransmitem via WebSocket (simulando um dashboard em tempo real).

O projeto possui logs de operações em fila, `stack Docker`, configuração por variáveis de ambiente, testes unitários e testes de integração.

---
### Sumário

* [Stack](#stack)

* [Estrutura do projeto](#estrutura-do-projeto)

* [Configuração (variáveis de ambiente)](#configuração-variáveis-de-ambiente---env)

* [Levantando o projeto com Docker Compose](#levantando-o-projeto-com-docker-compose)

* [Admin: seed de empresas](#admin-seed-de-empresas)

* [Rotas da API + cURLs](#rotas-da-api--curls)

* [Eventos (RabbitMQ)](#eventos-rabbitmq)

* [WebSocket / Hub](#websocket--hub)

* [Testes (unitários e integração)](#testes-unitários-e-integração)

* [Aderência ao 12-Factor](#aderência-ao-12-factor)

* [Dicas e troubleshooting](#dicas-e-troubleshooting)

* [Como o projeto atende ao desafio](#como-o-projeto-atende-ao-desafio)

---
### Stack

* Go 1.23

* MongoDB 7.x

* RabbitMQ 3.13 (com UI em :15672)

* Docker / Docker Compose

* WebSocket (serviço separado ws, retransmitindo os eventos da fila)

* Logs com slog estruturado (JSON)

---
### Estrutura do projeto
```
.
├── cmd/
│   ├── api/            # binário da API (main.go)
│   └── ws/             # binário do WebSocket consumer (main.go)
├── internal/
│   ├── admin/          # Seed Companies (Auto cadastro de emrpresas de teste)
│   ├── broker/         # Publisher RabbitMQ
│   ├── config/         # carregamento de env (Load), logger
│   ├── db/             # conexão Mongo
│   ├── handlers/       # HTTP handlers (Companies, CompanyByID, Health)
│   ├── models/         # Modelos (Company)
│   ├── repository/     # CompanyRepository (Mongo)
│   ├── utils/          # helpers (CNPJ, DecodeStrict, ComputeMinPCD, etc.)
│   └── ws/             # Hub (Broadcast/Unicast), cliente, etc.
├── docker/
│   ├── docker-compose.yml
│   ├── Dockerfile            # build de release da API
│   ├── Dockerfile.ws         # build de release do WS
│   ├── Dockerfile.test       # ambiente de testes (CI)
│   ├── dev-entrypoint.sh     # modo dev da API (live reload com Air)
│   └── run-tests.sh          # script que executa testes (unit e integração)
│
└── (go.mod, go.sum, .gitignore, .env)
```

---
### Nota sobre interfaces (Publisher)

Para desacoplar os handlers do transporte e facilitar testes, defini uma interface mínima que qualquer “publicador de eventos” deve cumprir:

```go
type Publisher interface {
    Publish(ctx context.Context, body string, headers amqp091.Table) error
    Close() error
}
```

* Vantagens:

  * body string: As mensagens são publicadas como JSON simples.

  * headers amqp091.Table: Possibilidade de enviar metadados do RabbitMQ (sem obrigar uso em todos os casos).

  * O broker.Publisher (RabbitMQ) implementa essa interface e é injetado nos handlers em runtime.

  * Nos testes unitários, trocamos a implementação real por um mock simples, permitindo inspecionar chamadas e evitar dependências externas.

---
### Configuração (variáveis de ambiente - .env)
<br>

> Importante utilizar o arquivo de configuração na raiz do projeto para carregar as configurações de ambiente. Neste caso, pode-se utilizar de exemplo o arquivo `.env-example`. Porém, o nome deve-se permancer como `.env`.


#### - Principais variáveis:


<b>API</b>

* `PORT` (padrão `8080`)

* `MONGO_URI` (padrão `mongodb://localhost:27017`, em Docker usar `mongodb://mongo:27017`)

* `MONGO_DB` (padrão `empresasdb`)
 
* `RABBITMQ_URL` (ex. `amqp://guest:guest@rabbitmq:5672/`)
 
* `RABBITMQ_QUEUE` (padrão `empresas_log`)
 
* `LOG_LEVEL` (`debug`, `info`, `warn`, `error`)
 
* `READ_HEADER_TIMEOUT` (ex. `5s`)

<b>WS</b>

* `WS_ADDR` (padrão :`8090`)
 
* `WS_ALLOWED_ORIGINS` (`*` por padrão)
 
* `RABBITMQ_URL`, `RABBITMQ_QUEUE` (iguais à API)

<br>


---
### Levantando o projeto com Docker Compose

> Requisitos: Docker e Docker Compose **instalados**.

<br>

> Importante: Rode todos os comandos "docker compose -f..." na `raiz do projeto`

<br>
a) Subir <b>Mongo + Rabbit + API + WS:</b>

```bash
# esse comando irá iniciar todos os containers em segundo plano (`-d`).

docker compose -f docker/docker-compose.yml up -d
```

b) Depois de subir a stack base, você pode conferir se a API pode começar a operar de fato, através dos logs da API:
```bash
# tudo certo quando aparecer uma linha como: "[HH:MM:SS] running..."

docker compose -f docker/docker-compose.yml logs -f api
```


<br>
c) Health Checker da API:


```bash
# status_code esperado 200, com o response {"status": "ok"}

curl --location 'http://localhost:8080/healthz'
```


<br>
c) Conectar ao WebSocket (precisa do ws rodando):

```bash
# instalar utilitário wscat se não tiver
# npm i -g wscat
wscat -c ws://localhost:8090/ws
```
<a id="container-ws"></a>


d )Se você quiser inspecionar as mensagens na fila (UI do RabbitMQ), deve-se **parar** o ser o serviço `ws` para o mesmo não consumi-las. Para parar o serviço do Web Socket, basta utilizar o comando:

```bash
docker compose -f docker/docker-compose.yml stop ws
```
<br>

e)Quando quiser voltar a ver os eventos em tempo real pelo WebSocket, suba o `ws` novamente:

```bash
docker compose -f docker/docker-compose.yml up -d ws
```

<br>
f) Abrir UI do RabbitMQ (Interface de gerenciamento do RabbitMQ ): 

```
http://localhost:15672 (user: guest, pass: guest)
```
---
### Admin: Seed de empresas

<br>
- Podemos cadastrar empresas de teste (12 configuradas) para auxiliar nossos testes, através de um processo que popula o Mongo a partir do arquivo `internal/admin/seeds/companies.json`. Este processo é idempotente, ou seja, pode ser executado várias vezes sem causar duplicações:


```bash 
docker compose -f docker/docker-compose.yml --profile admin up --build \
  --abort-on-container-exit --exit-code-from admin-seed admin-seed
```

<br>
- Podemos fazer um teste de validação do Mongo, fazendo uma contagem do número de documentos que temos no documento "empresas" até o momento, através do comando:

``` bash
docker compose -f docker/docker-compose.yml exec mongo \
  mongosh --quiet --eval 'db.getSiblingDB("empresasdb").companies.countDocuments()'
```

---
### Rotas da API + cURLs

`Domain`: http://localhost:8080

---

#### Listar empresas

Retorna a lista de empresas.

```
GET /api/companies
```

Exemplo de requisição:

```yaml
curl -s "http://localhost:8080/api/companies" | jq .
```

---

#### Listar empresas (com paginação)

```
GET /api/companies?limit=50&skip=0
```

Você pode usar os parâmetros de query `limit` e `skip` para controlar a quantidade de registros e a página.

- `limit`: Número de empresas a serem retornadas. Valor entre 1 e 200 (padrão: 50).
- `skip`: Número de empresas a serem puladas (padrão: 0).

Exemplo de requisição:

```yaml
curl -s "http://localhost:8080/api/companies?limit=10&skip=0" | jq .
```
---

#### Criar empresa - POST

Cria uma nova empresa na base de dados.

```
POST /api/companies
Content-Type: application/json
{
  "cnpj": "12.345.678/0001-90",
  "nome_fantasia": "ACME",
  "razao_social": "ACME Indústria LTDA",
  "endereco": "Rua A, 123",
  "numero_funcionarios": 180
}
```
O ID é o CNPJ sanitizado.

O CNPJ é validado. Caso seja duplicado, a resposta será 409 (Conflito).

Exemplo de requisição:

```ruby
curl -s -XPOST http://localhost:8080/api/companies \
  -H 'Content-Type: application/json' \
  -d '{"cnpj":"12.345.678/0001-90","nome_fantasia":"ACME","razao_social":"ACME Indústria LTDA","endereco":"Rua A, 123","numero_funcionarios":43}' \
  -w "\nStatus Code: %{http_code}\n" | jq .
```
---
#### Buscar por ID (CNPJ sanitizado) - GET


Busca uma empresa pelo CNPJ sanitizado.

```bash
GET /api/companies/{cnpj_sanitizado}
Exemplo de requisição:
```

Exemplo de requisição:

```bash
curl -s -i -w "\nStatus Code: %{http_code}\n" http://localhost:8080/api/companies/12345678000190
```

---
#### Atualização - PATCH
Atualização parcial (PATCH)
Realiza a atualização parcial de uma empresa. Os campos são opcionais.

```bash
PATCH /api/companies/{cnpj_sanitizado}
Content-Type: application/json
```
Se o campo numero_funcionarios for enviado, o Número Mínimo PCD será recalculado.
Exemplo de requisição:

```bash
curl --location --request PATCH 'http://localhost:8080/api/companies/12345678000190' \
--header 'Content-Type: application/json' \
--data '{ "numero_funcionarios": 520}' | jq .
```
---
#### Substituiação - PUT
Substituição completa (PUT)
Substitui completamente os dados de uma empresa. O documento anterior será totalmente substituído.
```bash
PUT /api/companies/{cnpj_sanitizado}
Content-Type: application/json
```
O CNPJ enviado no body deve ser igual ao CNPJ na URL (caso contrário, retornará 400).
O Número Mínimo PCD será recalculado.

Exemplo de requisição:

```rust
curl --location --request PUT 'http://localhost:8080/api/companies/12345678000190' \
--header 'Content-Type: application/json' \
--data '{
    "nome_fantasia": "Loja UYTR - Filial",
    "razao_social": "AATT Indústria LTDA",
    "endereco": "Av. Central, 512",
    "numero_funcionarios": 145
  }' | jq .
```
---
#### Remover - DELETE
* Remove uma empresa com base no CNPJ sanitizado.
* Caso sucesso, `o status code só retorna 204`


```bash
DELETE /api/companies/{cnpj_sanitizado}
```
Exemplo de requisição:

```bash
curl -s -o /dev/null -w "Status Code: %{http_code}\n" --location --request DELETE 'http://localhost:8080/api/companies/12345678000190'

```
---
<br>

### Eventos (RabbitMQ)

Cada operação realizada no sistema publica uma mensagem na fila configurada (`RABBITMQ_QUEUE`, padrão `empresas_log`), utilizando o default exchange e a routing key igual ao nome da fila.

Tipos de Eventos:

* Cadastro: "Cadastro da EMPRESA {NomeFantasia}"

* Edição: "Edição da EMPRESA {NomeFantasia}"

* Exclusão: "Exclusão da EMPRESA {NomeFantasia}"

A interface de gerenciamento do RabbitMQ pode ser acessada em http://localhost:15672
 com as credenciais usuário: guest e senha: guest.

Na UI, você poderá visualizar a fila `empresas_log` sendo constantemente alimentada com mensagens relacionadas às operações realizadas.

---
<br>

### WebSocket / Hub

* O serviço `ws` consome a fila do RabbitMQ e retransmite cada evento aos clientes conectados.

  * Endereço: `ws://localhost:8090/ws`

  * CORS/Origins: `WS_ALLOWED_ORIGINS` (padrão `*`)

* <u>API de mensagens</u>: O servidor envia para os clientes os eventos publicados na fila (formato JSON/string).
O Hub suporta `Broadcast` (todos os clientes) e `Unicast` (um cliente), mas para este caso, usamos Broadcast.


Exemplo de teste rápido (Importante o container ws estar "up", conforme explicado no tópico ["Levantando o projeto com Docker Compose" - letra (e)](#container-ws) ):
```bash
# com o container ws rodando, rode o comando abaixo:
wscat -c ws://localhost:8090/ws           # terminal 1 (cliente WS)
# em outro terminal, faça um POST/PUT/PATCH/DELETE na API
# você verá o evento aparecer no cliente WS
```
---

### Testes (unitários e integração)
Há <b>dois tipos</b> de testes no projeto:

* Unitários: rodam sem dependências externas.

* Integração: usam Testcontainers para subir MongoDB e RabbitMQ em contêineres durante os testes (exigem Docker em execução) e estão marcados com a build tag integration.

#### Rodar "os dois" localmente (Go direto - recomendado)

Executar os testes unitários e os de integração dentro do contêiner através do comando:

```golang 
go test -v -race -count=1 -tags=integration ./...
  ```

#### Rodar <b>apenas</b> unitários (sem integração):

```golang 
go test -v -race -count=1 ./...
```

#### Rodar apenas tesets de integração:
```golang 
# Repositório (Mongo)
go test -v -count=1 -tags=integration ./internal/repository

# Broker (Rabbit)
go test -v -count=1 -tags=integration ./internal/broker

```

#### Rodar um teste específico (pattern -run):
```golang 
# Handlers (unitário): só o teste de lista
go test -run TestCompanies_List -v ./internal/handlers -count=1
```

#### Outro exemplo teste específico - Função de PCD (unitário): teste dedicado
A função `ComputeMinPCD` implementa essa regra que atende a Lei 8.213/91 (Define percentuais mínimos de pessoas com deficiência conforme o número de empregados) com <b>arredondamento para cima</b>. 

```
Lei 8.213/91, art. 93:
  100–200 funcionários → 2%
  201–500 → 3%
  501–1000 → 4%
  1001+ → 5%
  Empresas com <100 não têm exigência mínima.
```

Para rodar o teste:

```golang 
go test -run TestComputeMinPCD -v ./internal/utils -count=1
```

<br>


---

<br>

### Aderência ao 12-Factor

* <b>Config:</b> utilização de variáveis de ambiente (sem arquivos fixos hardcoded).

* <b>Dependencies:</b> declaradas no go.mod, image de build reproduzível.

* <b>Logs:</b> estruturados em stdout com slog (JSON).

* <b>Processes:</b> processos stateless; nada gravado em disco local.

* <b>Disposability:</b> shutdown gracioso (graceful) na API; exponential backoff para conectar no Rabbit (evita crash loops).

* <b>Dev/Prod Parity:</b> Docker Compose para dev e prod-like; serviço ci para testes.

* <b>Admin tasks:</b> hook no binário via flags (-task seed) para rodar seed como “processo admin”.
---

<br>

### Dicas e troubleshooting

* <b>Health</b> a rota é /healthz (sem /api).

* <b>WebSocket ECONNREFUSED</b> garante que o ws está up e escutando em :8090 e conecte em ws://localhost:8090/ws.

* <b>404 em PUT mismatch:</b> o handler busca o recurso antes de validar mismatch do CNPJ; em testes unitários, mocke GetByID para retornar um documento, senão o fluxo devolve 404 antes do 400.

* <b>Tags de integração:</b> os testes de integração têm //go:build integration. Rode com -tags=integration.
---

<br>

### Revisão. O que o projeto oferece:

* **CRUD completo (create, read, update, delete)**:

Endpoints HTTP: POST /api/companies, GET /api/companies e /{id}, PATCH /{id}, PUT /{id}, DELETE /{id}.

Persistência em MongoDB. CNPJ sanitizado é usado como ID. Duplicidade de CNPJ retorna 409.

Todos os campos de cadastro são editáveis. No PUT, o cnpj do body deve bater com o {id}.

* **Número mínimo de PCD**:

  * Campo numero_minimo_pcd_exigidos calculado via ComputeMinPCD conforme Lei 8.213/91.

  * Recalculado em create, put e patch (quando numero_funcionarios muda).

  * Testes cobrindo o cálculo e os fluxos.

* **Mensagens de LOG em fila (RabbitMQ)**:

  * Publicação em fila empresas_log para Cadastro, Edição e Exclusão.

  * Mensagens simples e informativas (JSON/string).

  * Testes de integração com Rabbit (publica/consome).

* **Segundo serviço (WebSocket)**:

  * Serviço ws consome a fila e retransmite os eventos em tempo real via /ws.

  * Simula uma dashboard multi-cliente (Broadcast).

* **Stack Docker**:

  * docker-compose.yml sobe Mongo, Rabbit, API e WS.

  * Serviço admin-seed para popular dados de exemplo.

  * Serviço ci para rodar testes (unit + integração).

* **Desejável**:

  * Config por variáveis de ambiente e .env suportado.

  * Testes abrangendo unidade e integração.