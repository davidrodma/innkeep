---
name: run-checks
description: Roda as checagens do InnKeep (formatação, lint, testes unitários, integração, E2E da API, front-end e infraestrutura) com os comandos corretos para este repositório. Use antes de concluir qualquer mudança ou quando pedirem para "rodar os testes".
---

# Checagens do InnKeep

Escolha o conjunto pelo que foi alterado. Os comandos assumem a raiz do repositório e espelham os
jobs do CI (`.github/workflows/ci.yml`): o que passa aqui passa lá.

## 1. Go (sempre que mexer em `pkg/`, `services/`, `tools/` ou `tests/`)

```bash
./scripts/go-docker.sh sh -c 'gofmt -l pkg services tools tests; go vet -tags=integration,e2e ./... && go test ./...'
```

- `gofmt -l` deve imprimir **nada**. Se imprimir arquivos, rode `gofmt -w` neles.
- Não há Go instalado no host: o script roda a toolchain em contêiner (caches em volumes).

Lint — o mesmo `golangci-lint` do CI, configurado em `.golangci.yml`:

```bash
docker run --rm -v "$PWD":/app -w /app -v innkeep-gomod:/go/pkg/mod -v innkeep-lintcache:/root/.cache \
  golangci/golangci-lint:latest golangci-lint run ./...
```

- Exceção a um alerta só na própria linha e com o motivo: `//nolint:gosec // G704: ...`. Nunca
  desligue um linter inteiro para calar um caso.

## 2. Integração com PostgreSQL e detector de corrida

Rode quando mudar repositório, migração, outbox, inbox, event store ou código concorrente. Precisa
do PostgreSQL do Compose no ar (`docker compose up -d postgres`). O `-race` exige cgo, por isso a
imagem Debian do Go (não a alpine):

```bash
GO_IMAGE=golang:1.27 CGO_ENABLED=1 \
DOCKER_ARGS="--network host -e INTEGRATION_DATABASE_URL=postgres://postgres:postgres_dev@localhost:5432/postgres?sslmode=disable" \
  ./scripts/go-docker.sh go test -race -tags=integration -count=1 ./...
```

## 3. Ponta a ponta da API (mudou fluxo entre serviços, eventos ou contratos)

```bash
docker compose up -d --build                      # recompila o que mudou
RATE_LIMIT_AUTH_PER_MINUTE=500 docker compose up -d gateway
DOCKER_ARGS="--network host" ./scripts/go-docker.sh go test -tags=e2e -count=1 -timeout 8m ./tests/e2e/...
docker compose up -d gateway                      # volta ao limite de login padrão
```

## 4. Front-end (mudou `web/`)

```bash
cd web
npm run lint && npm run typecheck && npm test && npm run build
```

Fluxos de navegador (stack no ar + `docker compose run --rm seed`; após mudar o `web`, rode
`docker compose up -d --build web`):

```bash
npx playwright test
```

## 5. Segurança (mudou dependências, autenticação, dados pessoais ou integrações)

```bash
./scripts/go-docker.sh sh -c 'go install golang.org/x/vuln/cmd/govulncheck@latest && /go/bin/govulncheck ./...'
(cd web && npm audit --omit=dev --audit-level=high)
docker run --rm -v "$PWD":/scan:ro ghcr.io/gitleaks/gitleaks:v8.28.0 dir /scan --no-banner --redact
```

- O `govulncheck` usa a biblioteca padrão do Go do contêiner (1.27, a mesma das imagens e do CI).
- Uma chave pública **de propósito** (exemplo, desenvolvimento) recebe `// gitleaks:allow` na própria
  linha, com a explicação. Qualquer outra chave no código é um vazamento: remova e troque-a.

## 6. Infraestrutura (mudou `deploy/` ou `.github/`)

```bash
./scripts/validate-infra.sh
```

## 7. Documentação gerada

Se rotas mudaram: `python3 scripts/list-endpoints.py > docs/api.md`.

Relate o resultado com números (pacotes/testes que passaram) e mostre a saída de qualquer falha.
