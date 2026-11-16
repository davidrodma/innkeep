// Package domain contém o modelo do contexto de Propriedades: o catálogo
// (propriedades, tipos de acomodação e quartos) e as políticas comerciais.
//
// Este contexto responde "o que a propriedade TEM". Quanto está livre e
// quanto custa em cada data é assunto do contexto de Inventário.
package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// Kind é o tipo de meio de hospedagem.
type Kind string

const (
	KindHotel          Kind = "HOTEL"
	KindPousada        Kind = "POUSADA"
	KindHostel         Kind = "HOSTEL"
	KindInn            Kind = "INN"
	KindApartHotel     Kind = "APART_HOTEL"
	KindVacationRental Kind = "VACATION_RENTAL"
	KindOther          Kind = "OTHER"
)

var validKinds = map[Kind]bool{
	KindHotel: true, KindPousada: true, KindHostel: true, KindInn: true,
	KindApartHotel: true, KindVacationRental: true, KindOther: true,
}

// Erros de domínio do contexto.
var (
	ErrPropertyNotFound = kernel.NotFound("property.not_found", "propriedade não encontrada")
	ErrSlugTaken        = kernel.Conflict("property.slug_taken", "já existe uma propriedade com este endereço (slug)")
	ErrRoomTypeNotFound = kernel.NotFound("room_type.not_found", "tipo de acomodação não encontrado")
	ErrRoomNotFound     = kernel.NotFound("room.not_found", "quarto não encontrado")
	ErrRoomNameTaken    = kernel.Conflict("room.name_taken", "já existe um quarto com este nome na propriedade")
)

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	timePattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
)

// Address é o endereço da propriedade (Value Object).
type Address struct {
	Line       string // logradouro e número
	District   string // bairro
	City       string
	State      string // UF
	PostalCode string // CEP
	Country    string // código ISO de 2 letras
}

// Property é a raiz do agregado: um hotel, pousada, hostel etc.
type Property struct {
	ID          string
	Slug        string // identificador amigável usado na URL pública
	Name        string
	LegalName   string // razão social
	TaxID       string // CNPJ (ou CPF, para pessoa física)
	Cadastur    string // nº do cadastro no Ministério do Turismo
	Kind        Kind
	Description string
	Email       string
	Phone       string
	Address     Address
	Timezone    string // fuso IANA, ex.: "America/Sao_Paulo"
	Currency    string // ISO 4217

	CheckInTime  string // "14:00"
	CheckOutTime string // "12:00"

	// Política de garantia: percentual do total exigido como sinal para
	// confirmar a reserva (0 = não exige) e o prazo, em minutos, que o hóspede
	// tem para pagá-lo antes de a reserva expirar (0 = sem prazo).
	DepositPercent     int
	HoldMinutes        int
	CancellationPolicy string
	PixKey             string // chave Pix informada ao hóspede para pagar o sinal

	// Published controla se a propriedade aparece no site público de reservas.
	Published bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PropertyInput reúne os dados editáveis de uma propriedade.
type PropertyInput struct {
	Slug               string
	Name               string
	LegalName          string
	TaxID              string
	Cadastur           string
	Kind               Kind
	Description        string
	Email              string
	Phone              string
	Address            Address
	Timezone           string
	Currency           string
	CheckInTime        string
	CheckOutTime       string
	DepositPercent     int
	HoldMinutes        int
	CancellationPolicy string
	PixKey             string
	Published          bool
}

// NewProperty cria uma propriedade válida.
func NewProperty(input PropertyInput) (*Property, error) {
	property := &Property{ID: kernel.NewID(), CreatedAt: time.Now().UTC()}
	if err := property.Update(input); err != nil {
		return nil, err
	}
	return property, nil
}

// Update valida e aplica os dados. Toda alteração passa por aqui, então uma
// Property em memória está sempre em estado válido — a essência de um
// agregado no DDD: ele protege as próprias invariantes.
func (p *Property) Update(input PropertyInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if n := utf8.RuneCountInString(input.Name); n < 2 || n > 120 {
		return kernel.Invalid("property.invalid_name", "informe o nome da propriedade").WithField("name", "entre 2 e 120 caracteres")
	}

	input.Slug = strings.TrimSpace(strings.ToLower(input.Slug))
	if input.Slug == "" {
		input.Slug = Slugify(input.Name)
	}
	if !slugPattern.MatchString(input.Slug) || len(input.Slug) > 80 {
		return kernel.Invalid("property.invalid_slug", "endereço (slug) inválido").
			WithField("slug", "use apenas letras minúsculas, números e hífens")
	}

	if input.Kind == "" {
		input.Kind = KindPousada
	}
	if !validKinds[input.Kind] {
		return kernel.Invalid("property.invalid_kind", "tipo de propriedade inválido").WithField("kind", "tipo desconhecido")
	}

	if input.Timezone == "" {
		input.Timezone = "America/Sao_Paulo"
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return kernel.Invalid("property.invalid_timezone", "fuso horário inválido").WithField("timezone", "use um fuso IANA, ex.: America/Sao_Paulo")
	}

	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.Currency == "" {
		input.Currency = kernel.DefaultCurrency
	}
	if len(input.Currency) != 3 {
		return kernel.Invalid("property.invalid_currency", "moeda inválida").WithField("currency", "use o código ISO de 3 letras")
	}

	if input.CheckInTime == "" {
		input.CheckInTime = "14:00"
	}
	if input.CheckOutTime == "" {
		input.CheckOutTime = "12:00"
	}
	if !timePattern.MatchString(input.CheckInTime) {
		return kernel.Invalid("property.invalid_check_in_time", "horário de check-in inválido").WithField("check_in_time", "use HH:MM")
	}
	if !timePattern.MatchString(input.CheckOutTime) {
		return kernel.Invalid("property.invalid_check_out_time", "horário de check-out inválido").WithField("check_out_time", "use HH:MM")
	}

	if input.DepositPercent < 0 || input.DepositPercent > 100 {
		return kernel.Invalid("property.invalid_deposit", "percentual de sinal inválido").WithField("deposit_percent", "entre 0 e 100")
	}
	const maxHoldMinutes = 7 * 24 * 60
	if input.HoldMinutes < 0 || input.HoldMinutes > maxHoldMinutes {
		return kernel.Invalid("property.invalid_hold", "prazo de pagamento do sinal inválido").WithField("hold_minutes", "entre 0 e 10080 (7 dias)")
	}

	p.Slug = input.Slug
	p.Name = input.Name
	p.LegalName = strings.TrimSpace(input.LegalName)
	p.TaxID = strings.TrimSpace(input.TaxID)
	p.Cadastur = strings.TrimSpace(input.Cadastur)
	p.Kind = input.Kind
	p.Description = strings.TrimSpace(input.Description)
	p.Email = strings.TrimSpace(strings.ToLower(input.Email))
	p.Phone = strings.TrimSpace(input.Phone)
	p.Address = input.Address
	p.Timezone = input.Timezone
	p.Currency = input.Currency
	p.CheckInTime = input.CheckInTime
	p.CheckOutTime = input.CheckOutTime
	p.DepositPercent = input.DepositPercent
	p.HoldMinutes = input.HoldMinutes
	p.CancellationPolicy = strings.TrimSpace(input.CancellationPolicy)
	p.PixKey = strings.TrimSpace(input.PixKey)
	p.Published = input.Published
	p.UpdatedAt = time.Now().UTC()
	return nil
}

var accentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// Slugify transforma um nome em slug: "Pousada Recanto da Serra" vira
// "pousada-recanto-da-serra".
func Slugify(name string) string {
	lowered := accentReplacer.Replace(strings.ToLower(strings.TrimSpace(name)))
	var builder strings.Builder
	lastHyphen := true
	for _, r := range lowered {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			builder.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			builder.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(builder.String(), "-")
}
