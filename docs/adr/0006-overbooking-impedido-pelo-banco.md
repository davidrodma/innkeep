# ADR 0006 — Overbooking impedido por restrição EXCLUDE no PostgreSQL

**Situação:** aceita

## Contexto

"Verificar se está livre e depois gravar" tem uma janela de corrida: duas requisições simultâneas
veem o quarto livre e ambas o reservam. Locks em memória não funcionam com várias réplicas; locks
distribuídos (Redis) acrescentam um ponto de falha.

## Decisão

Entregar a regra ao banco:

```sql
EXCLUDE USING gist (room_id WITH =, (daterange(check_in, check_out, '[)')) WITH &&)
```

O serviço **tenta gravar** e trata a violação (`SQLSTATE 23P01`) como "quarto ocupado", tentando o
próximo quarto livre dentro de um **savepoint** (a transação externa continua válida).

Reservas de canais externos podem ser aceitas sem quarto (`room_id` nulo = overbooking sinalizado),
porque já estão confirmadas na plataforma de origem.

## Consequências

- (+) Garantia absoluta, inclusive sob concorrência e com N réplicas — provada por teste com 20
  gravações simultâneas.
- (+) Período semiaberto `[)`: saída e entrada no mesmo dia não conflitam.
- (−) Depende da extensão `btree_gist` (disponível no RDS).
