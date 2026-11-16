package kernel_test

// Testes do Shared Kernel. São "table-driven tests", o estilo idiomático de Go:
// uma tabela de casos (entrada + resultado esperado) percorrida por um laço,
// com t.Run dando nome a cada caso na saída do `go test -v`.

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

func TestMoneyBasisPointsRoundsHalfUp(t *testing.T) {
	tests := []struct {
		name  string
		cents int64
		bps   int64
		want  int64
	}{
		{"15% de R$ 100,00", 10000, 1500, 1500},
		{"50% de R$ 0,01 arredonda para cima", 1, 5000, 1},
		{"13% de R$ 70,00 (comissão)", 7000, 1300, 910},
		{"4,99% de R$ 123,45", 12345, 499, 616},
		{"zero", 0, 1500, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kernel.BRL(tt.cents).BasisPoints(tt.bps).Cents(); got != tt.want {
				t.Errorf("BasisPoints(%d) de %d = %d; esperado %d", tt.bps, tt.cents, got, tt.want)
			}
		})
	}
}

func TestMoneySplitNeverLosesCents(t *testing.T) {
	parts := kernel.BRL(10000).Split(3)
	var sum int64
	for _, part := range parts {
		sum += part.Cents()
	}
	if sum != 10000 {
		t.Fatalf("a soma das partes deveria ser 10000, é %d", sum)
	}
	if parts[0].Cents() != 3334 || parts[2].Cents() != 3333 {
		t.Fatalf("partes inesperadas: %v", parts)
	}
}

func TestMoneyRejectsMixedCurrencies(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("somar moedas diferentes deveria entrar em pânico")
		}
	}()
	kernel.BRL(100).Add(kernel.NewMoney(100, "USD"))
}

func TestErrorsIsSurvivesCopies(t *testing.T) {
	sentinel := kernel.Conflict("demo.conflict", "conflito")
	wrapped := sentinel.Wrap(errors.New("causa no banco"))
	withField := sentinel.WithField("campo", "detalhe")

	// Wrap e WithField devolvem cópias (outros ponteiros): errors.Is precisa
	// continuar reconhecendo o erro original.
	if !errors.Is(wrapped, sentinel) || !errors.Is(withField, sentinel) {
		t.Fatal("errors.Is deveria reconhecer cópias do mesmo erro de domínio")
	}
	if errors.Is(wrapped, kernel.Conflict("outro.codigo", "x")) {
		t.Fatal("códigos diferentes não são o mesmo erro")
	}
	if !errors.Is(fmt.Errorf("contexto: %w", wrapped), sentinel) {
		t.Fatal("errors.Is deveria atravessar camadas de fmt.Errorf")
	}
}

func TestMoneyJSONRoundTrip(t *testing.T) {
	encoded, err := json.Marshal(kernel.BRL(15050))
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"cents":15050,"currency":"BRL"}` {
		t.Fatalf("JSON inesperado: %s", encoded)
	}
	var decoded kernel.Money
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != kernel.BRL(15050) {
		t.Fatalf("ida e volta falhou: %v %v", decoded, err)
	}
}

func TestStayPeriodValidation(t *testing.T) {
	tests := []struct {
		name     string
		in, out  string
		wantCode string
	}{
		{"válido", "2026-11-10", "2026-11-12", ""},
		{"saída igual à entrada", "2026-11-10", "2026-11-10", "stay.checkout_before_checkin"},
		{"saída antes da entrada", "2026-11-10", "2026-11-09", "stay.checkout_before_checkin"},
		{"longa demais", "2026-01-01", "2027-06-01", "stay.too_long"},
		{"data inválida", "2026-02-30", "2026-03-02", "stay.invalid_check_in"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := kernel.ParseStayPeriod(tt.in, tt.out)
			if got := codeOf(err); got != tt.wantCode {
				t.Errorf("código = %q; esperado %q (erro: %v)", got, tt.wantCode, err)
			}
		})
	}
}

func codeOf(err error) string {
	if err == nil {
		return ""
	}
	return kernel.CodeOf(err)
}

func TestStayPeriodNightsAndOverlap(t *testing.T) {
	stay := kernel.MustStayPeriod("2026-11-10", "2026-11-13")
	if stay.Nights() != 3 {
		t.Fatalf("noites = %d; esperado 3", stay.Nights())
	}
	if len(stay.NightDates()) != 3 || stay.NightDates()[2].String() != "2026-11-12" {
		t.Fatalf("noites inesperadas: %v", stay.NightDates())
	}

	tests := []struct {
		name    string
		other   kernel.StayPeriod
		overlap bool
	}{
		// Intervalo semiaberto: sair e entrar no mesmo dia NÃO conflita.
		{"entra no dia da saída", kernel.MustStayPeriod("2026-11-13", "2026-11-15"), false},
		{"sai no dia da entrada", kernel.MustStayPeriod("2026-11-08", "2026-11-10"), false},
		{"uma noite em comum", kernel.MustStayPeriod("2026-11-12", "2026-11-14"), true},
		{"contém o período", kernel.MustStayPeriod("2026-11-01", "2026-11-30"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stay.Overlaps(tt.other); got != tt.overlap {
				t.Errorf("Overlaps = %v; esperado %v", got, tt.overlap)
			}
		})
	}
}

func TestDateArithmeticAcrossBoundaries(t *testing.T) {
	if got := kernel.MustParseDate("2028-02-28").AddDays(1).String(); got != "2028-02-29" {
		t.Errorf("ano bissexto: %s", got)
	}
	if got := kernel.MustParseDate("2026-12-31").AddDays(1).String(); got != "2027-01-01" {
		t.Errorf("virada de ano: %s", got)
	}
	if got := kernel.MustParseDate("2026-01-01").DaysUntil(kernel.MustParseDate("2026-12-31")); got != 364 {
		t.Errorf("DaysUntil = %d", got)
	}
}

func TestIDs(t *testing.T) {
	first, second := kernel.NewID(), kernel.NewID()
	if !kernel.IsID(first) || first == second {
		t.Fatalf("IDs inválidos ou repetidos: %s %s", first, second)
	}
	if first[14] != '7' {
		t.Fatalf("esperado UUID versão 7: %s", first)
	}
	// IDs v7 gerados em sequência são ordenáveis como texto (bons para
	// índices B-tree) — inclusive vários no mesmo milissegundo.
	previous := kernel.NewID()
	for i := 0; i < 10_000; i++ {
		next := kernel.NewID()
		if next <= previous {
			t.Fatalf("IDs v7 deveriam crescer: %s depois de %s", next, previous)
		}
		previous = next
	}

	a := kernel.DeterministicID("ns", "BOOKING_COM:123")
	b := kernel.DeterministicID("ns", "BOOKING_COM:123")
	c := kernel.DeterministicID("ns", "BOOKING_COM:124")
	if a != b || a == c || a[14] != '5' {
		t.Fatalf("IDs determinísticos inesperados: %s %s %s", a, b, c)
	}
}
