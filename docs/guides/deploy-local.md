# Guia: rodando o InnKeep na sua máquina

Tudo roda em contêineres Docker. Você **não** precisa instalar Go, Node, PostgreSQL, Kafka nem Redis.

## 1. Pré-requisitos

| Sistema | O que instalar |
|---|---|
| Windows | **WSL 2** com uma distribuição Linux (Ubuntu) e o Docker Engine dentro dela — ou o Docker Desktop com integração WSL. Rode os comandos no terminal do Linux, de preferência com o projeto clonado no sistema de arquivos do Linux (`~/innkeep`): é bem mais rápido do que em `/mnt/c`. As pastas do Windows aparecem no Linux sob `/mnt/` (ex.: `C:\projetos\innkeep` é `/mnt/c/projetos/innkeep`). |
| macOS | Docker Desktop (ou OrbStack/Colima). |
| Linux | Docker Engine + plugin Docker Compose. |

Recursos recomendados para o Docker: **4 CPUs e 6 GB de RAM** (o Kafka e os 9 serviços sobem juntos).

Portas usadas no `localhost`: 3000, 8080, 8025, 8085, 8090, 5432, 6379 e 29092 (e 16686, 9090, 3001
com observabilidade). Se alguma já estiver ocupada, altere o lado esquerdo do mapeamento em
`docker-compose.yml` (ex.: `"127.0.0.1:18080:8080"`).

## 2. Subindo

```bash
git clone https://github.com/davidrodma/innkeep.git
cd innkeep
cp .env.example .env
docker compose up -d --build
```

Daqui em diante, os comandos rodam na **raiz do repositório**: a pasta `innkeep` em que o `cd` acima
entrou, onde fica o `docker-compose.yml`. Os que precisam de outra pasta trazem o próprio `cd` (como
`cd web`). O motivo é que o `docker compose` procura o `docker-compose.yml` na pasta atual e, se não
achar, nas pastas acima dela. Fora do projeto, ele responde `no configuration file provided: not found`.

A primeira vez leva alguns minutos (download das imagens e compilação). Acompanhe com:

```bash
docker compose ps          # todos devem ficar "healthy"
docker compose logs -f reservation
```

Carregue a pousada de demonstração (fictícia, feita **pela API**, como um cliente real faria):

```bash
docker compose run --rm seed
```

## 3. Passeio guiado

1. **Site público** — http://localhost:3000. Escolha datas, uma acomodação e faça uma reserva.
   A página de acompanhamento começa em "confirmando a disponibilidade" e muda sozinha para
   "falta o sinal": isso é a saga acontecendo entre três serviços.
2. **E-mails** — http://localhost:8025. Chegou o e-mail com a chave Pix e o prazo.
3. **Kafka** — http://localhost:8085 → Topics → `innkeep.reservation.events` → Messages. Veja os
   eventos da sua reserva no envelope CloudEvents.
4. **Painel** — http://localhost:3000/login (`admin@innkeep.local` / `innkeep-dev-admin`).
   Abra a reserva, receba o sinal com **duas formas de pagamento** (botão "Outra forma de pagamento")
   e veja a reserva ficar **Confirmada** — o financeiro publicou "sinal quitado" e o serviço de reservas
   reagiu. O "Histórico" mostra os eventos gravados no Event Store.
5. **Canal de venda** — http://localhost:8090 mostra o que o "Booking.com" simulado recebeu. A reserva
   que você fez reduziu a disponibilidade lá. Crie uma reserva **no canal**:

   ```bash
   curl -X POST localhost:8090/sim/reservations -H 'Content-Type: application/json' \
     -d '{"hotel_id":"9000001","room_id":"900103","check_in":"2027-01-10","check_out":"2027-01-12","guest_name":"Cliente Booking"}'
   ```

   Em até 20 segundos ela aparece no painel do InnKeep, já confirmada e com o quarto alocado.

## 4. Observabilidade

Suba as ferramentas extras e reinicie os serviços apontando para o OpenTelemetry Collector:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318 docker compose --profile observability up -d
```

- **Jaeger** (http://localhost:16686): escolha o serviço `reservation` e abra um trace de
  `POST /v1/reservations`. A árvore atravessa gateway → reservation → PostgreSQL → Kafka → inventory →
  billing. Copie o `trace_id` de qualquer linha de log para encontrar o trace correspondente.
- **Grafana** (http://localhost:3001) com Prometheus (métricas) e Jaeger já configurados como fontes.

## 5. Desenvolvendo

### Compilar e testar o Go sem instalar o Go

```bash
./scripts/go-docker.sh go build ./...
./scripts/go-docker.sh go test ./...
./scripts/go-docker.sh sh -c 'gofmt -l . ; go vet ./...'
```

Os caches de módulos e de compilação ficam em volumes Docker: só a primeira execução é lenta.

### Mudou um serviço? Recompile só ele

```bash
docker compose up -d --build reservation
```

### Front-end com recarga instantânea

```bash
cd web
npm install
API_URL=http://localhost:8080 npm run dev     # http://localhost:3000 (pare o contêiner "web" antes)
```

### Rodar um serviço Go fora do Docker (com o Go instalado)

A infraestrutura fica acessível em `localhost`. Pare o contêiner do serviço e rode-o direto:

```bash
docker compose stop billing
DATABASE_URL=postgres://billing:innkeep_dev@localhost:5432/billing?sslmode=disable \
KAFKA_BROKERS=localhost:29092 REDIS_URL=redis://localhost:6379/0 \
AUTH_JWKS_URL=http://localhost:8080/.well-known/jwks.json HTTP_ADDR=:8086 LOG_FORMAT=text \
go run ./services/billing/cmd/billing
```

(O gateway continuará apontando para o contêiner parado; para chamadas pela API, rode também o gateway
localmente ou chame o serviço direto na porta escolhida.)

## 6. Testes

```bash
# Unitários (Go)
./scripts/go-docker.sh go test ./...

# Integração com PostgreSQL real (cria e apaga bancos temporários)
DOCKER_ARGS="--network host -e INTEGRATION_DATABASE_URL=postgres://postgres:postgres_dev@localhost:5432/postgres?sslmode=disable" \
  ./scripts/go-docker.sh go test -tags=integration ./...

# Ponta a ponta da API (stack no ar; eleve o limite de login para a bateria)
RATE_LIMIT_AUTH_PER_MINUTE=500 docker compose up -d gateway
DOCKER_ARGS="--network host" ./scripts/go-docker.sh go test -tags=e2e -count=1 ./tests/e2e/...

# Front-end
cd web && npm test                      # Vitest
npx playwright install chromium         # uma vez
npm run test:e2e                        # Playwright (stack no ar + seed)
```

Os testes de ponta a ponta criam propriedades de teste ("Pousada E2E ..."). Para voltar a um banco
limpo, veja a próxima seção.

## 7. Problemas comuns

| Sintoma | Causa provável e solução |
|---|---|
| `no configuration file provided: not found` | O terminal está fora da pasta do projeto. Entre na raiz do repositório (onde fica o `docker-compose.yml`) e repita o comando. |
| Um serviço fica `unhealthy` | `docker compose logs <serviço>`. Na primeira subida o Kafka pode demorar; `docker compose up -d` de novo resolve. |
| `port is already allocated` | Outra aplicação usa a porta. Troque o mapeamento no `docker-compose.yml`. |
| Tudo lento no Windows | Clone o projeto dentro do Linux do WSL (`~/innkeep`), não em `/mnt/c`. Aumente a memória do WSL em `%UserProfile%\.wslconfig`. |
| Login falha com "muitas tentativas" | O bloqueio de força bruta dura 15 min. Para zerar: `docker compose exec redis redis-cli FLUSHALL`. |
| Quero começar do zero | `docker compose down -v` (apaga os volumes), depois `up -d` e `run --rm seed`. |
| Mensagens paradas | Veja o *lag* dos grupos no Kafka UI e as DLQs (`innkeep.dlq.<serviço>`). |
