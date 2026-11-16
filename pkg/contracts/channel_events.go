package contracts

import "github.com/davidrodma/innkeep/pkg/kernel"

// Eventos do contexto de Canais de Venda (channel manager). Chave de
// partição: canal + ID externo, para preservar a ordem "recebida -> modificada
// -> cancelada" de cada reserva vinda do canal.
const (
	TypeChannelReservationReceived  = "innkeep.channel.reservation_received.v1"
	TypeChannelReservationModified  = "innkeep.channel.reservation_modified.v1"
	TypeChannelReservationCancelled = "innkeep.channel.reservation_cancelled.v1"
	TypeChannelSyncRequested        = "innkeep.channel.sync_requested.v1"
)

// ChannelSyncRequested: o serviço de canais pede a republicação completa de
// disponibilidade e tarifas de um tipo de quarto (ex.: ao conectar um canal
// novo, que nasce sem nenhum dado). O inventário responde emitindo
// AvailabilityChanged e RateChanged para o período.
//
// É "requisição/resposta" feita com eventos: o serviço de canais não chama o
// inventário por HTTP, então continua funcionando mesmo se ele estiver fora
// do ar — a resposta chega quando o inventário voltar.
type ChannelSyncRequested struct {
	ConnectionID string `json:"connection_id"`
	PropertyID   string `json:"property_id"`
	RoomTypeID   string `json:"room_type_id"`
	From         string `json:"from"` // inclusive
	To           string `json:"to"`   // exclusive
}

func (ChannelSyncRequested) EventType() string      { return TypeChannelSyncRequested }
func (e ChannelSyncRequested) PartitionKey() string { return e.RoomTypeID }

// ChannelReservation é a reserva recebida de um canal, JÁ TRADUZIDA para os
// IDs internos. O serviço de canais funciona como Anti-Corruption Layer: ele
// absorve o formato de cada parceiro (XML da Booking.com, iCal do Airbnb...) e
// entrega ao restante do sistema um modelo único e limpo.
type ChannelReservation struct {
	ConnectionID   string `json:"connection_id"`
	Channel        string `json:"channel"` // BOOKING_COM | AIRBNB
	PropertyID     string `json:"property_id"`
	ExternalID     string `json:"external_id"`
	RoomTypeID     string `json:"room_type_id"`
	RoomID         string `json:"room_id,omitempty"` // quando o anúncio corresponde a um quarto específico
	CheckIn        string `json:"check_in"`
	CheckOut       string `json:"check_out"`
	Adults         int    `json:"adults"`
	Children       int    `json:"children"`
	Holder         Holder `json:"holder"`
	TotalCents     int64  `json:"total_cents"`
	Currency       string `json:"currency"`
	PaymentCollect string `json:"payment_collect"` // PROPERTY | CHANNEL
	Notes          string `json:"notes,omitempty"`
}

func (r ChannelReservation) key() string { return r.Channel + ":" + r.ExternalID }

// ChannelReservationReceived: nova reserva feita no canal externo.
type ChannelReservationReceived struct{ ChannelReservation }

func (ChannelReservationReceived) EventType() string      { return TypeChannelReservationReceived }
func (e ChannelReservationReceived) PartitionKey() string { return e.key() }

// ChannelReservationModified: a reserva foi alterada no canal externo.
type ChannelReservationModified struct{ ChannelReservation }

func (ChannelReservationModified) EventType() string      { return TypeChannelReservationModified }
func (e ChannelReservationModified) PartitionKey() string { return e.key() }

// ChannelReservationCancelled: a reserva foi cancelada no canal externo.
type ChannelReservationCancelled struct {
	ConnectionID string `json:"connection_id"`
	Channel      string `json:"channel"`
	PropertyID   string `json:"property_id"`
	ExternalID   string `json:"external_id"`
	Reason       string `json:"reason,omitempty"`
}

func (ChannelReservationCancelled) EventType() string { return TypeChannelReservationCancelled }
func (e ChannelReservationCancelled) PartitionKey() string {
	return e.Channel + ":" + e.ExternalID
}

// ReservationIDForChannel devolve o ID interno — determinístico — da reserva
// correspondente a uma reserva de canal. Receber a mesma reserva duas vezes
// produz o mesmo ID, então a segunda criação é naturalmente ignorada, e
// modificações/cancelamentos encontram a reserva sem precisar de tabela de
// correspondência.
//
// A propriedade faz parte da chave: o número de uma reserva só é garantidamente
// único DENTRO da conta de um canal, e duas propriedades diferentes não podem
// acabar compartilhando a mesma reserva interna.
func ReservationIDForChannel(propertyID, channel, externalID string) string {
	return kernel.DeterministicID("channel-reservation", propertyID+":"+channel+":"+externalID)
}
