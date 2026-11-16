---
name: new-service
description: Cria um novo microserviço Go no InnKeep seguindo a arquitetura hexagonal do projeto (domínio, casos de uso, portas, adaptadores REST/Kafka/PostgreSQL, migrações com outbox/inbox, main de composição) e o registra no Compose, gateway, Helm, Terraform, CI e documentação. Use quando pedirem um serviço/bounded context novo.
---

# Novo microserviço

Antes de começar, confirme com o usuário **qual pergunta do negócio** o serviço responde e **de quais
dados ele é dono**. Se a resposta for "dos dados de outro serviço", provavelmente não é um serviço novo.

Use `services/property` como modelo (o mais simples) e `services/billing` para algo com consumidor.

## 1. Estrutura

```
services/<nome>/
├── cmd/<nome>/main.go
├── internal/domain/            # entidades e erros de domínio (kernel.NotFound(...))
├── internal/app/ports.go       # interfaces: repositórios, EventPublisher, TxRunner
├── internal/app/service.go     # casos de uso
├── internal/adapter/in/rest/handler.go
├── internal/adapter/in/consumer/consumer.go   # se consumir eventos
├── internal/adapter/out/postgres/*.go
└── migrations/{migrations.go,0001_init.sql}   # incluir outbox e/ou inbox
```

## 2. main.go (composição)

Siga `services/property/cmd/property/main.go`: `config.Load` → `app.Bootstrap` → `postgres.Migrate`
(+ subcomando `migrate`) → `postgres.NewPool` → `outbox.NewPipeline` → casos de uso →
`messaging.NewRouter(inbox.Middleware(tx, nome))` + `messaging.NewConsumer` → `httpx.NewHealth` +
`auth.NewGuard(auth.NewVerifier(...))` → `app.Run` com `http`, `outbox-relay` e `event-consumer`.

## 3. Registrar

- **Eventos**: novo contexto → constante de tópico em `pkg/contracts/envelope.go` (`TopicX` e `AllTopics`).
- **Compose**: bloco do serviço (âncoras `*go-service`, `*go-build`, `*common-env`, `*depends-infra`) e
  o banco em `deploy/docker/postgres/init-databases.sh`.
- **Gateway**: rotas em `services/gateway/cmd/gateway/main.go` (+ `<NOME>_URL` no Compose e no Helm).
- **Permissões**: novas em `pkg/kernel/access/access.go` (e na matriz de papéis).
- **Helm**: bloco em `services:` do `values.yaml` (`migrate`, `kafka`, `secretEnv`).
- **Terraform**: nome na lista `services` (ECR); `scripts/bootstrap-secrets.sh` (banco).
- **CI/CD**: entrada na matriz de `.github/workflows/release.yml`.
- **Docs**: tabela de serviços em `docs/architecture/overview.md` e `CLAUDE.md`; diagrama do README;
  `docs/architecture/events.md`; `python3 scripts/list-endpoints.py > docs/api.md`; ADR se a decisão for relevante.

## 4. Testes

Domínio (unitários), repositório (`*_integration_test.go`), fluxo entre serviços (`tests/e2e`).
Finalize com a skill `run-checks`.
