package contracts

import "time"

// Eventos do contexto Financeiro (billing). Chave de partição: o ID da reserva
// (cada reserva tem exatamente uma conta/folio), o que preserva a ordem em
// relação aos demais eventos daquela reserva.
const (
	TypePaymentRecorded = "innkeep.billing.payment_recorded.v1"
	TypeDepositSettled  = "innkeep.billing.deposit_settled.v1"
	TypeFolioSettled    = "innkeep.billing.folio_settled.v1"
)

// PaymentRecorded: um pagamento foi lançado na conta do hóspede.
type PaymentRecorded struct {
	FolioID       string    `json:"folio_id"`
	ReservationID string    `json:"reservation_id"`
	PropertyID    string    `json:"property_id"`
	EntryID       string    `json:"entry_id"`
	Method        string    `json:"method"` // PIX, CASH, CREDIT_CARD...
	AmountCents   int64     `json:"amount_cents"`
	FeeCents      int64     `json:"fee_cents"` // taxa da maquininha / comissão do canal
	Currency      string    `json:"currency"`
	PaidCents     int64     `json:"paid_cents"`    // total pago até agora
	BalanceCents  int64     `json:"balance_cents"` // quanto falta (negativo = crédito do hóspede)
	At            time.Time `json:"at"`
}

func (PaymentRecorded) EventType() string      { return TypePaymentRecorded }
func (e PaymentRecorded) PartitionKey() string { return e.ReservationID }

// DepositSettled: os pagamentos atingiram o valor do sinal. O serviço de
// reservas reage confirmando a reserva.
type DepositSettled struct {
	FolioID         string    `json:"folio_id"`
	ReservationID   string    `json:"reservation_id"`
	PropertyID      string    `json:"property_id"`
	DepositDueCents int64     `json:"deposit_due_cents"`
	PaidCents       int64     `json:"paid_cents"`
	At              time.Time `json:"at"`
}

func (DepositSettled) EventType() string      { return TypeDepositSettled }
func (e DepositSettled) PartitionKey() string { return e.ReservationID }

// FolioSettled: a conta foi quitada (saldo zero).
type FolioSettled struct {
	FolioID       string    `json:"folio_id"`
	ReservationID string    `json:"reservation_id"`
	PropertyID    string    `json:"property_id"`
	TotalCents    int64     `json:"total_cents"`
	At            time.Time `json:"at"`
}

func (FolioSettled) EventType() string      { return TypeFolioSettled }
func (e FolioSettled) PartitionKey() string { return e.ReservationID }
