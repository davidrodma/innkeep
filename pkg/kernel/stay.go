package kernel

// MaxStayNights limita a duração de uma estadia (evita erros de digitação
// como um check-out no ano errado, que bloquearia o quarto por anos).
const MaxStayNights = 365

// StayPeriod é o período de uma estadia: da data de check-in (inclusive) até
// a data de check-out (exclusive). O hóspede dorme nas noites
// [check-in, check-out) — quem sai dia 12 não ocupa a noite do dia 12, e por
// isso outro hóspede pode entrar no mesmo dia.
//
// É um Value Object: nasce válido (NewStayPeriod valida) e nunca muda.
type StayPeriod struct {
	checkIn  Date
	checkOut Date
}

// NewStayPeriod valida e cria o período.
func NewStayPeriod(checkIn, checkOut Date) (StayPeriod, error) {
	if checkIn.IsZero() || checkOut.IsZero() {
		return StayPeriod{}, Invalid("stay.dates_required", "informe as datas de check-in e check-out")
	}
	if !checkOut.After(checkIn) {
		return StayPeriod{}, Invalid("stay.checkout_before_checkin", "o check-out deve ser posterior ao check-in").
			WithField("check_out", "deve ser posterior ao check-in")
	}
	if checkIn.DaysUntil(checkOut) > MaxStayNights {
		return StayPeriod{}, Invalid("stay.too_long", "a estadia não pode passar de %d noites", MaxStayNights).
			WithField("check_out", "estadia longa demais")
	}
	return StayPeriod{checkIn: checkIn, checkOut: checkOut}, nil
}

// ParseStayPeriod cria o período a partir de duas strings AAAA-MM-DD.
func ParseStayPeriod(checkIn, checkOut string) (StayPeriod, error) {
	in, err := ParseDate(checkIn)
	if err != nil {
		return StayPeriod{}, Invalid("stay.invalid_check_in", "data de check-in inválida").WithField("check_in", "use AAAA-MM-DD")
	}
	out, err := ParseDate(checkOut)
	if err != nil {
		return StayPeriod{}, Invalid("stay.invalid_check_out", "data de check-out inválida").WithField("check_out", "use AAAA-MM-DD")
	}
	return NewStayPeriod(in, out)
}

// MustStayPeriod é para testes e seeds: entra em pânico se o período for inválido.
func MustStayPeriod(checkIn, checkOut string) StayPeriod {
	period, err := ParseStayPeriod(checkIn, checkOut)
	if err != nil {
		panic(err)
	}
	return period
}

func (s StayPeriod) CheckIn() Date  { return s.checkIn }
func (s StayPeriod) CheckOut() Date { return s.checkOut }
func (s StayPeriod) IsZero() bool   { return s.checkIn.IsZero() && s.checkOut.IsZero() }

// Nights devolve o número de diárias.
func (s StayPeriod) Nights() int { return s.checkIn.DaysUntil(s.checkOut) }

// NightDates devolve cada noite da estadia (a data em que a diária começa).
func (s StayPeriod) NightDates() []Date {
	nights := s.Nights()
	dates := make([]Date, 0, nights)
	for i := 0; i < nights; i++ {
		dates = append(dates, s.checkIn.AddDays(i))
	}
	return dates
}

// Overlaps informa se dois períodos disputam alguma noite. Como o intervalo é
// semiaberto [in, out), sair e entrar no mesmo dia NÃO é sobreposição.
func (s StayPeriod) Overlaps(other StayPeriod) bool {
	return s.checkIn.Before(other.checkOut) && other.checkIn.Before(s.checkOut)
}

// Contains informa se a noite de date pertence à estadia.
func (s StayPeriod) Contains(date Date) bool {
	return !date.Before(s.checkIn) && date.Before(s.checkOut)
}

// Equal compara dois períodos por valor.
func (s StayPeriod) Equal(other StayPeriod) bool { return s == other }
