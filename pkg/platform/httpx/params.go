package httpx

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// PathID lê um parâmetro de rota que deve ser um UUID.
func PathID(r *http.Request, name string) (string, error) {
	value := r.PathValue(name)
	if !kernel.IsID(value) {
		return "", kernel.Invalid("request.invalid_id", "identificador inválido no caminho: %s", name)
	}
	return strings.ToLower(value), nil
}

// QueryID lê um parâmetro de consulta opcional que, se presente, deve ser UUID.
func QueryID(r *http.Request, name string) (string, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return "", nil
	}
	if !kernel.IsID(value) {
		return "", kernel.Invalid("request.invalid_id", "identificador inválido: %s", name).WithField(name, "deve ser um UUID")
	}
	return strings.ToLower(value), nil
}

// RequiredQueryID é como QueryID, mas o parâmetro é obrigatório.
func RequiredQueryID(r *http.Request, name string) (string, error) {
	value, err := QueryID(r, name)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", kernel.Invalid("request.missing_parameter", "parâmetro obrigatório ausente: %s", name).WithField(name, "obrigatório")
	}
	return value, nil
}

// QueryDate lê uma data AAAA-MM-DD opcional da query string.
func QueryDate(r *http.Request, name string) (kernel.Date, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return kernel.Date{}, nil
	}
	date, err := kernel.ParseDate(value)
	if err != nil {
		return kernel.Date{}, kernel.Invalid("request.invalid_date", "data inválida em %s", name).WithField(name, "use AAAA-MM-DD")
	}
	return date, nil
}

// RequiredQueryDate é como QueryDate, mas o parâmetro é obrigatório.
func RequiredQueryDate(r *http.Request, name string) (kernel.Date, error) {
	date, err := QueryDate(r, name)
	if err != nil {
		return kernel.Date{}, err
	}
	if date.IsZero() {
		return kernel.Date{}, kernel.Invalid("request.missing_parameter", "parâmetro obrigatório ausente: %s", name).WithField(name, "obrigatório")
	}
	return date, nil
}

// QueryInt lê um inteiro da query string, aplicando valor padrão e limites.
func QueryInt(r *http.Request, name string, fallback, min, max int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Page descreve a paginação pedida pelo cliente (?limit=&offset=).
type Page struct {
	Limit  int
	Offset int
}

// QueryPage lê limit/offset com padrões e tetos seguros: sem teto, um cliente
// poderia pedir a tabela inteira de uma vez.
func QueryPage(r *http.Request) Page {
	return Page{
		Limit:  QueryInt(r, "limit", 50, 1, 200),
		Offset: QueryInt(r, "offset", 0, 0, 1_000_000),
	}
}
