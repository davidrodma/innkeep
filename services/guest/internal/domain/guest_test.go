package domain_test

import (
	"testing"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/guest/internal/domain"
)

func TestValidCPF(t *testing.T) {
	tests := []struct {
		cpf   string
		valid bool
	}{
		{"52998224725", true},  // CPF de exemplo com dígitos verificadores corretos
		{"52998224726", false}, // último dígito errado
		{"11111111111", false}, // todos iguais: passa na conta, mas é inválido
		{"1234567890", false},  // 10 dígitos
		{"5299822472a", false},
	}
	for _, tt := range tests {
		t.Run(tt.cpf, func(t *testing.T) {
			if got := domain.ValidCPF(tt.cpf); got != tt.valid {
				t.Errorf("ValidCPF(%s) = %v; esperado %v", tt.cpf, got, tt.valid)
			}
		})
	}
}

func TestNormalizeAndMaskDocument(t *testing.T) {
	if got := domain.NormalizeDocument(domain.DocumentCPF, "529.982.247-25"); got != "52998224725" {
		t.Errorf("normalização do CPF: %s", got)
	}
	if got := domain.NormalizeDocument(domain.DocumentPassport, " fx 123456 "); got != "FX123456" {
		t.Errorf("normalização do passaporte: %s", got)
	}
	if got := domain.MaskDocument("52998224725"); got != "*********25" {
		t.Errorf("máscara: %s", got)
	}
}

func TestNewGuestValidation(t *testing.T) {
	today := kernel.MustParseDate("2026-11-14")
	valid := domain.GuestInput{FullName: "  Maria   da Silva ", DocumentNumber: "529.982.247-25", Email: "MARIA@EXAMPLE.COM"}

	guest, err := domain.NewGuest(valid, today)
	if err != nil {
		t.Fatal(err)
	}
	if guest.FullName != "Maria da Silva" || guest.Email != "maria@example.com" || guest.DocumentNumber != "52998224725" {
		t.Fatalf("normalização inesperada: %+v", guest)
	}

	tests := []struct {
		name   string
		mutate func(*domain.GuestInput)
		code   string
	}{
		{"sem sobrenome", func(in *domain.GuestInput) { in.FullName = "Maria" }, "guest.invalid_name"},
		{"CPF inválido", func(in *domain.GuestInput) { in.DocumentNumber = "111.111.111-11" }, "guest.invalid_cpf"},
		{"sem documento", func(in *domain.GuestInput) { in.DocumentNumber = "" }, "guest.document_required"},
		{"nascimento no futuro", func(in *domain.GuestInput) { in.BirthDate = kernel.MustParseDate("2030-01-01") }, "guest.invalid_birth_date"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valid
			tt.mutate(&input)
			if _, err := domain.NewGuest(input, today); kernel.CodeOf(err) != tt.code {
				t.Fatalf("código = %q; esperado %q", kernel.CodeOf(err), tt.code)
			}
		})
	}
}

func TestMinorsCannotStayAlone(t *testing.T) {
	checkIn := kernel.MustParseDate("2026-11-20")
	adult := &domain.Guest{BirthDate: kernel.MustParseDate("1990-05-20")}
	minor := &domain.Guest{BirthDate: kernel.MustParseDate("2012-01-10")}
	turns18Tomorrow := &domain.Guest{BirthDate: kernel.MustParseDate("2008-11-21")}

	if err := domain.ValidateParty([]*domain.Guest{minor}, checkIn); kernel.CodeOf(err) != "registration.minor_unaccompanied" {
		t.Fatalf("menor desacompanhado deveria ser recusado (ECA, art. 82): %v", err)
	}
	if err := domain.ValidateParty([]*domain.Guest{minor, adult}, checkIn); err != nil {
		t.Fatalf("menor acompanhado de adulto é permitido: %v", err)
	}
	if !turns18Tomorrow.IsMinor(checkIn) {
		t.Fatal("quem faz 18 anos amanhã ainda é menor hoje")
	}
}
