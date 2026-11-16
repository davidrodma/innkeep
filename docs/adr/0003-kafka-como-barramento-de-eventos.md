# ADR 0003 — Apache Kafka como barramento, um tópico por contexto

**Situação:** aceita

## Contexto

Os serviços precisam trocar fatos com ordem garantida por agregado (uma reserva não pode ter o
check-in processado antes da confirmação), com retenção (um serviço novo deve conseguir reconstruir
seus dados) e com vários consumidores independentes para o mesmo evento.

## Decisão

- **Apache Kafka** (KRaft, sem ZooKeeper) localmente; **Amazon MSK** em produção.
- **Um tópico por bounded context** (`innkeep.<contexto>.events`), com o ID do agregado como chave.
- Envelope **CloudEvents 1.0** em JSON.
- Cliente Go **franz-go** (Go puro, sem cgo).
- Consumo "pelo menos uma vez", commit de offset após o processamento, retry com backoff e **DLQ por
  grupo consumidor**.

## Alternativas consideradas

- **RabbitMQ**: excelente fila, mas não retém o histórico nem ordena por chave com a mesma naturalidade.
- **Amazon SNS + SQS**: sem replay e com amarração à AWS (o ambiente local ficaria diferente).
- **Um tópico por tipo de evento**: perderia a ordem entre eventos diferentes do mesmo agregado.

## Consequências

- (+) Ordem por reserva, replay, consumidores novos sem alterar produtores.
- (−) Operar Kafka é complexo — mitigado com MSK gerenciado em produção.
- (−) Consumidores precisam ser idempotentes (ADR 0004).
