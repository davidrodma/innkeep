# ADR 0004 — Transactional Outbox e Inbox em todos os serviços

**Situação:** aceita

## Contexto

Gravar no banco e publicar no Kafka são duas operações em sistemas diferentes. Se o processo cair entre
elas, o estado e os eventos divergem para sempre ("escrita dupla"). E o Kafka entrega cada mensagem
pelo menos uma vez — às vezes duas.

## Decisão

- **Outbox**: todo evento é gravado na tabela `outbox` do serviço, **na mesma transação** da mudança de
  estado. Um relay publica no Kafka, acordado por `LISTEN/NOTIFY`, com advisory lock para eleger uma
  única réplica publicadora e preservar a ordem.
- **Inbox**: todo consumidor registra o ID do evento na tabela `inbox` na mesma transação em que aplica
  os efeitos; IDs repetidos são ignorados.
- Handlers que só enviam **valores absolutos** a sistemas externos (o serviço de canais) dispensam a
  inbox: reprocessar é inofensivo por natureza.

## Alternativas consideradas

- **Change Data Capture (Debezium)**: robusto, mas acrescenta Kafka Connect à operação.
- **Publicar direto e torcer**: perde eventos em falhas — inaceitável para reservas e pagamentos.

## Consequências

- (+) Nenhum evento se perde; nenhum efeito é aplicado duas vezes. Provado por testes de integração.
- (−) Pequena latência extra (milissegundos) e uma tabela a limpar (o relay apaga eventos publicados
  há mais de 7 dias).
