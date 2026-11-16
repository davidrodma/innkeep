#!/usr/bin/env bash
# Executa a toolchain do Go dentro de um contêiner Docker.
#
# Serve para quem ainda não instalou o Go na máquina (ou quer garantir a mesma
# versão usada no CI). Os caches de módulos e de compilação ficam em volumes
# nomeados, então só a primeira execução é lenta.
#
# Uso:
#   ./scripts/go-docker.sh go test ./...
#   ./scripts/go-docker.sh go build ./services/reservation/...
#   ./scripts/go-docker.sh sh -c 'go mod tidy && go vet ./...'
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${GO_IMAGE:-golang:1.27-alpine}"

exec docker run --rm \
  -v "${ROOT}":/src -w /src \
  -v innkeep-gomod:/go/pkg/mod \
  -v innkeep-gocache:/root/.cache/go-build \
  -e CGO_ENABLED="${CGO_ENABLED:-0}" \
  -e GOFLAGS="${GOFLAGS:--buildvcs=false}" \
  ${DOCKER_ARGS:-} \
  "${IMAGE}" "$@"
