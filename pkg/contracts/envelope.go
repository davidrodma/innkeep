// Package contracts é a "Linguagem Publicada" (Published Language, no
// vocabulário do DDD) do InnKeep: os eventos de integração que os
// microserviços trocam pelo Kafka.
//
// É o ÚNICO ponto de acoplamento entre os serviços. Nenhum serviço importa o
// código interno de outro (o compilador impede: tudo fica em `internal/`); eles
// só concordam sobre o formato destas mensagens.
//
// Regras de evolução (leia antes de alterar qualquer struct daqui):
//   - Adicionar um campo opcional é compatível: consumidores antigos ignoram.
//   - Renomear/remover campo ou mudar o significado NÃO é compatível: crie um
//     novo tipo com sufixo ".v2" e publique os dois durante a migração.
//   - Todo evento é um fato no passado ("reserva confirmada"), nunca um
//     pedido ("confirme a reserva").
package contracts

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// SpecVersion é a versão da especificação CloudEvents seguida pelo envelope.
const SpecVersion = "1.0"

// Event é implementado por todo payload de evento de integração.
type Event interface {
	// EventType devolve o tipo versionado, no formato
	// "innkeep.<contexto>.<fato>.v<N>". O <contexto> define o tópico Kafka.
	EventType() string
	// PartitionKey devolve a identidade do agregado ao qual o fato pertence.
	// O Kafka garante ordem apenas dentro de uma partição, e a partição é
	// escolhida pelo hash desta chave: todos os eventos da mesma reserva caem
	// na mesma partição e, portanto, chegam em ordem.
	PartitionKey() string
}

// Envelope embrulha um evento com metadados padronizados. O formato segue a
// especificação CloudEvents 1.0 (https://cloudevents.io), um padrão da CNCF
// entendido por diversas ferramentas — não inventamos um formato próprio.
type Envelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`              // identidade única do evento (base da idempotência)
	Source          string          `json:"source"`          // serviço que publicou, ex.: "innkeep/reservation"
	Type            string          `json:"type"`            // ex.: "innkeep.reservation.confirmed.v1"
	Subject         string          `json:"subject"`         // ID do agregado
	Time            time.Time       `json:"time"`            // quando o fato aconteceu
	DataContentType string          `json:"datacontenttype"` // sempre application/json
	Data            json.RawMessage `json:"data"`            // payload (uma das structs deste pacote)

	// Extensões CloudEvents.
	CorrelationID string `json:"correlationid,omitempty"` // liga todos os eventos de um mesmo fluxo de negócio
	CausationID   string `json:"causationid,omitempty"`   // ID do evento que causou este
	TraceParent   string `json:"traceparent,omitempty"`   // contexto W3C para o tracing distribuído
	PartitionKey  string `json:"partitionkey,omitempty"`  // chave de partição no Kafka
}

// NewEnvelope cria o envelope de um evento, gerando ID e timestamp.
func NewEnvelope(source string, event Event) (Envelope, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return Envelope{}, fmt.Errorf("contracts: serializando %s: %w", event.EventType(), err)
	}
	return Envelope{
		SpecVersion:     SpecVersion,
		ID:              kernel.NewID(),
		Source:          "innkeep/" + source,
		Type:            event.EventType(),
		Subject:         event.PartitionKey(),
		Time:            time.Now().UTC(),
		DataContentType: "application/json",
		Data:            data,
		PartitionKey:    event.PartitionKey(),
	}, nil
}

// Decode lê o payload do envelope para o tipo T.
//
//	evt, err := contracts.Decode[contracts.ReservationConfirmed](envelope)
func Decode[T any](envelope Envelope) (T, error) {
	var payload T
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return payload, kernel.Invalid("event.invalid_payload", "payload inválido para o evento %s", envelope.Type).Wrap(err)
	}
	return payload, nil
}

// TopicFor deriva o tópico Kafka a partir do tipo do evento:
// "innkeep.reservation.confirmed.v1" => "innkeep.reservation.events".
//
// Usamos um tópico por contexto (e não um por tipo de evento) porque a ordem
// entre eventos diferentes do mesmo agregado importa: "confirmada" precisa
// chegar antes de "check-in realizado", e o Kafka só ordena dentro do mesmo
// tópico/partição.
func TopicFor(eventType string) string {
	parts := strings.Split(eventType, ".")
	if len(parts) < 3 {
		return "innkeep.unknown.events"
	}
	return parts[0] + "." + parts[1] + ".events"
}

// Tópicos Kafka do sistema (um por bounded context).
const (
	TopicProperty    = "innkeep.property.events"
	TopicInventory   = "innkeep.inventory.events"
	TopicReservation = "innkeep.reservation.events"
	TopicBilling     = "innkeep.billing.events"
	TopicGuest       = "innkeep.guest.events"
	TopicChannel     = "innkeep.channel.events"
)

// AllTopics lista todos os tópicos de eventos de negócio.
func AllTopics() []string {
	return []string{TopicProperty, TopicInventory, TopicReservation, TopicBilling, TopicGuest, TopicChannel}
}

// DeadLetterTopic devolve o tópico de "cartas mortas" de um grupo consumidor:
// é para lá que vão as mensagens que falharam repetidamente, para análise
// humana, sem travar o processamento das demais.
func DeadLetterTopic(consumerGroup string) string {
	return "innkeep.dlq." + consumerGroup
}
