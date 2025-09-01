## Cadastro de Empresas — Go + MongoDB + RabbitMQ + WebSocket

Este projeto consiste em um serviço RESTful em GoLang para cadastro de empresas, com persistência em MongoDB, publicação de eventos em RabbitMQ e um segundo serviço que consome esses eventos e os retransmite via WebSocket (simulando uma dashboard em tempo real).

O projeto atende ao enunciado do desafio (CRUD completo, logs de operações em fila, stack Docker, configuração por variáveis de ambiente, testes e boa aderência ao 12-Factor). A seção “Como o projeto atende ao desafio” está no fim do arquivo.

---
### Sumário

* [Stack](#stack)

* [Estrutura do projeto](#estrutura-do-projeto)

* [Configuração (variáveis de ambiente)](#configuração-variáveis-de-ambiente)

* [Subindo com Docker Compose](#subindo-com-docker-compose)

* [Admin: seed de empresas](#admin-seed-de-empresas)

* [Rotas da API + cURLs](#rotas-da-api--curls)

* [Eventos (RabbitMQ)](#eventos-rabbitmq)

* [WebSocket / Hub](#websocket--hub)

* [Testes (unitários e integração)](#testes-unitários-e-integração)

* [Lei 8.213/91 (PCD)](#lei-821391-pcd)

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

Para facilitar testes, o handlers.Publisher foi definido para permitir mocks:

```go
type Publisher interface {
    Publish(ctx context.Context, body string, headers amqp091.Table) error
    Close() error
}
```


O `broker.Publisher` (Rabbit) implementa essa interface.

Em testes, usamos um `pubMock`.

No modo “admin seed” evitamos publicar (seed não depende de Rabbit).

---
### Configuração (variáveis de ambiente - .env)

Principais variáveis:

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

> <b>OBS:</b> Você pode usar um `.env na raiz para facilitar. Neste caso, pode usar de exemplo o arquivo `.env-example`

---
### Subindo com Docker Compose

> Requisitos: Docker e Docker Compose <u>instalados</u>.

<br>
Subir <b>Mongo + Rabbit + API + WS:</b>

```
docker compose -f docker/docker-compose.yml up -d
```
<br>
Checar health da API:

```
curl -i http://localhost:8080/healthz
```


Abrir UI do RabbitMQ: http://localhost:15672 (user: guest, pass: guest)

<br>
Conectar ao WebSocket (precisa do ws rodando):

```bash
# instalar utilitário wscat se não tiver
# npm i -g wscat
wscat -c ws://localhost:8090/ws
```

---
### Admin: Seed de empresas

<br>

Roda um job que insere empresas de exemplo a partir de `internal/admin/seeds/companies.json` (idempotente).

```
docker compose -f docker/docker-compose.yml --profile admin up --build \
  --abort-on-container-exit --exit-code-from admin-seed admin-seed
```

<br>
Validar no Mongo:

```
docker compose -f docker/docker-compose.yml exec mongo \
  mongosh --quiet --eval 'db.getSiblingDB("empresasdb").empresas.countDocuments()'
```

---
### Rotas da API + cURLs

Base: http://localhost:8080

<br>
<b>Health</b>

```
GET /healthz
```

<br>
<b>Listar empresas</b>

```
GET /api/companies
```

<br>
<b>Listar empresas (com paginação)</b>

```
GET /api/companies?limit=50&skip=0
```


`limit` ∈ [1..200] (default 50 se inválido/fora do range)

`skip` ≥ 0 (default 0 se inválido)

Exemplo:
```
curl -s "http://localhost:8080/api/companies?limit=10&skip=0"
```

<br>
<b>Criar empresa</b>

```
POST /api/companies
Content-Type: application/json
```

Body:

```
{
  "cnpj": "12.345.678/0001-90",
  "nome_fantasia": "ACME",
  "razao_social": "ACME Indústria LTDA",
  "endereco": "Rua A, 123",
  "numero_funcionarios": 180
}
```

* O <b>ID</b> é o <b>CNPJ sanitizado</b>.

* Valida o CNPJ. Duplicado retorna <b>409</b>.
```
curl -i -XPOST http://localhost:8080/api/companies \
 -H 'Content-Type: application/json' \
 -d '{"cnpj":"12.345.678/0001-90","nome_fantasia":"ACME","razao_social":"ACME Indústria LTDA","endereco":"Rua A, 123","numero_funcionarios":180}'
```

<br>

### Buscar por ID (CNPJ sanitizado)
```
GET /api/companies/{cnpj_sanitizado}
```

```
curl -i http://localhost:8080/api/companies/12345678000190
```

### Atualização parcial (PATCH)
```
PATCH /api/companies/{cnpj_sanitizado}
Content-Type: application/json
```


<br>
Campos são opcionais (parciais). Se enviar numero_funcionarios, o Número Mínimo PCD é recalculado:

```
curl -i -XPATCH http://localhost:8080/api/companies/12345678000190 \
 -H 'Content-Type: application/json' \
 -d '{"numero_funcionarios": 520}'
```
<br>

### Substituição completa (PUT)
```
PUT /api/companies/{cnpj_sanitizado}
Content-Type: application/json
```

* PUT substitui o documento inteiro.

* Se cnpj vier no body, <b>deve ser igual</b> ao `{id}` da URL, senão <b>400</b>.

* Recalcula o <b>Número Mínimo PCD.</b>
```
curl -i -XPUT http://localhost:8080/api/companies/12345678000190 \
 -H 'Content-Type: application/json' \
 -d '{"cnpj":"12345678000190","nome_fantasia":"ACME 2","razao_social":"ACME Indústria LTDA","endereco":"Rua B, 456","numero_funcionarios": 1000}'
```
<br>

## Remover
```
DELETE /api/companies/{cnpj_sanitizado}
```
```
curl -i -XDELETE http://localhost:8080/api/companies/12345678000190
```

---
<br>

### Eventos (RabbitMQ)

Cada operação publica uma mensagem na fila (`RABBITMQ_QUEUE`, padrão `empresas_log`), via <b>default</b> exchange com routing key = nome da fila.

Tipos:

* <b>Cadastro:</b> “Cadastro da EMPRESA {NomeFantasia}”

* <b>Edição:</b> “Edição da EMPRESA {NomeFantasia}”

* <b>Exclusão:</b> “Exclusão da EMPRESA {NomeFantasia}”


A UI do Rabbit está em http://localhost:15672 (guest/guest). Você verá a fila empresas_log sendo alimentada.

---
<br>

### WebSocket / Hub

O serviço `ws` consome a fila do RabbitMQ e retransmite cada evento aos clientes conectados.

Endereço: `ws://localhost:8090/ws`

CORS/Origins: `WS_ALLOWED_ORIGINS` (padrão `*`)

API de mensagens: o servidor envia para os clientes os eventos publicados na fila (formato JSON/string).
O Hub suporta `Broadcast` (todos os clientes) e `Unicast` (um cliente), mas para este caso, usamos Broadcast.

Exemplo de teste rápido:
```bash
wscat -c ws://localhost:8090/ws           # terminal 1 (cliente WS)
# em outro terminal, faça um POST/PUT/PATCH/DELETE na API
# você verá o evento aparecer no cliente WS
```
---

### Testes (unitários e integração)
####  Rodar TUDO com Docker (CI)

Usa o serviço `ci` do Compose:

```
docker compose -f docker/docker-compose.yml --profile test run --rm ci
```

Por baixo, o script `docker/run-tests.sh` executa:

<b>Unitários</b> (sem tags)

<b>Integração</b> com Testcontainers (Mongo e Rabbit) via `-tags=integration`

Se preferir em <b>um comando:</b>

```
docker compose -f docker/docker-compose.yml --profile test run --rm ci \
  sh -lc 'go test -v -race -count=1 -tags=integration ./...'
```

#### Rodar testes específicos (direto no Go)

Exemplos:
```bash
# Handlers: só um teste
go test -run TestCompanies_List -v ./internal/handlers -count=1
```

```bash
# Domínio/Utils (PCD)
go test -run TestComputeMinPCD -v ./internal/utils -count=1
```

```bash
# Integração (repositório)
go test -v -count=1 -tags=integration ./internal/repository
```

```bash
# Integração (Rabbit)
go test -v -count=1 -tags=integration ./internal/broker
```


### O que os testes cobrem

#### Unitários

`company_handlers_test.go`: validações de query (`limit/skip`), fluxos de `POST/GET/PATCH/PUT/DELETE`, erros (CNPJ inválido, duplicado, not found), mapeamento de status HTTP, e publicação de eventos (mock do Publisher).

`pcd_test.go`: cálculo de <b>Número Mínimo de PCD</b> com casos de borda (ver próxima seção).

`hub_test.go`: comportamento do Hub (Broadcast/Unicast), filas por cliente, etc.

#### Integração

`company_repository_integration_test.go`: sob Mongo real (Testcontainers), cobre `Create/Get/Update/Replace/Delete` e chaves únicas (duplicidade).

`rabbitmq_integration_test.go`: sob Rabbit real (Testcontainers), publica e consome, garantindo funcionamento da fila.
---

<br>

### Lei 8.213/91 (PCD)

A Lei 8.213/91 define percentuais mínimos de pessoas com deficiência conforme o número de empregados:

100 a 200 empregados: <b>2%</b>

201 a 500: <b>3%</b>

501 a 1.000: <b>4%</b>

acima de 1.000: <b>5%</b>

A função `ComputeMinPCD` implementa essa regra com <b>arredondamento para cima</b> e é usada em:

* <b>POST</b> (criação)

* <b>PATCH</b> (se `numero_funcionarios` mudar)

* <b>PUT</b> (sempre, pois substitui o documento)

Testes:

* `internal/utils/pcd_test.go`: verifica a função pura (casos de borda).

* Nos testes de handlers, há asserts garantindo que o campo `numero_minimo_pcd_exigidos` é recalculado corretamente em <b>create, update (PATCH) e replace (PUT).</b>
---

<br>

### Aderência ao 12-Factor

* <b>Config:</b> toda via variáveis de ambiente (sem arquivos fixos hardcoded).

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

* <b>WebSocket ECONNREFUSED</b> garanta que o ws está up e escutando em :8090 e conecte em ws://localhost:8090/ws.

* <b>Rabbit “connection refused”:</b> revise RABBITMQ_URL no serviço (use rabbitmq como host em Compose). Se a API cair cedo, é por isso.

* <b>404 em PUT mismatch:</b> o handler busca o recurso antes de validar mismatch do CNPJ; em testes unitários, mocke GetByID para retornar um documento, senão o fluxo devolve 404 antes do 400.

* <b>Tags de integração:</b> os testes de integração têm //go:build integration. Rode com -tags=integration.
---

<br>

### Como o projeto atende ao desafio

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

  * 12-Factor: configurável, logs em stdout, admin tasks, etc.