//go:build e2e

// Package e2e contém os testes de ponta a ponta (end-to-end) do backend.
//
// Diferente dos testes unitários, estes NÃO usam dublês: exercitam a stack
// inteira em execução (gateway, 8 serviços, PostgreSQL, Kafka, Redis e o
// simulador de OTA), sempre pela API pública, como um cliente real faria.
//
// Como os fluxos são assíncronos (sagas por eventos), os testes não esperam
// respostas imediatas: eles consultam a API repetidamente até o estado
// esperado aparecer — exatamente o que o front-end faz. Veja `eventually`.
//
// Para rodar (com a stack no ar — `docker compose up -d`):
//
//	go test -tags=e2e -count=1 ./tests/e2e/...
//
// A tag de build "e2e" mantém estes testes fora do `go test ./...` comum.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	_ "time/tzdata" // base de fusos embutida: o teste não depende do sistema
)

var (
	apiURL         = env("E2E_API_URL", "http://localhost:8080")
	simulatorURL   = env("E2E_OTASIM_URL", "http://localhost:8090")
	simulatorInner = env("E2E_OTASIM_INTERNAL_URL", "http://otasim:8090") // como o serviço channel enxerga o simulador
	adminEmail     = env("E2E_ADMIN_EMAIL", "admin@innkeep.local")
	adminPassword  = env("E2E_ADMIN_PASSWORD", "innkeep-dev-admin")
)

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// ---------------------------------------------------------------------------
// Cliente HTTP mínimo
// ---------------------------------------------------------------------------

type client struct {
	t     *testing.T
	token string
}

type object = map[string]any

// call faz a requisição e devolve status e corpo decodificado.
func (c *client) call(method, url string, body any) (int, object) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("serializando corpo: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	if strings.HasPrefix(url, "/") {
		url = apiURL + url
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		c.t.Fatalf("montando requisição: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var decoded object
	_ = json.Unmarshal(raw, &decoded)
	return resp.StatusCode, decoded
}

// must exige um dos status informados e devolve o corpo.
func (c *client) must(method, url string, body any, statuses ...int) object {
	c.t.Helper()
	status, decoded := c.call(method, url, body)
	for _, expected := range statuses {
		if status == expected {
			return decoded
		}
	}
	c.t.Fatalf("%s %s: status %d (esperado %v): %v", method, url, status, statuses, decoded)
	return nil
}

func login(t *testing.T, email, password string) *client {
	t.Helper()
	anonymous := &client{t: t}
	body := anonymous.must("POST", "/v1/auth/login", object{"email": email, "password": password}, 200)
	return &client{t: t, token: body["access_token"].(string)}
}

// eventually repete `check` até ele devolver true ou o prazo estourar. É a
// forma correta de testar consistência eventual: afirmar "isto VAI acontecer
// em até N segundos", em vez de dormir um tempo fixo e torcer.
func eventually(t *testing.T, timeout time.Duration, description string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if check() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("não aconteceu em %s: %s", timeout, description)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// propertyZone é o fuso das propriedades criadas nos testes. "Hoje" é sempre
// o dia corrente NO FUSO DA PROPRIEDADE — às 22h em São Paulo já é amanhã em
// UTC, e o domínio (corretamente) recusaria um check-in "de amanhã".
var propertyZone = func() *time.Location {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err)
	}
	return location
}()

func day(offset int) string {
	return time.Now().In(propertyZone).AddDate(0, 0, offset).Format("2006-01-02")
}

func cents(value any) int64 { return int64(value.(float64)) }

// ---------------------------------------------------------------------------
// Cenário: cada teste cria a própria propriedade, para não depender de dados
// deixados por outros testes (isolamento).
// ---------------------------------------------------------------------------

type scenario struct {
	admin      *client
	propertyID string
	slug       string
	roomTypeID string
	roomIDs    []string
}

var slugCounter sync.Mutex

func newScenario(t *testing.T, rooms int, depositPercent int) *scenario {
	t.Helper()
	admin := login(t, adminEmail, adminPassword)

	slugCounter.Lock()
	slug := fmt.Sprintf("e2e-%d", time.Now().UnixNano())
	slugCounter.Unlock()

	property := admin.must("POST", "/v1/properties", object{
		"slug": slug, "name": "Pousada E2E " + slug, "kind": "POUSADA", "published": true,
		"deposit_percent": depositPercent, "hold_minutes": 60, "pix_key": "e2e@example.com",
	}, 201)
	s := &scenario{admin: admin, propertyID: property["id"].(string), slug: slug}

	roomType := admin.must("POST", "/v1/properties/"+s.propertyID+"/room-types", object{
		"name": "Quarto E2E", "max_occupancy": 3, "base_price_cents": 10000,
	}, 201)
	s.roomTypeID = roomType["id"].(string)
	for i := 1; i <= rooms; i++ {
		room := admin.must("POST", "/v1/properties/"+s.propertyID+"/rooms", object{
			"name": fmt.Sprint(100 + i), "room_type_id": s.roomTypeID,
		}, 201)
		s.roomIDs = append(s.roomIDs, room["id"].(string))
	}

	// O inventário e o serviço de reservas recebem o cadastro pelo Kafka.
	eventually(t, 60*time.Second, "inventário receber o catálogo", func() bool {
		status, body := admin.call("GET", fmt.Sprintf("/v1/availability?property_id=%s&from=%s&to=%s", s.propertyID, day(1), day(2)), nil)
		if status != 200 {
			return false
		}
		types, _ := body["room_types"].([]any)
		if len(types) != 1 {
			return false
		}
		days := types[0].(object)["days"].([]any)
		return len(days) == 1 && int(days[0].(object)["total"].(float64)) == rooms
	})
	return s
}

func (s *scenario) reserve(t *testing.T, checkIn, checkOut int, extra object) string {
	t.Helper()
	body := object{
		"property_id": s.propertyID, "room_type_id": s.roomTypeID,
		"check_in": day(checkIn), "check_out": day(checkOut), "adults": 2,
		"holder": object{"name": "Hóspede de Teste", "email": "hospede@example.com"},
	}
	for key, value := range extra {
		body[key] = value
	}
	var created object
	// O serviço de reservas também aprende a propriedade por evento: até lá,
	// responde 404 para ela.
	eventually(t, 60*time.Second, "serviço de reservas conhecer a propriedade", func() bool {
		status, response := s.admin.call("POST", "/v1/reservations", body)
		created = response
		return status == 202
	})
	return created["id"].(string)
}

func (s *scenario) reservation(id string) object {
	return s.admin.must("GET", "/v1/reservations/"+id, nil, 200)
}

func (s *scenario) waitStatus(t *testing.T, id, expected string) object {
	t.Helper()
	var current object
	eventually(t, 30*time.Second, "reserva chegar a "+expected, func() bool {
		current = s.reservation(id)
		return current["status"] == expected
	})
	return current
}

func (s *scenario) folio(t *testing.T, reservationID string) object {
	t.Helper()
	var folio object
	eventually(t, 30*time.Second, "financeiro abrir a conta", func() bool {
		status, body := s.admin.call("GET", "/v1/folios/by-reservation/"+reservationID, nil)
		folio = body
		return status == 200
	})
	return folio
}

// ---------------------------------------------------------------------------
// Testes
// ---------------------------------------------------------------------------

// TestReservationLifecycle percorre a vida inteira de uma reserva, cruzando
// quatro serviços: reservas, inventário, financeiro e hóspedes.
func TestReservationLifecycle(t *testing.T) {
	t.Parallel()
	s := newScenario(t, 1, 50)

	// Entrada hoje, 2 noites a R$ 100 => total R$ 200, sinal de 50% = R$ 100.
	id := s.reserve(t, 0, 2, nil)
	pending := s.waitStatus(t, id, "PENDING")
	if cents(pending["total_cents"]) != 20000 || cents(pending["deposit_due_cents"]) != 10000 {
		t.Fatalf("total/sinal inesperados: %v / %v", pending["total_cents"], pending["deposit_due_cents"])
	}
	if pending["room_id"] == "" {
		t.Fatal("o inventário deveria ter atribuído um quarto")
	}

	// Pagamento parcial, abaixo do sinal: a reserva continua pendente.
	folio := s.folio(t, id)
	folioID := folio["id"].(string)
	s.admin.must("POST", "/v1/folios/"+folioID+"/payments", object{"payments": []object{
		{"method": "PIX", "amount_cents": 4000},
	}}, 201)
	time.Sleep(time.Second)
	if status := s.reservation(id)["status"]; status != "PENDING" {
		t.Fatalf("com sinal incompleto a reserva deveria seguir PENDING, está %v", status)
	}

	// Completa o sinal com DUAS formas de pagamento em uma única operação.
	s.admin.must("POST", "/v1/folios/"+folioID+"/payments", object{"payments": []object{
		{"method": "CASH", "amount_cents": 3000}, {"method": "CREDIT_CARD", "amount_cents": 3000},
	}}, 201)
	s.waitStatus(t, id, "CONFIRMED") // billing -> evento -> reservation

	// Check-in sem a ficha de registro (FNRH) é recusado: regra de negócio.
	status, problem := s.admin.call("POST", "/v1/reservations/"+id+"/check-in", nil)
	if status != 422 || problem["code"] != "reservation.registration_required" {
		t.Fatalf("check-in sem FNRH deveria falhar com 422: %d %v", status, problem)
	}

	// Preenche a FNRH; o serviço de hóspedes avisa por evento.
	s.admin.must("PUT", "/v1/registrations/"+id, object{
		"property_id": s.propertyID, "check_in": day(0), "check_out": day(2),
		"guests": []object{{
			"full_name": "Hóspede de Teste", "document_type": "CPF", "document_number": "529.982.247-25",
			"birth_date": "1990-05-20", "primary": true, "purpose_of_trip": "LEISURE", "transport": "CAR",
		}},
	}, 200)
	eventually(t, 30*time.Second, "reserva registrar a FNRH", func() bool {
		return s.reservation(id)["registration_completed"] == true
	})

	checkedIn := s.admin.must("POST", "/v1/reservations/"+id+"/check-in", nil, 200)
	if checkedIn["status"] != "CHECKED_IN" {
		t.Fatalf("status após check-in: %v", checkedIn["status"])
	}

	// Lança um extra, quita o saldo e faz o check-out.
	s.admin.must("POST", "/v1/folios/"+folioID+"/charges", object{"description": "Frigobar", "amount_cents": 1500}, 201)
	folio = s.admin.must("GET", "/v1/folios/"+folioID, nil, 200)
	if balance := cents(folio["balance_cents"]); balance != 11500 { // 200 + 15 - 100 pagos
		t.Fatalf("saldo esperado 11500, obtido %d", balance)
	}
	s.admin.must("POST", "/v1/folios/"+folioID+"/payments", object{"payments": []object{
		{"method": "PIX", "amount_cents": 11500},
	}}, 201)
	s.admin.must("POST", "/v1/reservations/"+id+"/check-out", nil, 200)
	eventually(t, 30*time.Second, "conta ser fechada após o check-out", func() bool {
		return s.admin.must("GET", "/v1/folios/"+folioID, nil, 200)["status"] == "CLOSED"
	})

	// O histórico é o próprio fluxo de eventos do Event Store, em ordem.
	history := s.admin.must("GET", "/v1/reservations/"+id+"/history", nil, 200)["items"].([]any)
	var types []string
	for _, entry := range history {
		types = append(types, entry.(object)["type"].(string))
	}
	expected := "reservation.requested,reservation.room_allocated,reservation.confirmed," +
		"reservation.registration_completed,reservation.checked_in,reservation.checked_out"
	if strings.Join(types, ",") != expected {
		t.Fatalf("histórico inesperado:\n obtido:   %s\n esperado: %s", strings.Join(types, ","), expected)
	}
}

// TestNoOverbooking dispara vários pedidos simultâneos para o ÚNICO quarto de
// uma propriedade, nas mesmas datas. Exatamente um pode vencer.
func TestNoOverbooking(t *testing.T) {
	t.Parallel()
	s := newScenario(t, 1, 0)
	// Reserva em outras datas: só garante que o serviço de reservas já conhece
	// a propriedade antes da disputa começar.
	warmUp := s.reserve(t, 20, 21, nil)

	const competitors = 8
	ids := make([]string, competitors)
	var group sync.WaitGroup
	for i := range ids {
		group.Add(1)
		go func() {
			defer group.Done()
			body := s.admin.must("POST", "/v1/reservations", object{
				"property_id": s.propertyID, "room_type_id": s.roomTypeID,
				"check_in": day(6), "check_out": day(8), "adults": 1,
				"holder": object{"name": fmt.Sprintf("Concorrente %d", i)},
			}, 202)
			ids[i] = body["id"].(string)
		}()
	}
	group.Wait()

	confirmed, rejected := 0, 0
	for _, id := range append(ids, warmUp) {
		eventually(t, 30*time.Second, "saga concluir", func() bool {
			status := s.reservation(id)["status"]
			return status == "CONFIRMED" || status == "REJECTED"
		})
		if s.reservation(id)["status"] == "CONFIRMED" {
			confirmed++
		} else {
			rejected++
		}
	}
	// A reserva de aquecimento e UMA das concorrentes são confirmadas; as demais, rejeitadas.
	if confirmed != 2 || rejected != competitors-1 {
		t.Fatalf("esperado 2 confirmadas e %d rejeitadas; obtido %d e %d", competitors-1, confirmed, rejected)
	}
}

// TestPublicBooking cobre o caminho do hóspede no site público.
func TestPublicBooking(t *testing.T) {
	t.Parallel()
	s := newScenario(t, 2, 30)
	s.reserve(t, 20, 21, nil) // aquece a réplica de políticas no serviço de reservas
	visitor := &client{t: t}

	search := visitor.must("GET", fmt.Sprintf("/v1/public/availability?property_id=%s&check_in=%s&check_out=%s&adults=2",
		s.propertyID, day(10), day(13)), nil, 200)
	offer := search["offers"].([]any)[0].(object)
	if offer["bookable"] != true || cents(offer["total_cents"]) != 30000 {
		t.Fatalf("oferta inesperada: %v", offer)
	}

	// O cliente tenta impor preço e confirmação: campos ignorados no site público.
	created := visitor.must("POST", "/v1/public/reservations", object{
		"property_id": s.propertyID, "room_type_id": s.roomTypeID,
		"check_in": day(10), "check_out": day(13), "adults": 2,
		"holder":             object{"name": "Visitante do Site", "email": "Visitante@Example.com"},
		"quoted_total_cents": 100, "confirm_now": true,
	}, 202)
	code := created["code"].(string)

	var view object
	eventually(t, 30*time.Second, "reserva pública ficar pendente", func() bool {
		_, view = visitor.call("POST", "/v1/public/reservations/lookup", object{"code": code, "email": "visitante@example.com"})
		return view["status"] == "PENDING"
	})
	if cents(view["total_cents"]) != 30000 || cents(view["deposit_due_cents"]) != 9000 {
		t.Fatalf("preço deveria vir da tarifa (30000) com sinal de 30%% (9000): %v", view)
	}

	// Localizador certo com e-mail errado não revela a reserva.
	if status, _ := visitor.call("POST", "/v1/public/reservations/lookup", object{"code": code, "email": "outro@example.com"}); status != 404 {
		t.Fatalf("consulta com e-mail errado deveria dar 404, deu %d", status)
	}
	// Rotas administrativas exigem autenticação.
	if status, _ := visitor.call("GET", "/v1/reservations?property_id="+s.propertyID, nil); status != 401 {
		t.Fatalf("rota administrativa sem token deveria dar 401, deu %d", status)
	}
}

// TestChannelSync cobre a integração com canais, usando o simulador de OTA:
// disponibilidade saindo, reserva entrando e cancelamento entrando.
func TestChannelSync(t *testing.T) {
	t.Parallel()
	s := newScenario(t, 2, 0)
	simulator := &client{t: t}
	externalRoom := "e2e-" + s.slug

	connection := s.admin.must("POST", "/v1/channels/connections", object{
		"property_id": s.propertyID, "channel": "BOOKING_COM", "mode": "API",
		"external_property_id": s.slug, "client_id": "e2e", "client_secret": "e2e", "base_url": simulatorInner,
	}, 201)
	s.admin.must("POST", "/v1/channels/connections/"+connection["id"].(string)+"/mappings", object{
		"room_type_id": s.roomTypeID, "external_room_id": externalRoom, "external_rate_id": "1",
	}, 201)

	availableAt := func(offset int) (int, bool) {
		_, state := simulator.call("GET", simulatorURL+"/sim/state", nil)
		for _, room := range state["rooms"].([]any) {
			if room.(object)["room_id"] != externalRoom {
				continue
			}
			for _, entry := range room.(object)["days"].([]any) {
				if entry.(object)["date"] == day(offset) && entry.(object)["available"] != nil {
					return int(entry.(object)["available"].(float64)), true
				}
			}
		}
		return 0, false
	}

	// 1. Mapear o canal dispara o envio completo da disponibilidade.
	eventually(t, 60*time.Second, "canal receber a disponibilidade inicial", func() bool {
		available, known := availableAt(3)
		return known && available == 2
	})

	// 2. Uma reserva direta no InnKeep reduz a disponibilidade no canal.
	direct := s.reserve(t, 3, 4, nil)
	s.waitStatus(t, direct, "CONFIRMED")
	eventually(t, 30*time.Second, "canal refletir a reserva direta", func() bool {
		available, _ := availableAt(3)
		return available == 1
	})

	// 3. Uma reserva feita NO CANAL entra no InnKeep.
	booked := simulator.must("POST", simulatorURL+"/sim/reservations", object{
		"hotel_id": s.slug, "room_id": externalRoom, "check_in": day(3), "check_out": day(5),
		"guest_name": "Cliente Booking", "email": "cliente@example.com", "total_cents": 26000,
	}, 201)
	var imported object
	eventually(t, 90*time.Second, "reserva do canal entrar no InnKeep", func() bool {
		list := s.admin.must("GET", "/v1/reservations?property_id="+s.propertyID+"&q="+booked["id"].(string), nil, 200)
		items := list["items"].([]any)
		if len(items) == 0 {
			return false
		}
		imported = items[0].(object)
		return imported["status"] == "CONFIRMED"
	})
	if imported["source"] != "BOOKING_COM" || cents(imported["total_cents"]) != 26000 {
		t.Fatalf("reserva importada inesperada: %v", imported)
	}
	eventually(t, 30*time.Second, "canal ficar sem disponibilidade no dia 3", func() bool {
		available, _ := availableAt(3)
		return available == 0
	})

	// 4. O cancelamento no canal cancela a reserva e devolve o quarto.
	simulator.must("POST", simulatorURL+"/sim/reservations/"+booked["id"].(string)+"/cancel", nil, 204)
	eventually(t, 90*time.Second, "cancelamento do canal chegar ao InnKeep", func() bool {
		return s.reservation(imported["id"].(string))["status"] == "CANCELLED"
	})
	eventually(t, 30*time.Second, "canal recuperar a disponibilidade", func() bool {
		available, _ := availableAt(4)
		return available == 2
	})
}

// TestAccessControl confere a autorização por papel (RBAC).
func TestAccessControl(t *testing.T) {
	t.Parallel()
	admin := login(t, adminEmail, adminPassword)
	email := fmt.Sprintf("recepcao-%d@example.com", time.Now().UnixNano())
	admin.must("POST", "/v1/users", object{
		"email": email, "name": "Recepção E2E", "password": "senha-de-teste-123", "role": "RECEPTIONIST",
	}, 201)

	receptionist := login(t, email, "senha-de-teste-123")
	if status, _ := receptionist.call("GET", "/v1/users", nil); status != 403 {
		t.Fatalf("recepção não pode gerenciar usuários: esperado 403, obtido %d", status)
	}
	if status, _ := receptionist.call("GET", "/v1/properties", nil); status != 200 {
		t.Fatalf("recepção deve poder listar propriedades: obtido %d", status)
	}

	// Senha errada repetidas vezes bloqueia o login (força bruta).
	anonymous := &client{t: t}
	lastStatus := 0
	for attempt := 0; attempt < 6; attempt++ {
		lastStatus, _ = anonymous.call("POST", "/v1/auth/login", object{"email": email, "password": "errada"})
	}
	if lastStatus != 429 {
		t.Fatalf("após várias falhas o login deveria responder 429, respondeu %d", lastStatus)
	}
}
