package kernel

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DefaultCurrency é a moeda padrão do sistema (código ISO 4217).
const DefaultCurrency = "BRL"

// Money é um Value Object: imutável, comparável por valor e sem identidade.
//
// O valor é guardado em CENTAVOS (int64). Nunca use float para dinheiro:
// 0.1 + 0.2 != 0.3 em ponto flutuante, e um centavo perdido por arredondamento
// vira diferença de caixa no fim do mês.
type Money struct {
	cents    int64
	currency string
}

// NewMoney cria um valor monetário a partir de centavos.
func NewMoney(cents int64, currency string) Money {
	if currency == "" {
		currency = DefaultCurrency
	}
	return Money{cents: cents, currency: strings.ToUpper(currency)}
}

// BRL é um atalho para valores em reais.
func BRL(cents int64) Money { return NewMoney(cents, DefaultCurrency) }

// Zero devolve zero na moeda informada.
func Zero(currency string) Money { return NewMoney(0, currency) }

func (m Money) Cents() int64 { return m.cents }

func (m Money) Currency() string {
	if m.currency == "" {
		return DefaultCurrency
	}
	return m.currency
}

func (m Money) IsZero() bool     { return m.cents == 0 }
func (m Money) IsNegative() bool { return m.cents < 0 }
func (m Money) IsPositive() bool { return m.cents > 0 }

// Add soma dois valores da mesma moeda.
//
// Somar moedas diferentes é um erro de programação (não uma condição de
// negócio que o chamador possa tratar), por isso entra em pânico — do mesmo
// jeito que acessar um índice fora dos limites de um slice.
func (m Money) Add(other Money) Money {
	m.mustMatch(other)
	return Money{cents: m.cents + other.cents, currency: m.Currency()}
}

// Sub subtrai other de m.
func (m Money) Sub(other Money) Money {
	m.mustMatch(other)
	return Money{cents: m.cents - other.cents, currency: m.Currency()}
}

// Negate inverte o sinal.
func (m Money) Negate() Money { return Money{cents: -m.cents, currency: m.Currency()} }

// GreaterOrEqual informa se m >= other.
func (m Money) GreaterOrEqual(other Money) bool {
	m.mustMatch(other)
	return m.cents >= other.cents
}

// LessThan informa se m < other.
func (m Money) LessThan(other Money) bool {
	m.mustMatch(other)
	return m.cents < other.cents
}

// BasisPoints calcula uma fração do valor em "pontos-base": 1 bp = 0,01%.
// 1500 bps = 15%. Usar bps (inteiro) evita float em taxas e comissões.
// O arredondamento é "half up" (meio centavo arredonda para cima).
func (m Money) BasisPoints(bps int64) Money {
	product := m.cents * bps
	quotient := product / 10000
	remainder := product % 10000
	if remainder >= 5000 {
		quotient++
	} else if remainder <= -5000 {
		quotient--
	}
	return Money{cents: quotient, currency: m.Currency()}
}

// Percent calcula uma porcentagem inteira do valor (30 => 30%).
func (m Money) Percent(percent int) Money { return m.BasisPoints(int64(percent) * 100) }

// Split divide o valor em n partes sem perder centavos: as primeiras partes
// absorvem o resto da divisão. Ex.: R$ 100,00 em 3 => 33,34 + 33,33 + 33,33.
func (m Money) Split(n int) []Money {
	if n <= 0 {
		return nil
	}
	base := m.cents / int64(n)
	remainder := m.cents % int64(n)
	parts := make([]Money, n)
	for i := range parts {
		cents := base
		if int64(i) < remainder {
			cents++
		}
		parts[i] = Money{cents: cents, currency: m.Currency()}
	}
	return parts
}

// String formata para leitura humana em logs (ex.: "BRL 150.00").
func (m Money) String() string {
	sign := ""
	cents := m.cents
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s %s%d.%02d", m.Currency(), sign, cents/100, cents%100)
}

func (m Money) mustMatch(other Money) {
	if m.Currency() != other.Currency() {
		panic(fmt.Sprintf("kernel: operação entre moedas diferentes (%s e %s)", m.Currency(), other.Currency()))
	}
}

type moneyJSON struct {
	Cents    int64  `json:"cents"`
	Currency string `json:"currency"`
}

// MarshalJSON serializa como {"cents": 15000, "currency": "BRL"}.
func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(moneyJSON{Cents: m.cents, Currency: m.Currency()})
}

// UnmarshalJSON lê o formato produzido por MarshalJSON.
func (m *Money) UnmarshalJSON(data []byte) error {
	var raw moneyJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*m = NewMoney(raw.Cents, raw.Currency)
	return nil
}
