---
name: local-stack
description: Opera o ambiente local do InnKeep com Docker Compose — subir, recompilar um serviço, carregar dados de demonstração, ver logs, inspecionar Kafka/Redis/PostgreSQL, ligar a observabilidade e diagnosticar problemas. Use quando pedirem para rodar, reiniciar, depurar ou "ver funcionando".
---

# Ambiente local (Docker Compose)

## Subir e popular

```bash
cp -n .env.example .env
docker compose up -d --build
docker compose ps                    # esperar todos "healthy"
docker compose run --rm seed         # pousada de demonstração (idempotente)
```

Endereços: site/painel http://localhost:3000 (`admin@innkeep.local` / `innkeep-dev-admin`), API
http://localhost:8080, e-mails http://localhost:8025, Kafka UI http://localhost:8085, simulador de OTA
http://localhost:8090.

## Depois de alterar código

```bash
docker compose up -d --build <serviço>     # ex.: reservation, web, gateway
docker compose logs -f --tail 50 <serviço>
```

## Inspecionar

```bash
docker compose exec postgres psql -U postgres -d reservation -c 'select code, status from reservation_views order by requested_at desc limit 5'
docker compose exec postgres psql -U postgres -d inventory -c 'select count(*) from outbox where published_at is null'
docker compose exec kafka /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --all-groups
docker compose exec redis redis-cli --scan --pattern 'inventory:*' | head
```

Mensagens que falharam ficam em `innkeep.dlq.<serviço>` (cabeçalhos `dlq_reason` e `dlq_error`).

## Observabilidade

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318 docker compose --profile observability up -d
```

Jaeger http://localhost:16686 · Grafana http://localhost:3001 · Prometheus http://localhost:9090.

## Diagnóstico rápido

| Sintoma | Ação |
|---|---|
| serviço `unhealthy` | `docker compose logs <serviço>`; costuma ser Kafka ainda subindo → `docker compose up -d` |
| login bloqueado ("muitas tentativas") | `docker compose exec redis redis-cli FLUSHALL` |
| reserva presa em "Processando" | logs do `inventory` e lag do grupo `inventory` no Kafka UI |
| dados de teste demais | **pergunte ao usuário** antes de `docker compose down -v` (apaga todos os dados) |
