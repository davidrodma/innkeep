// Comando seed: carrega dados de demonstração pela API pública do InnKeep.
//
// Em vez de inserir linhas direto nos bancos, o seed usa a MESMA API que o
// front-end usa (via gateway). Assim os dados passam por todas as regras de
// negócio e por todos os eventos — o inventário recebe os quartos pelo Kafka,
// o financeiro abre as contas ao ver as reservas, e assim por diante. É
// também um bom roteiro de leitura de como a API é usada de ponta a ponta.
//
// Todos os nomes e dados são FICTÍCIOS. É seguro rodar mais de uma vez: se a
// propriedade de demonstração já existir, nada é criado.
//
// Uso: seed   (variáveis: API_URL, ADMIN_EMAIL, ADMIN_PASSWORD, OTASIM_URL)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const demoSlug = "pousada-recanto-da-serra"

type client struct {
	baseURL string
	token   string
	http    *http.Client
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func main() {
	api := &client{baseURL: env("API_URL", "http://localhost:8080"), http: &http.Client{Timeout: 30 * time.Second}}
	email := env("ADMIN_EMAIL", "admin@innkeep.local")
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		log.Fatal("defina ADMIN_PASSWORD (a senha do administrador inicial, BOOTSTRAP_ADMIN_PASSWORD)")
	}

	// O gateway e os serviços podem ainda estar subindo.
	var login struct {
		AccessToken string `json:"access_token"`
	}
	for attempt := 1; ; attempt++ {
		err := api.call("POST", "/v1/auth/login", map[string]string{"email": email, "password": password}, &login)
		if err == nil {
			break
		}
		if attempt == 30 {
			log.Fatalf("não foi possível fazer login em %s: %v", api.baseURL, err)
		}
		time.Sleep(2 * time.Second)
	}
	api.token = login.AccessToken

	var existing struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
	}
	must(api.call("GET", "/v1/properties", nil, &existing))
	for _, property := range existing.Items {
		if property.Slug == demoSlug {
			log.Println("dados de demonstração já existem; nada a fazer")
			return
		}
	}

	// --- Propriedade ---
	var property struct {
		ID string `json:"id"`
	}
	must(api.call("POST", "/v1/properties", map[string]any{
		"slug": demoSlug, "name": "Pousada Recanto da Serra", "kind": "POUSADA",
		"description": "Pousada familiar de quatro quartos, a poucos minutos do centro. Café da manhã incluso.",
		"email":       "contato@recantodaserra.example", "phone": "+55 11 4000-0000",
		"address": map[string]string{"line": "Rua das Araucárias, 120", "district": "Centro",
			"city": "Serra Negra", "state": "SP", "postal_code": "13930-000", "country": "BR"},
		"timezone": "America/Sao_Paulo", "currency": "BRL", "check_in_time": "14:00", "check_out_time": "12:00",
		// 50% de sinal, com 24 horas para pagar.
		"deposit_percent": 50, "hold_minutes": 1440,
		"cancellation_policy": "Cancelamento gratuito até 7 dias antes da chegada. Depois disso, o sinal não é devolvido.",
		"pix_key":             "contato@recantodaserra.example", "published": true,
	}, &property))
	log.Println("propriedade criada")

	// --- Tipos de acomodação e quartos ---
	createType := func(name, beds string, occupancy int, priceCents int64, order int) string {
		var created struct {
			ID string `json:"id"`
		}
		must(api.call("POST", "/v1/properties/"+property.ID+"/room-types", map[string]any{
			"name": name, "kind": "PRIVATE", "max_occupancy": occupancy, "bed_configuration": beds,
			"base_price_cents": priceCents, "sort_order": order,
			"amenities": []string{"Wi-Fi", "Ventilador de teto", "Roupa de cama"},
		}, &created))
		return created.ID
	}
	bunks := createType("Quarto com beliches", "2 beliches", 4, 7000, 1)
	double := createType("Quarto casal", "1 cama de casal", 2, 10000, 2)
	suite := createType("Suíte", "1 cama de casal + 1 de solteiro, banheiro privativo", 3, 15000, 3)

	createRoom := func(name, typeID string) {
		must(api.call("POST", "/v1/properties/"+property.ID+"/rooms", map[string]any{"name": name, "room_type_id": typeID}, nil))
	}
	createRoom("1", bunks)
	createRoom("2", bunks)
	createRoom("3", double)
	createRoom("4", suite)
	log.Println("tipos de acomodação e quartos criados")

	// --- Taxas por forma de pagamento ---
	must(api.call("PUT", "/v1/fee-rules", map[string]any{"property_id": property.ID, "rules": []map[string]any{
		{"method": "CREDIT_CARD", "bps": 600}, {"method": "DEBIT_CARD", "bps": 200},
		{"method": "PAYMENT_LINK", "bps": 500}, {"method": "BOOKING_COM", "bps": 1500}, {"method": "AIRBNB", "bps": 300},
	}}, nil))

	// --- Canal de venda apontando para o simulador ---
	if simulator := os.Getenv("OTASIM_URL"); simulator != "" {
		var connection struct {
			ID string `json:"id"`
		}
		must(api.call("POST", "/v1/channels/connections", map[string]any{
			"property_id": property.ID, "channel": "BOOKING_COM", "mode": "API", "name": "Booking.com (simulador)",
			"external_property_id": "9000001", "client_id": "innkeep-demo", "client_secret": "simulador", "base_url": simulator,
		}, &connection))
		for external, typeID := range map[string]string{"900101": bunks, "900102": double, "900103": suite} {
			must(api.call("POST", "/v1/channels/connections/"+connection.ID+"/mappings", map[string]any{
				"room_type_id": typeID, "external_room_id": external, "external_rate_id": "1",
			}, nil))
		}
		log.Println("canal Booking.com (simulador) conectado e mapeado")
	}

	// O inventário recebe o catálogo pelo Kafka: aguarda a disponibilidade
	// aparecer antes de criar as reservas (consistência eventual, na prática).
	today := time.Now()
	date := func(offset int) string { return today.AddDate(0, 0, offset).Format("2006-01-02") }
	for attempt := 1; ; attempt++ {
		var availability struct {
			RoomTypes []json.RawMessage `json:"room_types"`
		}
		err := api.call("GET", fmt.Sprintf("/v1/availability?property_id=%s&from=%s&to=%s", property.ID, date(0), date(1)), nil, &availability)
		if err == nil && len(availability.RoomTypes) == 3 {
			break
		}
		if attempt == 30 {
			log.Fatal("o inventário não recebeu o catálogo a tempo; verifique o serviço inventory e o Kafka")
		}
		time.Sleep(time.Second)
	}

	// --- Reservas de exemplo (hóspedes fictícios) ---
	reserve := func(typeID, name, email string, from, to, adults int, confirmNow bool) string {
		var created struct {
			ID string `json:"id"`
		}
		must(api.call("POST", "/v1/reservations", map[string]any{
			"property_id": property.ID, "room_type_id": typeID, "check_in": date(from), "check_out": date(to),
			"adults": adults, "holder": map[string]string{"name": name, "email": email, "phone": "+55 11 90000-0000"},
			"confirm_now": confirmNow,
		}, &created))
		return created.ID
	}
	reserve(suite, "Helena Duarte Prado", "helena@example.com", 0, 3, 2, true)
	reserve(double, "Otávio Ramos Leal", "otavio@example.com", 1, 2, 2, true)
	pending := reserve(bunks, "Lívia Nogueira Sá", "livia@example.com", 2, 5, 3, false)
	reserve(bunks, "Caetano Brandão Luz", "caetano@example.com", 4, 6, 1, true)
	log.Println("reservas criadas")

	// Paga o sinal de uma reserva pendente com DUAS formas de pagamento: o
	// financeiro anuncia "sinal quitado" e o serviço de reservas a confirma.
	var folio struct {
		ID              string `json:"id"`
		DepositDueCents int64  `json:"deposit_due_cents"`
	}
	for attempt := 1; attempt <= 30; attempt++ {
		if err := api.call("GET", "/v1/folios/by-reservation/"+pending, nil, &folio); err == nil && folio.ID != "" {
			break
		}
		time.Sleep(time.Second)
	}
	if folio.ID != "" && folio.DepositDueCents > 0 {
		half := folio.DepositDueCents / 2
		must(api.call("POST", "/v1/folios/"+folio.ID+"/payments", map[string]any{"payments": []map[string]any{
			{"method": "PIX", "amount_cents": half}, {"method": "CASH", "amount_cents": folio.DepositDueCents - half},
		}}, nil))
		log.Println("sinal pago com Pix + dinheiro; a reserva será confirmada pelo evento do financeiro")
	}

	// --- Despesas ---
	expense := func(category, description string, cents int64, offset int) {
		must(api.call("POST", "/v1/expenses", map[string]any{
			"property_id": property.ID, "category": category, "description": description,
			"amount_cents": cents, "due_date": date(offset),
		}, nil))
	}
	expense("UTILITIES", "Conta de luz", 21840, 5)
	expense("SUPPLIES", "Produtos de limpeza e papel", 9870, -1)
	expense("MAINTENANCE", "Troca de disjuntor", 4290, -3)

	log.Println("dados de demonstração carregados com sucesso")
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// call faz uma requisição JSON à API e decodifica a resposta em out.
func (c *client) call(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, raw)
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}
