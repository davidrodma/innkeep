# InnKeep — instruções para o Claude Code

PMS (sistema de gestão hoteleira) open source para hotéis, pousadas e hostels, em **microserviços
orientados a eventos**. Back-end em Go, front-end em Next.js. Repositório público: nada de segredos
nem dados pessoais reais.

@MEMORY.md

## Regras de idioma (obrigatórias)

- **Código em inglês**: identificadores, arquivos, rotas, tabelas, eventos, chaves JSON.
- **Comentários, documentação, mensagens de erro e textos de interface em pt-BR**, com tom didático:
  explique o *porquê* (o mantenedor é sênior em front-end e está aprendendo Go, Kafka, Redis e
  microserviços).

## Comandos

Go e Node podem não estar instalados no host; o Go roda em contêiner:

```bash
./scripts/go-docker.sh sh -c 'gofmt -l . ; go vet ./... && go test ./...'   # checagens do Go
docker compose up -d --build && docker compose run --rm seed                # stack completa
cd web && npm run lint && npm run typecheck && npm test && npm run build    # front-end
./scripts/validate-infra.sh                                                 # Helm + Terraform
python3 scripts/list-endpoints.py > docs/api.md                             # referência da API
```

Testes que precisam da stack no ar: integração (`-tags=integration`, `INTEGRATION_DATABASE_URL`),
E2E da API (`-tags=e2e`, `./tests/e2e`) e Playwright (`cd web && npm run test:e2e`). Detalhes na
skill `run-checks` e em `docs/guides/deploy-local.md`.

## Mapa do repositório

```
pkg/kernel/          Shared Kernel (DDD): Money (centavos), Date, StayPeriod [in,out), erros, IDs. Só stdlib.
pkg/kernel/access/   Papéis, permissões (RBAC), Principal.
pkg/contracts/       Eventos de integração (CloudEvents) — único acoplamento entre serviços.
pkg/platform/        config, logging, telemetry, postgres (TxManager), outbox, inbox, eventstore,
                     messaging (Kafka), httpx, auth (JWT/JWKS), redisx, fieldcrypt, app (ciclo de vida).
services/<nome>/     cmd/<nome>/main.go (composição) · internal/{domain,app,adapter/{in,out}} · migrations/
web/                 Next.js 16: src/app (rotas), src/features (por funcionalidade), src/lib, src/components.
tools/otasim, seed   Simulador de Booking.com/Airbnb; dados de demonstração via API.
tests/e2e/           E2E da API (Go). Integração fica junto do adaptador (`*_integration_test.go`).
deploy/              docker/ (Dockerfile e configs), helm/innkeep, terraform (AWS).
docs/                architecture/, adr/, guides/, api.md (gerado), glossary.md.
```

Serviços: `gateway`, `identity`, `property`, `inventory`, `reservation` (Event Sourcing + CQRS),
`guest` (FNRH, dados cifrados), `billing`, `channel`, `notification`.

## Decisões que não podem ser quebradas

- **Banco por serviço**; nenhum serviço lê o banco ou importa o `internal/` de outro.
- Entre serviços, **só eventos** (REST apenas na borda, via gateway).
- Escrita + evento na **mesma transação** (`tx.WithinTx` + `outbox.Publisher`). Consumidores
  **idempotentes** (`inbox.Middleware` ou operação absoluta).
- `domain/` não importa nada além de `pkg/kernel` e stdlib. Casos de uso dependem de portas (interfaces).
- **Autorização antes de qualquer alteração** (`access.EnsureProperty` após descobrir a propriedade do recurso).
- Dinheiro sempre em centavos `int64`; porcentagens em pontos-base; "hoje" no fuso da propriedade.
- Eventos: fatos no passado, versionados (`.v1`), evolução só por acréscimo, **sem dados pessoais sensíveis**.
- Logs com IDs, nunca nome/e-mail/CPF.

Detalhes por área em `.claude/rules/` (carregadas conforme os arquivos editados). Fundamentos em
`docs/architecture/overview.md` e nos ADRs.

## Agentes e skills do projeto

- Skills: `run-checks`, `local-stack`, `new-service`, `new-event`, `add-endpoint`.
- Agentes: `architecture-reviewer`, `security-reviewer`, `test-writer`, `go-mentor`.
