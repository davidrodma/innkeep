# syntax=docker/dockerfile:1
#
# Imagem dos binários Go do InnKeep (serviços e ferramentas).
#
# Um único Dockerfile serve a todos: o argumento TARGET escolhe qual pacote
# `main` compilar. Exemplo:
#
#   docker build -f deploy/docker/go-service.Dockerfile \
#     --build-arg TARGET=services/reservation/cmd/reservation -t innkeep/reservation .
#
# É um build em DUAS ETAPAS (multi-stage):
#   1. "build": imagem completa do Go (~300 MB), com compilador e dependências;
#   2. final:   só o binário, sobre uma base "distroless" (~2 MB).
# A imagem que vai para produção não tem shell, gerenciador de pacotes nem
# compilador — menos superfície de ataque e nada para um invasor usar.

# Mantenha igual ao GO_VERSION de .github/workflows/ci.yml: o CI testa com a
# mesma linha do Go que compila as imagens.
ARG GO_VERSION=1.27

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

# Copia primeiro só os arquivos de dependências: enquanto go.mod/go.sum não
# mudarem, o Docker reaproveita a camada do download (cache de camadas).
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY pkg ./pkg
COPY services ./services
COPY tools ./tools

ARG TARGET
ARG VERSION=dev
# CGO_ENABLED=0 gera um binário estático (sem depender de bibliotecas C do
# sistema), requisito para rodar na imagem distroless "static".
# -trimpath e -ldflags "-s -w" removem caminhos locais e símbolos de depuração.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    test -n "${TARGET}" || (echo "informe --build-arg TARGET=<pacote main>" && exit 1); \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./${TARGET}

# Base distroless do Google: contém apenas certificados de CA, fuso horário e
# o usuário não-root. O sufixo ":nonroot" faz o processo rodar sem privilégios.
FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
ENV APP_VERSION=${VERSION}

COPY --from=build /out/app /app

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
