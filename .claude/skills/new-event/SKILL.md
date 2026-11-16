---
name: new-event
description: Cria ou evolui um evento de integração do InnKeep (contrato em pkg/contracts, publicação via outbox, consumo idempotente, documentação e testes), respeitando as regras de versionamento e LGPD. Use ao acrescentar um fato que outro serviço precisa saber ou ao mudar o formato de um evento existente.
---

# Novo evento de integração

## 1. Defina o fato

- Nome no passado: `innkeep.<contexto>.<fato>.v1` (ex.: `innkeep.billing.refund_issued.v1`).
- Quem publica (o dono do dado) e quem consome — e **por quê** cada consumidor precisa disso.
- Chave de partição: o agregado cuja ordem importa.
- Campos: o estado que o consumidor precisa para não ter que perguntar ao produtor. **Sem dados
  pessoais sensíveis.**

## 2. Contrato (`pkg/contracts/<contexto>_events.go`)

```go
const TypeRefundIssued = "innkeep.billing.refund_issued.v1"

// RefundIssued: uma devolução foi registrada na conta do hóspede.
type RefundIssued struct {
	FolioID       string `json:"folio_id"`
	ReservationID string `json:"reservation_id"`
	AmountCents   int64  `json:"amount_cents"`
}

func (RefundIssued) EventType() string      { return TypeRefundIssued }
func (e RefundIssued) PartitionKey() string { return e.ReservationID }
```

Evoluindo um evento existente: só acréscimos opcionais (`omitempty`). Mudança incompatível → tipo
`.v2` novo, publicado junto com o `.v1` até todos os consumidores migrarem.

## 3. Publicar

Dentro do caso de uso, **na mesma transação** da mudança de estado:

```go
return s.tx.WithinTx(ctx, func(ctx context.Context) error {
	if err := s.folios.Save(ctx, folio); err != nil {
		return err
	}
	return s.publisher.Publish(ctx, contracts.RefundIssued{...})
})
```

## 4. Consumir

No `internal/adapter/in/consumer` do serviço interessado:

```go
router.On(contracts.TypeRefundIssued, messaging.Typed(
	func(ctx context.Context, _ contracts.Envelope, e contracts.RefundIssued) error {
		return service.OnRefundIssued(ctx, e)
	}))
```

Garanta que o tópico esteja na lista `Topics` do `messaging.NewConsumer` do serviço. O handler deve
ser idempotente (o roteador com `inbox.Middleware` cuida disso) e tolerar chegar antes de eventos de
outros tópicos.

## 5. Documentar e testar

- Linha nova em `docs/architecture/events.md`.
- Teste unitário do caso de uso que publica; teste E2E se o evento fecha um fluxo entre serviços.
- Finalize com a skill `run-checks`.
