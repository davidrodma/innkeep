package contracts

import "time"

// Eventos do contexto de Hóspedes. Chave de partição: o ID da reserva.
const TypeRegistrationCompleted = "innkeep.guest.registration_completed.v1"

// RegistrationCompleted: a Ficha Nacional de Registro de Hóspedes (FNRH) da
// reserva foi preenchida, liberando o check-in.
//
// Repare que o evento NÃO carrega dados pessoais — apenas IDs. Eventos ficam
// retidos no Kafka e são lidos por vários serviços; dados sensíveis (CPF,
// endereço) nunca devem trafegar por eles (LGPD). Quem precisar dos dados
// consulta o serviço de hóspedes, que aplica o controle de acesso.
type RegistrationCompleted struct {
	ReservationID  string    `json:"reservation_id"`
	PropertyID     string    `json:"property_id"`
	PrimaryGuestID string    `json:"primary_guest_id"`
	GuestCount     int       `json:"guest_count"`
	At             time.Time `json:"at"`
}

func (RegistrationCompleted) EventType() string      { return TypeRegistrationCompleted }
func (e RegistrationCompleted) PartitionKey() string { return e.ReservationID }
