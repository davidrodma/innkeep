package kernel

import (
	"fmt"
	"time"
)

// DateLayout é o formato ISO 8601 de data usado nas APIs e eventos.
const DateLayout = "2006-01-02"

// Date é uma data civil (ano, mês, dia) SEM horário e SEM fuso.
//
// Por que não usar time.Time? Uma diária é um conceito de calendário: o
// hóspede entra "dia 10" e sai "dia 12", independentemente do fuso do
// servidor. Guardar isso como instante (time.Time) é a origem clássica do bug
// "a reserva apareceu um dia antes" quando alguém converte fusos.
type Date struct {
	year  int
	month time.Month
	day   int
}

// NewDate cria uma data normalizando valores fora do intervalo (dia 32 vira
// dia 1 do mês seguinte, como em time.Date).
func NewDate(year int, month time.Month, day int) Date {
	return DateOf(time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
}

// DateOf extrai a data civil de um instante, no fuso do próprio t.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{year: y, month: m, day: d}
}

// Today devolve a data de hoje no fuso informado (o "hoje" de uma pousada em
// Manaus não é o mesmo "hoje" do servidor em UTC).
func Today(loc *time.Location) Date {
	if loc == nil {
		loc = time.UTC
	}
	return DateOf(time.Now().In(loc))
}

// ParseDate lê uma data no formato AAAA-MM-DD.
func ParseDate(value string) (Date, error) {
	t, err := time.Parse(DateLayout, value)
	if err != nil {
		return Date{}, Invalid("date.invalid", "data inválida %q: use o formato AAAA-MM-DD", value)
	}
	return DateOf(t), nil
}

// MustParseDate é como ParseDate, mas entra em pânico se a data for inválida.
// Use apenas com literais em testes e seeds.
func MustParseDate(value string) Date {
	d, err := ParseDate(value)
	if err != nil {
		panic(err)
	}
	return d
}

func (d Date) Year() int         { return d.year }
func (d Date) Month() time.Month { return d.month }
func (d Date) Day() int          { return d.day }
func (d Date) IsZero() bool      { return d == Date{} }

// Time devolve a meia-noite UTC da data. Útil para gravar em colunas DATE.
func (d Date) Time() time.Time {
	return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, time.UTC)
}

// Weekday devolve o dia da semana.
func (d Date) Weekday() time.Weekday { return d.Time().Weekday() }

// AddDays soma (ou subtrai, se negativo) dias.
func (d Date) AddDays(days int) Date { return DateOf(d.Time().AddDate(0, 0, days)) }

// DaysUntil devolve quantos dias faltam de d até other (negativo se other for anterior).
func (d Date) DaysUntil(other Date) int {
	return int(other.Time().Sub(d.Time()).Hours() / 24)
}

func (d Date) Before(other Date) bool { return d.Time().Before(other.Time()) }
func (d Date) After(other Date) bool  { return d.Time().After(other.Time()) }
func (d Date) Equal(other Date) bool  { return d == other }

// String formata como AAAA-MM-DD.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.year, int(d.month), d.day)
}

// MarshalText permite usar Date em JSON (como string) e como chave de map.
func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// UnmarshalText lê uma data AAAA-MM-DD (string vazia vira a data zero).
func (d *Date) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*d = Date{}
		return nil
	}
	parsed, err := ParseDate(string(text))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
