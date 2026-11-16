package domain

import (
	"strings"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// Valores aceitos para o motivo da viagem e o meio de transporte, conforme
// os campos da FNRH.
var (
	TripPurposes = []string{"LEISURE", "BUSINESS", "EVENT", "FAMILY", "STUDY", "HEALTH", "RELIGION", "SHOPPING", "OTHER"}
	Transports   = []string{"CAR", "BUS", "PLANE", "MOTORCYCLE", "SHIP", "TRAIN", "OTHER"}
)

// Place é uma localidade (procedência ou destino).
type Place struct {
	City    string
	State   string
	Country string
}

// RegistrationCard é a Ficha Nacional de Registro de Hóspedes (FNRH) de uma
// pessoa em uma estadia.
//
// Base legal: a Lei Geral do Turismo (Lei 11.771/2008) obriga os meios de
// hospedagem a registrar cada hóspede e a informar os dados ao Ministério do
// Turismo. Por isso a ficha guarda mais do que o cadastro: de onde a pessoa
// veio, para onde vai, por que viaja e como chegou.
type RegistrationCard struct {
	ID            string
	ReservationID string
	PropertyID    string
	GuestID       string
	// Primary marca o titular da estadia.
	Primary       bool
	PurposeOfTrip string
	Transport     string
	Origin        Place // última procedência
	Destination   Place // próximo destino
	VehiclePlate  string
	CheckIn       kernel.Date // previsto
	CheckOut      kernel.Date // previsto
	CheckedInAt   *time.Time  // efetivo
	CheckedOutAt  *time.Time  // efetivo
	// ConsentAt registra quando o hóspede tomou ciência do tratamento dos
	// seus dados (LGPD: transparência).
	ConsentAt time.Time
	CreatedBy string
	CreatedAt time.Time
}

// TripDetails são os dados de viagem informados na ficha.
type TripDetails struct {
	PurposeOfTrip string
	Transport     string
	Origin        Place
	Destination   Place
	VehiclePlate  string
}

// NewRegistrationCard cria a ficha de um hóspede para uma estadia.
func NewRegistrationCard(reservationID, propertyID, guestID string, primary bool, stay kernel.StayPeriod, trip TripDetails, by string, now time.Time) (*RegistrationCard, error) {
	trip.PurposeOfTrip = strings.ToUpper(strings.TrimSpace(trip.PurposeOfTrip))
	trip.Transport = strings.ToUpper(strings.TrimSpace(trip.Transport))
	if trip.PurposeOfTrip == "" {
		trip.PurposeOfTrip = "LEISURE"
	}
	if trip.Transport == "" {
		trip.Transport = "CAR"
	}
	if !contains(TripPurposes, trip.PurposeOfTrip) {
		return nil, kernel.Invalid("registration.invalid_purpose", "motivo da viagem inválido").WithField("purpose_of_trip", "valor desconhecido")
	}
	if !contains(Transports, trip.Transport) {
		return nil, kernel.Invalid("registration.invalid_transport", "meio de transporte inválido").WithField("transport", "valor desconhecido")
	}
	return &RegistrationCard{
		ID: kernel.NewID(), ReservationID: reservationID, PropertyID: propertyID, GuestID: guestID, Primary: primary,
		PurposeOfTrip: trip.PurposeOfTrip, Transport: trip.Transport,
		Origin: trip.Origin, Destination: trip.Destination,
		VehiclePlate: strings.ToUpper(strings.TrimSpace(trip.VehiclePlate)),
		CheckIn:      stay.CheckIn(), CheckOut: stay.CheckOut(),
		ConsentAt: now, CreatedBy: by, CreatedAt: now,
	}, nil
}

// ValidateParty confere as regras sobre o grupo de hóspedes de uma estadia.
//
// Estatuto da Criança e do Adolescente (Lei 8.069/1990, art. 82): é proibida
// a hospedagem de criança ou adolescente desacompanhado dos pais ou
// responsável, salvo se autorizado. O sistema exige ao menos um adulto na
// ficha; a conferência dos documentos de parentesco/autorização continua
// sendo responsabilidade da recepção.
func ValidateParty(guests []*Guest, on kernel.Date) error {
	if len(guests) == 0 {
		return kernel.Invalid("registration.guests_required", "informe ao menos um hóspede").WithField("guests", "obrigatório")
	}
	adults := 0
	for _, guest := range guests {
		if !guest.IsMinor(on) {
			adults++
		}
	}
	if adults == 0 {
		return kernel.Rule("registration.minor_unaccompanied",
			"menores de 18 anos não podem se hospedar desacompanhados de um responsável (ECA, art. 82)")
	}
	return nil
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
