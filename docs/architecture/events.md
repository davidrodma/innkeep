# Catálogo de eventos

Os eventos de integração são o **único** acoplamento entre os serviços. Eles ficam em
[`pkg/contracts`](../../pkg/contracts/) e seguem três regras:

1. **Fatos no passado** (`reservation.confirmed`), nunca ordens.
2. **Versionados no nome** (`.v1`). Acrescentar um campo opcional é compatível; mudar o significado de
   um campo exige um tipo novo (`.v2`), publicado em paralelo durante a migração.
3. **Sem dados pessoais sensíveis.** Documentos e endereços nunca trafegam no Kafka.

Envelope (CloudEvents 1.0): `specversion`, `id`, `source`, `type`, `subject`, `time`, `data`, mais as
extensões `correlationid`, `causationid`, `traceparent` e `partitionkey`.

## Tópicos

| Tópico | Chave de partição | Produtor |
|---|---|---|
| `innkeep.property.events` | ID da propriedade | property |
| `innkeep.inventory.events` | ID da reserva (alocações) ou do tipo de quarto (disponibilidade/tarifas) | inventory |
| `innkeep.reservation.events` | ID da reserva | reservation |
| `innkeep.billing.events` | ID da reserva | billing |
| `innkeep.guest.events` | ID da reserva | guest |
| `innkeep.channel.events` | `canal:id-externo` (reservas) ou tipo de quarto (sincronização) | channel |
| `innkeep.dlq.<serviço>` | a original | qualquer consumidor (mensagens que falharam) |

## Eventos e consumidores

| Evento | Significa | Consumido por → reação |
|---|---|---|
| `property.registered/updated.v1` | propriedade criada/alterada (estado completo) | reservation → copia as políticas (sinal, prazo, fuso) · notification → nome e contato para e-mails |
| `property.room_type_defined/changed.v1` | tipo de acomodação criado/alterado | inventory → cópia local; republica tarifas se o preço mudou |
| `property.room_added/changed.v1` | quarto criado/alterado | inventory → cópia local; republica disponibilidade |
| `reservation.requested.v1` | pedido de reserva | inventory → tenta alocar um quarto |
| `inventory.allocation_confirmed.v1` | quarto alocado e precificado | reservation → PENDING ou CONFIRMED |
| `inventory.allocation_rejected.v1` | sem disponibilidade | reservation → REJECTED |
| `reservation.accepted.v1` | alocação aceita, com diárias e sinal | billing → abre o folio · notification → "falta o sinal" |
| `billing.deposit_settled.v1` | sinal quitado | reservation → CONFIRMED |
| `billing.payment_recorded.v1`, `folio_settled.v1` | pagamento lançado / conta quitada | (disponível para relatórios e integrações) |
| `reservation.confirmed.v1` | reserva garantida | notification → "reserva confirmada" |
| `reservation.cancelled/expired/no_show.v1` | reserva encerrada sem estadia | inventory → libera o quarto · billing → anula diárias · notification |
| `reservation.stay_change_requested.v1` | pedido de mudança de datas/quarto | inventory → move a alocação |
| `inventory.allocation_changed/change_rejected.v1` | resposta à mudança | reservation → aplica ou descarta |
| `reservation.stay_changed.v1` | estadia alterada | billing → refaz as diárias |
| `guest.registration_completed.v1` | FNRH preenchida | reservation → libera o check-in |
| `reservation.checked_in.v1` | hóspede entrou | guest → carimba a ficha |
| `reservation.checked_out.v1` | hóspede saiu | inventory → libera noites restantes · billing → fecha a conta quitada · property → quarto sujo · guest |
| `inventory.availability_changed.v1` | disponibilidade mudou em uma ou mais datas | channel → envia ao Booking.com / atualiza o iCal |
| `inventory.rate_changed.v1` | preço/restrições mudaram | channel → envia ao Booking.com |
| `channel.sync_requested.v1` | canal novo pede tudo de novo | inventory → republica disponibilidade e tarifas |
| `channel.reservation_received/modified/cancelled.v1` | reserva feita/alterada/cancelada no canal | reservation → cria/altera/cancela (ID determinístico) |

## Como inspecionar

- **Kafka UI** (http://localhost:8085): tópicos, mensagens, grupos consumidores e atraso (*lag*).
- **DLQ**: uma mensagem em `innkeep.dlq.<serviço>` traz nos cabeçalhos `dlq_reason`, `dlq_error`,
  `dlq_original_topic` e `dlq_original_offset`. Depois de corrigir a causa, reenvie para o tópico
  original (ou deixe a próxima mensagem do mesmo agregado corrigir o estado).
