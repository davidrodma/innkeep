---
paths:
  - "services/*/migrations/**"
  - "services/*/internal/adapter/out/postgres/**"
---

# Regras para banco de dados

- Migrações goose (`-- +goose Up` / `-- +goose Down`), numeradas `NNNN_descricao.sql`, embutidas via `go:embed`.
- **Nunca edite uma migração já publicada**: crie a próxima.
- Migrações compatíveis com a versão ANTERIOR do código (*expand/contract*): adicionar coluna nula ou
  com default; remover só em um deploy posterior. O Job de migração roda antes do rolling update.
- Todo serviço que publica eventos tem `outbox` (com índice parcial `WHERE published_at IS NULL`);
  todo serviço que consome tem `inbox (consumer, event_id)`.
- Regras de integridade no banco quando possível (`CHECK`, `UNIQUE`, `EXCLUDE`): o banco é a última
  linha de defesa contra corrida.
- Valores monetários em `BIGINT` (centavos); datas de estadia em `DATE`; instantes em `TIMESTAMPTZ`.
- IDs `UUID` gerados na aplicação (`kernel.NewID`, v7).
- Repositórios usam `tx.DB(ctx)` (participam da transação do contexto) e traduzem violações
  (`postgres.IsUniqueViolation`, `IsExclusionViolation`) para erros de domínio.
