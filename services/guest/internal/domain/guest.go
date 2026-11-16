// Package domain contém o modelo do contexto de Hóspedes: o cadastro das
// pessoas e a Ficha Nacional de Registro de Hóspedes (FNRH).
//
// Este é o contexto que concentra os DADOS PESSOAIS do sistema. Mantê-los em
// um serviço próprio, com banco próprio, é uma decisão de segurança e de
// LGPD: o acesso é controlado em um só lugar e os outros serviços trabalham
// apenas com IDs.
package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// DocumentType é o tipo do documento de identificação.
type DocumentType string

const (
	DocumentCPF      DocumentType = "CPF"
	DocumentRG       DocumentType = "RG"
	DocumentPassport DocumentType = "PASSPORT"
	DocumentCNH      DocumentType = "CNH"
	DocumentRNE      DocumentType = "RNE" // registro nacional de estrangeiro / CRNM
	DocumentOther    DocumentType = "OTHER"
)

var validDocuments = map[DocumentType]bool{
	DocumentCPF: true, DocumentRG: true, DocumentPassport: true, DocumentCNH: true, DocumentRNE: true, DocumentOther: true,
}

// Erros de domínio do contexto.
var (
	ErrGuestNotFound        = kernel.NotFound("guest.not_found", "hóspede não encontrado")
	ErrRegistrationNotFound = kernel.NotFound("registration.not_found", "ficha de registro não encontrada")
)

// Address é o endereço de residência permanente (exigido na FNRH).
type Address struct {
	Line       string
	District   string
	City       string
	State      string
	PostalCode string
	Country    string
}

// Guest é uma pessoa hospedada. O mesmo cadastro é reaproveitado em todas as
// estadias dela.
type Guest struct {
	ID          string
	FullName    string
	Email       string
	Phone       string
	BirthDate   kernel.Date
	Gender      string
	Nationality string
	Profession  string

	DocumentType DocumentType
	// DocumentNumber fica em texto puro apenas em memória. No banco é gravado
	// cifrado (veja o repositório).
	DocumentNumber string
	DocumentIssuer string

	Address          Address
	Notes            string
	MarketingConsent bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
	// AnonymizedAt indica que os dados pessoais foram eliminados a pedido do
	// titular (LGPD, art. 18).
	AnonymizedAt *time.Time
}

// GuestInput reúne os dados informados no cadastro.
type GuestInput struct {
	FullName         string
	Email            string
	Phone            string
	BirthDate        kernel.Date
	Gender           string
	Nationality      string
	Profession       string
	DocumentType     DocumentType
	DocumentNumber   string
	DocumentIssuer   string
	Address          Address
	Notes            string
	MarketingConsent bool
}

// NewGuest cria um hóspede válido.
func NewGuest(input GuestInput, today kernel.Date) (*Guest, error) {
	guest := &Guest{ID: kernel.NewID(), CreatedAt: time.Now().UTC()}
	if err := guest.Update(input, today); err != nil {
		return nil, err
	}
	return guest, nil
}

// Update valida e aplica os dados.
func (g *Guest) Update(input GuestInput, today kernel.Date) error {
	input.FullName = strings.Join(strings.Fields(input.FullName), " ")
	if n := utf8.RuneCountInString(input.FullName); n < 3 || n > 150 || !strings.Contains(input.FullName, " ") {
		return kernel.Invalid("guest.invalid_name", "informe o nome completo do hóspede").WithField("full_name", "nome e sobrenome")
	}

	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.Email != "" {
		if address, err := mail.ParseAddress(input.Email); err != nil || address.Address != input.Email {
			return kernel.Invalid("guest.invalid_email", "e-mail inválido").WithField("email", "formato inválido")
		}
	}

	if !input.BirthDate.IsZero() && (input.BirthDate.After(today) || input.BirthDate.Year() < 1900) {
		return kernel.Invalid("guest.invalid_birth_date", "data de nascimento inválida").WithField("birth_date", "data inválida")
	}

	if input.DocumentType == "" {
		input.DocumentType = DocumentCPF
	}
	if !validDocuments[input.DocumentType] {
		return kernel.Invalid("guest.invalid_document_type", "tipo de documento inválido").WithField("document_type", "valor desconhecido")
	}
	input.DocumentNumber = NormalizeDocument(input.DocumentType, input.DocumentNumber)
	if input.DocumentNumber == "" {
		return kernel.Invalid("guest.document_required", "informe o número do documento").WithField("document_number", "obrigatório")
	}
	if input.DocumentType == DocumentCPF && !ValidCPF(input.DocumentNumber) {
		return kernel.Invalid("guest.invalid_cpf", "CPF inválido").WithField("document_number", "CPF inválido")
	}

	g.FullName, g.Email, g.Phone = input.FullName, input.Email, strings.TrimSpace(input.Phone)
	g.BirthDate, g.Gender = input.BirthDate, strings.ToUpper(strings.TrimSpace(input.Gender))
	g.Nationality, g.Profession = strings.TrimSpace(input.Nationality), strings.TrimSpace(input.Profession)
	g.DocumentType, g.DocumentNumber = input.DocumentType, input.DocumentNumber
	g.DocumentIssuer = strings.TrimSpace(input.DocumentIssuer)
	g.Address, g.Notes, g.MarketingConsent = input.Address, strings.TrimSpace(input.Notes), input.MarketingConsent
	g.UpdatedAt = time.Now().UTC()
	return nil
}

// IsMinor informa se o hóspede é menor de 18 anos na data informada.
func (g *Guest) IsMinor(on kernel.Date) bool {
	if g.BirthDate.IsZero() {
		return false
	}
	eighteenth := kernel.NewDate(g.BirthDate.Year()+18, g.BirthDate.Month(), g.BirthDate.Day())
	return on.Before(eighteenth)
}

// Anonymize elimina os dados pessoais, preservando apenas o registro de que
// a pessoa existiu (as fichas de hospedagem continuam apontando para ela).
//
// Atenção: a legislação de turismo obriga a guardar a FNRH por um período.
// A eliminação deve respeitar esse prazo — a decisão de quando anonimizar é
// do controlador dos dados (a propriedade), não do sistema.
func (g *Guest) Anonymize(now time.Time) {
	*g = Guest{
		ID: g.ID, FullName: "Hóspede anonimizado", DocumentType: DocumentOther,
		CreatedAt: g.CreatedAt, UpdatedAt: now, AnonymizedAt: &now,
	}
}

// NormalizeDocument padroniza o número: CPF fica só com dígitos; os demais
// perdem espaços e ficam em maiúsculas.
func NormalizeDocument(documentType DocumentType, number string) string {
	number = strings.TrimSpace(number)
	if documentType == DocumentCPF {
		var digits strings.Builder
		for _, r := range number {
			if r >= '0' && r <= '9' {
				digits.WriteRune(r)
			}
		}
		return digits.String()
	}
	return strings.ToUpper(strings.ReplaceAll(number, " ", ""))
}

// ValidCPF confere os dígitos verificadores de um CPF (11 dígitos).
//
// O algoritmo: os 9 primeiros dígitos são multiplicados por pesos
// decrescentes (10..2), soma-se tudo e o resto da divisão por 11 define o 1º
// dígito verificador; repete-se com 10 dígitos (pesos 11..2) para o 2º.
func ValidCPF(cpf string) bool {
	if len(cpf) != 11 {
		return false
	}
	allEqual := true
	for i := 0; i < 11; i++ {
		if cpf[i] < '0' || cpf[i] > '9' {
			return false
		}
		if cpf[i] != cpf[0] {
			allEqual = false
		}
	}
	if allEqual { // "111.111.111-11" passa na conta, mas não é um CPF válido
		return false
	}
	check := func(length int) byte {
		sum := 0
		for i := 0; i < length; i++ {
			sum += int(cpf[i]-'0') * (length + 1 - i)
		}
		digit := (sum * 10) % 11
		if digit == 10 {
			digit = 0
		}
		return byte('0' + digit)
	}
	return cpf[9] == check(9) && cpf[10] == check(10)
}

// MaskDocument esconde o número, deixando só os dois últimos caracteres —
// para listagens, onde ninguém precisa ver o documento inteiro.
func MaskDocument(number string) string {
	if len(number) <= 2 {
		return strings.Repeat("*", len(number))
	}
	return strings.Repeat("*", len(number)-2) + number[len(number)-2:]
}
