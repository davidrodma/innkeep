<div align="center">

# InnKeep

**Sistema de gestão hoteleira (PMS) de código aberto para hotéis, pousadas e hostels.**

Reservas, disponibilidade, check-in com a ficha exigida por lei (FNRH), financeiro completo
e sincronização com Booking.com e Airbnb — em uma arquitetura de microserviços orientada a eventos.

`Go` · `Next.js` · `PostgreSQL` · `Apache Kafka` · `Redis` · `OpenTelemetry` · `Docker` · `Kubernetes` · `AWS`

[![CI](https://github.com/davidrodma/innkeep/actions/workflows/ci.yml/badge.svg)](https://github.com/davidrodma/innkeep/actions/workflows/ci.yml)
[![Licença: MIT](https://img.shields.io/badge/licen%C3%A7a-MIT-blue.svg)](LICENSE)

</div>

---

## Sumário

- [O que o InnKeep faz](#o-que-o-innkeep-faz)
- [Comece em 5 minutos](#comece-em-5-minutos)
- [Arquitetura em diagramas](#arquitetura-em-diagramas)
- [Estrutura do repositório](#estrutura-do-repositório)
- [Testes](#testes)
- [Documentação](#documentação)
- [Segurança e LGPD](#segurança-e-lgpd)
- [Como contribuir](#como-contribuir)

## O que o InnKeep faz

| Área | Funcionalidades |
|---|---|
| **Reservas** | Calendário de ocupação (tape chart), reservas diretas e pelo site, sinal com prazo de pagamento, confirmação automática ao receber o sinal, alteração de datas/quarto, cancelamento, no-show, histórico completo de cada reserva. |
| **Disponibilidade e tarifas** | Diária por tipo de acomodação e por data (alta temporada, fim de semana), estadia mínima, fechamento de vendas (stop-sell), bloqueio de quartos para manutenção. Overbooking impossível por construção. |
| **Recepção** | Ficha Nacional de Registro de Hóspedes (FNRH) com validação de CPF e regra de menores desacompanhados (ECA), check-in, check-out (inclusive antecipado), governança (quarto limpo/sujo). |
| **Financeiro** | Conta do hóspede (folio) com diárias e extras; **pagamentos parciais e com várias formas misturadas** (Pix + dinheiro + cartão); taxas por forma de pagamento (maquininha, comissão do Booking.com); despesas; caixa em espécie (sangria/suprimento); contas a receber; relatório mensal com ocupação, ADR e RevPAR. |
| **Canais de venda** | Booking.com pela API de conectividade (para parceiros certificados) ou por calendário iCal; Airbnb por iCal. Toda mudança de disponibilidade é replicada automaticamente. Simulador de OTA incluso para desenvolver sem credenciais. |
| **Site público** | Busca de disponibilidade, reserva online e página de acompanhamento com instruções de Pix. E-mails transacionais. |
| **Equipe** | Usuários com papéis (administrador, gerente, recepção, financeiro, governança) e restrição por propriedade. Várias propriedades na mesma instalação. |

## Comece em 5 minutos

Você só precisa do **Docker** (com Docker Compose). Não é preciso instalar Go nem Node.

```bash
git clone https://github.com/davidrodma/innkeep.git
cd innkeep
cp .env.example .env
docker compose up -d --build        # sobe os 9 serviços, a infraestrutura e o site
docker compose run --rm seed        # carrega uma pousada de demonstração
```

| Endereço | O que é |
|---|---|
| http://localhost:3000 | Site de reservas e painel (`/login` → `admin@innkeep.local` / `innkeep-dev-admin`) |
| http://localhost:8080 | API (gateway) |
| http://localhost:8025 | E-mails enviados pelo sistema (Mailpit) |
| http://localhost:8085 | Tópicos e mensagens do Kafka (Kafka UI) |
| http://localhost:8090 | Simulador de OTA (faz o papel da Booking.com) |

O passo a passo completo — inclusive com traces no Jaeger e métricas no Grafana — está em
[docs/guides/deploy-local.md](docs/guides/deploy-local.md). Para colocar em produção na AWS, veja
[docs/guides/deploy-producao.md](docs/guides/deploy-producao.md).

## Arquitetura em diagramas

> A explicação completa — **por que** cada decisão foi tomada, com analogias para quem está
> chegando agora em microserviços, Go, Kafka e Redis — está em
> [docs/architecture/overview.md](docs/architecture/overview.md). As decisões formais ficam nos
> [ADRs](docs/adr/).

### 1. Contexto: quem usa o sistema

```mermaid
flowchart LR
    guest(["👤 Hóspede"])
    staff(["👥 Equipe da propriedade"])
    ota["🌐 Booking.com / Airbnb"]
    mail["✉️ E-mail do hóspede"]

    subgraph innkeep["InnKeep"]
        site["Site público de reservas"]
        admin["Painel administrativo"]
        core[["Microserviços"]]
    end

    guest -- "busca e reserva" --> site
    staff -- "opera recepção, reservas e financeiro" --> admin
    site & admin --> core
    core <-- "disponibilidade e preços ⇄ reservas" --> ota
    core -- "confirmações" --> mail
```

### 2. Contêineres: os serviços e o que cada um guarda

Cada serviço é dono do **seu** banco. Ninguém lê o banco de outro: eles conversam por eventos no Kafka.

```mermaid
flowchart TB
    browser(["Navegador"]) --> web["web<br/>Next.js (BFF)"]
    web --> gw["gateway<br/>roteamento · JWT · rate limit"]

    gw --> identity & property & inventory & reservation & guest & billing & channel

    subgraph svc["Microserviços em Go"]
        identity["identity<br/>usuários · login"]
        property["property<br/>propriedades · quartos"]
        inventory["inventory<br/>disponibilidade · tarifas"]
        reservation["reservation<br/>Event Sourcing + CQRS"]
        guest["guest<br/>hóspedes · FNRH"]
        billing["billing<br/>contas · pagamentos · caixa"]
        channel["channel<br/>Booking.com · Airbnb"]
        notification["notification<br/>e-mails"]
    end

    kafka{{"Apache Kafka<br/>um tópico por contexto"}}
    redis[("Redis<br/>cache · rate limit · idempotência")]

    identity --- db1[("identity")]
    property --- db2[("property")]
    inventory --- db3[("inventory")]
    reservation --- db4[("reservation<br/>event store")]
    guest --- db5[("guest<br/>dados cifrados")]
    billing --- db6[("billing")]
    channel --- db7[("channel")]

    property & inventory & reservation & guest & billing & channel -. "outbox → eventos" .-> kafka
    kafka -. "consomem" .-> property & inventory & reservation & guest & billing & channel & notification
    gw & identity & inventory & reservation & billing & notification --- redis
    channel <--> ota["Booking.com / Airbnb"]
```

### 3. A saga de uma reserva feita pelo site

Uma reserva atravessa quatro serviços sem que nenhum chame o outro diretamente. Cada passo é uma
transação local que publica um fato; o próximo serviço reage a ele (**saga coreografada**).

```mermaid
sequenceDiagram
    autonumber
    actor H as Hóspede
    participant R as reservation
    participant K as Kafka
    participant I as inventory
    participant B as billing
    participant C as channel
    participant N as notification

    H->>R: POST /v1/public/reservations
    R->>R: grava "reserva solicitada" (Event Store + outbox)
    R-->>H: 202 Accepted · status REQUESTED
    R->>K: reservation.requested
    K->>I: reservation.requested
    I->>I: aloca o quarto (EXCLUDE no PostgreSQL impede overbooking)
    I->>K: inventory.allocation_confirmed + availability_changed
    K->>R: allocation_confirmed → status PENDING (aguardando sinal)
    K->>C: availability_changed → Booking.com recebe a nova disponibilidade
    R->>K: reservation.accepted
    K->>B: abre a conta (folio) com as diárias
    K->>N: e-mail "falta o sinal"
    Note over H,B: a recepção registra o Pix do sinal
    B->>K: billing.deposit_settled
    K->>R: confirma a reserva → reservation.confirmed
    K->>N: e-mail "reserva confirmada"
```

Se o inventário não tiver quarto, ele publica `allocation_rejected` e a reserva vira `REJECTED`.
Se o sinal não for pago no prazo, a reserva expira, o quarto é liberado e a disponibilidade volta
aos canais — tudo por eventos.

### 4. Como um evento nunca se perde (Outbox) nem é processado duas vezes (Inbox)

```mermaid
flowchart LR
    subgraph A["Serviço que publica"]
        uc["caso de uso"] -->|"mesma transação"| state[("tabelas")]
        uc -->|"mesma transação"| outbox[("outbox")]
        relay["relay<br/>LISTEN/NOTIFY"] -->|lê| outbox
    end
    relay -->|publica| kafka{{"Kafka"}}
    subgraph B["Serviço que consome"]
        consumer["consumidor"] -->|"mesma transação"| inbox[("inbox<br/>IDs já vistos")]
        consumer -->|"mesma transação"| state2[("tabelas")]
    end
    kafka -->|"entrega ≥ 1 vez"| consumer
    consumer -. "falhou 5×" .-> dlq{{"DLQ"}}
```

### 5. Por dentro de um serviço: arquitetura hexagonal

```mermaid
flowchart LR
    subgraph in["Adaptadores de entrada"]
        rest["rest/<br/>HTTP"]
        cons["consumer/<br/>Kafka"]
    end
    subgraph core["Núcleo (não conhece banco, HTTP nem Kafka)"]
        app["app/<br/>casos de uso + PORTAS (interfaces)"]
        domain["domain/<br/>entidades e regras"]
    end
    subgraph out["Adaptadores de saída"]
        pg["postgres/"]
        rd["redis/"]
        ext["bookingcom/ · ical/"]
    end
    rest & cons --> app --> domain
    pg & rd & ext -. "implementam as portas" .-> app
```

### 6. Produção na AWS

```mermaid
flowchart TB
    users(["Usuários"]) --> alb["Application Load Balancer<br/>TLS (ACM) · WAF"]
    subgraph vpc["VPC — 3 zonas de disponibilidade"]
        subgraph eks["EKS (Kubernetes)"]
            web2["web"] & gw2["gateway"] --> pods["8 serviços<br/>HPA · PDB · NetworkPolicy"]
            otel["OpenTelemetry Collector"]
        end
        rds[("RDS PostgreSQL<br/>Multi-AZ")]
        msk{{"Amazon MSK<br/>3 brokers · TLS · SASL/SCRAM"}}
        cache[("ElastiCache Redis<br/>TLS · réplica")]
    end
    alb --> web2 & gw2
    pods --> rds & msk & cache
    sm["Secrets Manager"] -. "External Secrets" .-> eks
    gha["GitHub Actions (OIDC)"] -->|"imagens"| ecr["ECR"] --> eks
    otel --> obs["CloudWatch / X-Ray / Grafana"]
```

## Estrutura do repositório

```text
.
├── pkg/                        # código compartilhado entre os serviços
│   ├── kernel/                 # Shared Kernel (DDD): Money, Date, StayPeriod, erros, IDs, papéis
│   ├── contracts/              # eventos de integração (CloudEvents) — o "idioma" entre serviços
│   └── platform/               # kit técnico: postgres, outbox, inbox, eventstore, messaging,
│                               # httpx, auth, redisx, telemetry, fieldcrypt, app
├── services/                   # um microserviço por pasta, todos com o mesmo formato:
│   └── <serviço>/
│       ├── cmd/<serviço>/      #   main.go — raiz de composição (liga as peças)
│       ├── internal/domain/    #   entidades e regras de negócio
│       ├── internal/app/       #   casos de uso e portas (interfaces)
│       ├── internal/adapter/   #   in/ (rest, consumer) e out/ (postgres, redis, ...)
│       └── migrations/         #   SQL versionado, embutido no binário
├── web/                        # Next.js: site público (/p/...) e painel (/admin/...)
├── tools/otasim/               # simulador da Booking.com/Airbnb para desenvolvimento
├── tools/seed/                 # dados de demonstração (pela API, como um cliente real)
├── tests/e2e/                  # testes de ponta a ponta da API
├── deploy/
│   ├── docker/                 # Dockerfile dos serviços, scripts e configs locais
│   ├── helm/innkeep/           # chart do Kubernetes
│   └── terraform/              # infraestrutura AWS (VPC, EKS, RDS, MSK, ElastiCache, ECR, IAM)
├── docs/                       # arquitetura, ADRs, guias, API, glossário
└── .claude/                    # regras, skills e agentes para desenvolvimento com IA
```

## Testes

| Camada | Ferramenta | Como rodar |
|---|---|---|
| Unitários do domínio (Go) | `go test` | `./scripts/go-docker.sh go test ./...` |
| Integração com PostgreSQL real | `go test -tags=integration` | veja [deploy-local.md](docs/guides/deploy-local.md#testes) |
| Ponta a ponta da API (stack completa) | `go test -tags=e2e` | `go test -tags=e2e ./tests/e2e/...` |
| Unitários do front-end | Vitest + Testing Library | `cd web && npm test` |
| Ponta a ponta no navegador | Playwright | `cd web && npm run test:e2e` |

Alguns destaques do que é provado automaticamente:

- **20 gravações simultâneas** do mesmo quarto nas mesmas datas → exatamente 1 vence (restrição `EXCLUDE` do banco);
- dois comandos concorrentes sobre a mesma reserva → o segundo recebe conflito (concorrência otimista do Event Store);
- transação desfeita depois de publicar → o evento some junto (Transactional Outbox);
- reserva feita no simulador do Booking.com → entra no InnKeep; cancelada lá → cancelada aqui e o quarto volta à venda;
- reserva pelo site → recepção recebe o sinal em Pix + dinheiro → reserva confirmada (no navegador, com Playwright).

## Documentação

| Documento | Para quê |
|---|---|
| [Arquitetura explicada](docs/architecture/overview.md) | O porquê de cada decisão, em linguagem didática |
| [Decisões de arquitetura (ADRs)](docs/adr/) | Registro formal das escolhas e alternativas descartadas |
| [Eventos](docs/architecture/events.md) | Catálogo de eventos, tópicos e quem consome o quê |
| [Referência da API](docs/api.md) | Todos os endpoints, gerada a partir do código |
| [Deploy local](docs/guides/deploy-local.md) | Rodar, depurar e testar na sua máquina |
| [Deploy em produção](docs/guides/deploy-producao.md) | AWS passo a passo: Terraform, EKS, segredos, CI/CD |
| [Glossário](docs/glossary.md) | Termos de hotelaria e de arquitetura |

## Segurança e LGPD

O código é público, então a segurança não pode depender de segredo de implementação. Resumo do que
foi feito (detalhes em [SECURITY.md](SECURITY.md)):

- senhas com **Argon2id**; JWT assinado com **Ed25519**, validado por chave pública (JWKS) em cada serviço;
  refresh token com rotação e detecção de reuso; bloqueio após tentativas de login;
- **documentos dos hóspedes e credenciais dos canais cifrados** (AES-256-GCM) na aplicação, com índice cego para busca;
- eventos nunca carregam dados pessoais sensíveis; logs registram IDs, não nomes ou e-mails;
- proteção contra SSRF nas URLs informadas por usuários, SQL sempre parametrizado, CORS restrito,
  cabeçalhos de segurança, limites de requisição, contêineres sem root e com sistema de arquivos somente leitura;
- nenhum segredo no repositório: tudo por variáveis de ambiente / AWS Secrets Manager.

## Como contribuir

Leia o [CONTRIBUTING.md](CONTRIBUTING.md). Em resumo: código em inglês, comentários e documentação em
português, testes acompanhando cada mudança e o CI verde antes do merge.

## Licença

[MIT](LICENSE).
