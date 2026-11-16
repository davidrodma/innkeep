// Comando otasim: simulador de OTA (agência de viagens online) para
// desenvolvimento e testes.
//
// A API da Booking.com só atende parceiros certificados, então não dá para
// desenvolver contra ela. Este simulador fala o MESMO protocolo (mesmos
// caminhos, mesmo XML, mesma autenticação por token) que o adaptador
// services/channel/internal/adapter/out/bookingcom espera — o que permite
// exercitar o fluxo inteiro na sua máquina:
//
//	reserva no InnKeep -> evento no Kafka -> serviço channel -> XML -> simulador
//	reserva no simulador -> fila -> serviço channel -> evento -> reserva no InnKeep
//
// Também publica calendários iCal no estilo do Airbnb.
//
// Abra http://localhost:8090 para ver o que o "canal" recebeu e criar
// reservas de teste. NÃO é um substituto da certificação com a Booking.com:
// ele implementa apenas o subconjunto do protocolo que o InnKeep usa.
package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// simulator guarda todo o estado em memória (some ao reiniciar).
type simulator struct {
	mu sync.Mutex
	// availability[quarto][data] = quartos à venda; rates[quarto][data] = centavos.
	availability map[string]map[string]int
	rates        map[string]map[string]int64
	reservations []*reservation
	nextID       int
	calls        []string
	// calendars[anúncio] = reservas do "Airbnb" publicadas em iCal.
	calendars map[string][]icalEvent
}

type reservation struct {
	ID         string `json:"id"`
	HotelID    string `json:"hotel_id"`
	RoomID     string `json:"room_id"`
	CheckIn    string `json:"check_in"`
	CheckOut   string `json:"check_out"`
	Adults     int    `json:"adults"`
	GuestName  string `json:"guest_name"`
	Email      string `json:"email"`
	TotalCents int64  `json:"total_cents"`
	Cancelled  bool   `json:"cancelled"`
	// pending: fila em que a mensagem aguarda confirmação ("new", "modify" ou "").
	Pending string `json:"pending"`
}

type icalEvent struct {
	UID      string
	CheckIn  string
	CheckOut string
}

func main() {
	sim := &simulator{
		availability: map[string]map[string]int{}, rates: map[string]map[string]int64{},
		calendars: map[string][]icalEvent{},
		// Numeração baseada no relógio: reiniciar o simulador não reaproveita
		// números de reserva já usados (como em um canal de verdade).
		nextID: 4_000_000_000 + int(time.Now().Unix()%100_000_000)*10,
	}
	mux := http.NewServeMux()

	// --- Protocolo da Booking.com (subconjunto) ---
	mux.HandleFunc("POST /token-based-authentication/exchange", sim.exchangeToken)
	mux.HandleFunc("POST /hotels/ota/OTA_HotelAvailNotif", sim.authenticated(sim.availNotif))
	mux.HandleFunc("POST /hotels/ota/OTA_HotelRateAmountNotif", sim.authenticated(sim.rateNotif))
	mux.HandleFunc("GET /hotels/ota/OTA_HotelResNotif", sim.authenticated(sim.pull("new")))
	mux.HandleFunc("POST /hotels/ota/OTA_HotelResNotif", sim.authenticated(sim.acknowledge("new")))
	mux.HandleFunc("GET /hotels/ota/OTA_HotelResModifyNotif", sim.authenticated(sim.pull("modify")))
	mux.HandleFunc("POST /hotels/ota/OTA_HotelResModifyNotif", sim.authenticated(sim.acknowledge("modify")))

	// --- Controles do simulador ---
	mux.HandleFunc("GET /{$}", sim.dashboard)
	mux.HandleFunc("GET /sim/state", sim.state)
	mux.HandleFunc("POST /sim/reservations", sim.createReservation)
	mux.HandleFunc("POST /sim/reservations/{id}/cancel", sim.cancelReservation)
	mux.HandleFunc("POST /sim/ical/{listing}/reservations", sim.createCalendarEvent)
	mux.HandleFunc("GET /sim/ical/{listing}", sim.calendar)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") })

	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	log.Printf("simulador de OTA ouvindo em %s", addr) //nolint:gosec // G706: addr vem de variável de ambiente, não de uma requisição
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}

// healthcheck responde ao HEALTHCHECK do Docker Compose (0 = saudável). A
// imagem é "distroless", sem curl nem wget: o próprio binário consulta o /healthz.
func healthcheck() int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:8090/healthz", nil)
	if err != nil {
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func (s *simulator) logCall(format string, args ...any) {
	entry := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
	s.calls = append([]string{entry}, s.calls...)
	if len(s.calls) > 60 {
		s.calls = s.calls[:60]
	}
}

// ---------------------------------------------------------------------------
// Protocolo da Booking.com
// ---------------------------------------------------------------------------

func (s *simulator) exchangeToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ClientID == "" || body.ClientSecret == "" {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	s.mu.Lock()
	s.logCall("autenticação: token emitido para %s", body.ClientID)
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"jwt": "sim-token-" + body.ClientID, "ruid": "SIM-RUID"})
}

func (s *simulator) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer sim-token-") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

type control struct {
	Start        string `xml:"Start,attr"`
	End          string `xml:"End,attr"`
	InvTypeCode  string `xml:"InvTypeCode,attr"`
	RatePlanCode string `xml:"RatePlanCode,attr"`
}

// eachDay percorre as datas de Start até End, INCLUSIVE (convenção da Booking).
func eachDay(start, end string, fn func(day string)) {
	from, errFrom := time.Parse("2006-01-02", start)
	to, errTo := time.Parse("2006-01-02", end)
	if errFrom != nil || errTo != nil {
		return
	}
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		fn(day.Format("2006-01-02"))
	}
}

func (s *simulator) availNotif(w http.ResponseWriter, r *http.Request) {
	var message struct {
		Items []struct {
			BookingLimit *int    `xml:"BookingLimit,attr"`
			Control      control `xml:"StatusApplicationControl"`
		} `xml:"AvailStatusMessages>AvailStatusMessage"`
	}
	if err := xml.NewDecoder(r.Body).Decode(&message); err != nil {
		writeXML(w, http.StatusBadRequest, `<OTA_HotelAvailNotifRS><Errors><Error Code="400" ShortText="XML invalido"/></Errors></OTA_HotelAvailNotifRS>`)
		return
	}
	s.mu.Lock()
	for _, item := range message.Items {
		if item.BookingLimit == nil {
			continue // mensagem só de restrições
		}
		if s.availability[item.Control.InvTypeCode] == nil {
			s.availability[item.Control.InvTypeCode] = map[string]int{}
		}
		eachDay(item.Control.Start, item.Control.End, func(day string) {
			s.availability[item.Control.InvTypeCode][day] = *item.BookingLimit
		})
		s.logCall("disponibilidade: quarto %s = %d de %s a %s", item.Control.InvTypeCode, *item.BookingLimit, item.Control.Start, item.Control.End)
	}
	s.mu.Unlock()
	writeXML(w, http.StatusOK, `<OTA_HotelAvailNotifRS Version="1.004"><Success/></OTA_HotelAvailNotifRS>`)
}

func (s *simulator) rateNotif(w http.ResponseWriter, r *http.Request) {
	var message struct {
		Items []struct {
			Control control `xml:"StatusApplicationControl"`
			Amount  struct {
				AfterTax int64 `xml:"AmountAfterTax,attr"`
			} `xml:"Rates>Rate>BaseByGuestAmts>BaseByGuestAmt"`
		} `xml:"RateAmountMessages>RateAmountMessage"`
	}
	if err := xml.NewDecoder(r.Body).Decode(&message); err != nil {
		writeXML(w, http.StatusBadRequest, `<OTA_HotelRateAmountNotifRS><Errors><Error Code="400" ShortText="XML invalido"/></Errors></OTA_HotelRateAmountNotifRS>`)
		return
	}
	s.mu.Lock()
	for _, item := range message.Items {
		if s.rates[item.Control.InvTypeCode] == nil {
			s.rates[item.Control.InvTypeCode] = map[string]int64{}
		}
		eachDay(item.Control.Start, item.Control.End, func(day string) {
			s.rates[item.Control.InvTypeCode][day] = item.Amount.AfterTax
		})
		s.logCall("preço: quarto %s = %d centavos de %s a %s", item.Control.InvTypeCode, item.Amount.AfterTax, item.Control.Start, item.Control.End)
	}
	s.mu.Unlock()
	writeXML(w, http.StatusOK, `<OTA_HotelRateAmountNotifRS><Success/></OTA_HotelRateAmountNotifRS>`)
}

// pull devolve as reservas que aguardam confirmação em uma das filas.
func (s *simulator) pull(queue string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		root, list, item := "OTA_HotelResNotifRQ", "HotelReservations", "HotelReservation"
		if queue == "modify" {
			root, list, item = "OTA_HotelResModifyNotifRQ", "HotelResModifies", "HotelResModify"
		}
		var body strings.Builder
		fmt.Fprintf(&body, `<%s xmlns="http://www.opentravel.org/OTA/2003/05" Version="2.001"><%s>`, root, list)
		hotels := r.URL.Query().Get("hotel_ids")
		for _, res := range s.reservations {
			if res.Pending != queue || !belongsTo(res, hotels) {
				continue
			}
			fmt.Fprintf(&body, "<%s>", item)
			if !res.Cancelled {
				body.WriteString(`<RoomStays><RoomStay IndexNumber="1">`)
				fmt.Fprintf(&body, `<RoomTypes><RoomType RoomTypeCode="%s"/></RoomTypes><RoomRates>`, xmlEscape(res.RoomID))
				// Uma diária por noite, como nos exemplos da documentação.
				nights := 0
				eachDay(res.CheckIn, res.CheckOut, func(string) { nights++ })
				nights-- // CheckOut é exclusivo aqui
				from, _ := time.Parse("2006-01-02", res.CheckIn)
				for night := 0; night < nights; night++ {
					fmt.Fprintf(&body, `<RoomRate EffectiveDate="%s" RatePlanCode="1"/>`, from.AddDate(0, 0, night).Format("2006-01-02"))
				}
				fmt.Fprintf(&body, `</RoomRates><GuestCounts><GuestCount Count="%d" AgeQualifyingCode="10"/></GuestCounts>`, res.Adults)
				fmt.Fprintf(&body, `<Total AmountAfterTax="%d" DecimalPlaces="2" CurrencyCode="BRL"/></RoomStay></RoomStays>`, res.TotalCents)
			}
			given, surname, _ := strings.Cut(res.GuestName, " ")
			fmt.Fprintf(&body, `<ResGlobalInfo><HotelReservationIDs><HotelReservationID ResID_Value="%s" ResID_Type="14" ResID_Source="BOOKING.COM"/></HotelReservationIDs>`, res.ID)
			fmt.Fprintf(&body, `<Total AmountAfterTax="%d" DecimalPlaces="2" CurrencyCode="BRL"/>`, res.TotalCents)
			fmt.Fprintf(&body, `<Profiles><ProfileInfo><Profile><Customer><PersonName><GivenName>%s</GivenName><Surname>%s</Surname></PersonName><Email>%s</Email></Customer></Profile></ProfileInfo></Profiles>`,
				xmlEscape(given), xmlEscape(surname), xmlEscape(res.Email))
			fmt.Fprintf(&body, "</ResGlobalInfo></%s>", item)
		}
		fmt.Fprintf(&body, "</%s></%s>", list, root)
		writeXML(w, http.StatusOK, body.String())
	}
}

// belongsTo informa se a reserva pertence a algum dos hotéis do filtro
// `hotel_ids` (lista separada por vírgulas). Na API real cada propriedade tem a
// sua fila; sem esse filtro, uma conexão consumiria as reservas de outra.
// Reservas criadas sem hotel_id são entregues a qualquer consulta.
func belongsTo(res *reservation, hotels string) bool {
	if hotels == "" || res.HotelID == "" {
		return true
	}
	for _, hotel := range strings.Split(hotels, ",") {
		if strings.TrimSpace(hotel) == res.HotelID {
			return true
		}
	}
	return false
}

// acknowledge retira da fila as reservas confirmadas pelo InnKeep.
func (s *simulator) acknowledge(queue string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var nested struct {
			New      []ackEntry `xml:"HotelReservations>HotelReservation"`
			Modified []ackEntry `xml:"HotelResModifies>HotelResModify"`
		}
		if err := xml.NewDecoder(r.Body).Decode(&nested); err != nil {
			writeXML(w, http.StatusBadRequest, `<Errors><Error Code="400" ShortText="XML invalido"/></Errors>`)
			return
		}
		s.mu.Lock()
		for _, entry := range append(nested.New, nested.Modified...) {
			for _, res := range s.reservations {
				if res.ID == entry.ID.Value && res.Pending == queue {
					res.Pending = ""
					s.logCall("reserva %s: recebimento confirmado pelo InnKeep", res.ID)
				}
			}
		}
		s.mu.Unlock()
		writeXML(w, http.StatusOK, `<OTA_HotelResNotifRS Version="2.001"><Success/></OTA_HotelResNotifRS>`)
	}
}

type ackEntry struct {
	ID struct {
		Value string `xml:"ResID_Value,attr"`
	} `xml:"ResGlobalInfo>HotelReservationIDs>HotelReservationID"`
}

func writeXML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprint(w, xml.Header+body)
}

func xmlEscape(value string) string {
	var builder strings.Builder
	_ = xml.EscapeText(&builder, []byte(value))
	return builder.String()
}

// ---------------------------------------------------------------------------
// Controles do simulador
// ---------------------------------------------------------------------------

// createReservation simula um hóspede reservando no site da OTA. Como uma OTA
// de verdade, só vende se a disponibilidade recebida do InnKeep permitir.
func (s *simulator) createReservation(w http.ResponseWriter, r *http.Request) {
	var res reservation
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil || res.RoomID == "" || res.CheckIn == "" || res.CheckOut == "" {
		http.Error(w, `{"error":"informe room_id, check_in e check_out"}`, http.StatusBadRequest)
		return
	}
	if res.Adults < 1 {
		res.Adults = 2
	}
	if res.GuestName == "" {
		res.GuestName = "Hospede Simulado"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	nights := []string{}
	checkOut, _ := time.Parse("2006-01-02", res.CheckOut)
	eachDay(res.CheckIn, checkOut.AddDate(0, 0, -1).Format("2006-01-02"), func(day string) { nights = append(nights, day) })
	for _, night := range nights {
		if available, known := s.availability[res.RoomID][night]; known && available < 1 {
			http.Error(w, `{"error":"sem disponibilidade em `+night+`"}`, http.StatusConflict)
			return
		}
	}
	for _, night := range nights {
		if _, known := s.availability[res.RoomID][night]; known {
			s.availability[res.RoomID][night]--
		}
	}
	if res.TotalCents == 0 {
		for _, night := range nights {
			res.TotalCents += s.rates[res.RoomID][night]
		}
	}
	s.nextID++
	res.ID, res.Pending = fmt.Sprint(s.nextID), "new"
	s.reservations = append(s.reservations, &res)
	s.logCall("reserva %s criada no canal: quarto %s, %s a %s", res.ID, res.RoomID, res.CheckIn, res.CheckOut)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}

func (s *simulator) cancelReservation(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, res := range s.reservations {
		if res.ID == r.PathValue("id") {
			res.Cancelled, res.Pending = true, "modify"
			s.logCall("reserva %s cancelada no canal", res.ID)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	http.NotFound(w, r)
}

func (s *simulator) createCalendarEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CheckIn  string `json:"check_in"`
		CheckOut string `json:"check_out"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CheckIn == "" || body.CheckOut == "" {
		http.Error(w, `{"error":"informe check_in e check_out"}`, http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	listing := strings.TrimSuffix(r.PathValue("listing"), ".ics")
	s.nextID++
	event := icalEvent{UID: fmt.Sprintf("airbnb-%d@simulador", s.nextID), CheckIn: body.CheckIn, CheckOut: body.CheckOut}
	s.calendars[listing] = append(s.calendars[listing], event)
	s.logCall("iCal: reserva no anúncio %s, %s a %s", listing, body.CheckIn, body.CheckOut)
	w.WriteHeader(http.StatusCreated)
}

// calendar publica o calendário de um anúncio no formato do Airbnb.
func (s *simulator) calendar(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	listing := strings.TrimSuffix(r.PathValue("listing"), ".ics")
	compact := func(date string) string { return strings.ReplaceAll(date, "-", "") }
	var body strings.Builder
	body.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Simulador//Airbnb//PT\r\n")
	for _, event := range s.calendars[listing] {
		fmt.Fprintf(&body, "BEGIN:VEVENT\r\nUID:%s\r\nDTSTART;VALUE=DATE:%s\r\nDTEND;VALUE=DATE:%s\r\nSUMMARY:Reserved\r\nEND:VEVENT\r\n",
			event.UID, compact(event.CheckIn), compact(event.CheckOut))
	}
	body.WriteString("END:VCALENDAR\r\n")
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	fmt.Fprint(w, body.String())
}

type roomState struct {
	RoomID string     `json:"room_id"`
	Days   []dayState `json:"days"`
}

type dayState struct {
	Date       string `json:"date"`
	Available  *int   `json:"available"`
	PriceCents *int64 `json:"price_cents"`
}

// snapshot monta, por quarto, os próximos dias com o que o canal conhece.
func (s *simulator) snapshot(days int) []roomState {
	rooms := map[string]bool{}
	for room := range s.availability {
		rooms[room] = true
	}
	for room := range s.rates {
		rooms[room] = true
	}
	names := make([]string, 0, len(rooms))
	for room := range rooms {
		names = append(names, room)
	}
	sort.Strings(names)

	today := time.Now()
	result := make([]roomState, 0, len(names))
	for _, room := range names {
		state := roomState{RoomID: room}
		for offset := 0; offset < days; offset++ {
			date := today.AddDate(0, 0, offset).Format("2006-01-02")
			day := dayState{Date: date}
			if available, ok := s.availability[room][date]; ok {
				day.Available = &available
			}
			if price, ok := s.rates[room][date]; ok {
				day.PriceCents = &price
			}
			state.Days = append(state.Days, day)
		}
		result = append(result, state)
	}
	return result
}

func (s *simulator) state(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"rooms": s.snapshot(30), "reservations": s.reservations, "calls": s.calls,
	})
}

var page = template.Must(template.New("page").Funcs(template.FuncMap{
	"day":   func(date string) string { return date[8:] + "/" + date[5:7] },
	"deref": func(value *int) int { return *value },
}).Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Simulador de OTA — InnKeep</title>
<meta http-equiv="refresh" content="5">
<style>
 body{font-family:system-ui,sans-serif;margin:24px;color:#1f2933;background:#f6f7f9}
 h1{font-size:20px;margin:0 0 4px} h2{font-size:15px;margin:24px 0 8px} p{color:#52606d;font-size:13px;margin:0 0 16px}
 table{border-collapse:collapse;background:#fff;font-size:12px} th,td{border:1px solid #e4e7eb;padding:4px 6px;text-align:center}
 th{background:#f0f2f5;font-weight:600} .zero{background:#fde2e2;color:#a61b1b;font-weight:700} .ok{background:#e3f9e5}
 pre{background:#fff;border:1px solid #e4e7eb;padding:12px;font-size:12px;max-height:260px;overflow:auto}
 .wrap{overflow-x:auto}
</style></head><body>
<h1>Simulador de OTA</h1>
<p>Faz o papel da Booking.com em desenvolvimento. A página se atualiza a cada 5 segundos.</p>
<h2>Disponibilidade recebida do InnKeep (próximos 21 dias)</h2>
{{if .Rooms}}<div class="wrap"><table><tr><th>Quarto</th>{{range (index .Rooms 0).Days}}<th>{{day .Date}}</th>{{end}}</tr>
{{range .Rooms}}<tr><th>{{.RoomID}}</th>{{range .Days}}<td class="{{if .Available}}{{if eq (deref .Available) 0}}zero{{else}}ok{{end}}{{end}}">{{if .Available}}{{deref .Available}}{{else}}–{{end}}</td>{{end}}</tr>{{end}}
</table></div>{{else}}<p>Nada recebido ainda. Conecte o canal no painel do InnKeep e mapeie um tipo de quarto.</p>{{end}}
<h2>Reservas feitas no canal</h2>
{{if .Reservations}}<table><tr><th>ID</th><th>Quarto</th><th>Entrada</th><th>Saída</th><th>Hóspede</th><th>Situação</th></tr>
{{range .Reservations}}<tr><td>{{.ID}}</td><td>{{.RoomID}}</td><td>{{.CheckIn}}</td><td>{{.CheckOut}}</td><td>{{.GuestName}}</td>
<td>{{if .Cancelled}}cancelada{{else}}ativa{{end}}{{if .Pending}} · aguardando InnKeep{{else}} · entregue{{end}}</td></tr>{{end}}</table>
{{else}}<p>Nenhuma. Crie uma com: <code>curl -X POST localhost:8090/sim/reservations -d '{"room_id":"...","check_in":"AAAA-MM-DD","check_out":"AAAA-MM-DD"}'</code></p>{{end}}
<h2>Chamadas recebidas</h2><pre>{{range .Calls}}{{.}}
{{end}}</pre>
</body></html>`))

func (s *simulator) dashboard(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Execute(w, map[string]any{"Rooms": s.snapshot(21), "Reservations": s.reservations, "Calls": s.calls})
}
