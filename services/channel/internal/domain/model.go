// Package domain contém o modelo do contexto de Canais de Venda (channel
// manager): as conexões com as agências online (OTAs) e o mapeamento entre os
// nossos tipos de quarto e os anúncios de cada canal.
package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// Channel é um canal de venda externo.
type Channel string

const (
	ChannelBookingCom Channel = "BOOKING_COM"
	ChannelAirbnb     Channel = "AIRBNB"
)

// Mode é a forma de integração com o canal.
type Mode string

const (
	// ModeAPI: integração direta pela API do canal. A disponibilidade é
	// enviada no instante em que muda. Exige credenciais de parceiro.
	ModeAPI Mode = "API"
	// ModeICal: troca de calendários no formato iCalendar. Funciona em
	// qualquer conta, sem parceria — mas o CANAL decide quando reler o nosso
	// calendário (de minutos a algumas horas), então não é instantâneo.
	ModeICal Mode = "ICAL"
)

// Erros de domínio do contexto.
var (
	ErrConnectionNotFound = kernel.NotFound("channel.connection_not_found", "conexão de canal não encontrada")
	ErrMappingNotFound    = kernel.NotFound("channel.mapping_not_found", "mapeamento não encontrado")
	ErrMappingExists      = kernel.Conflict("channel.mapping_exists", "este tipo de quarto já está mapeado nesta conexão")
	ErrUnsupported        = kernel.Rule("channel.unsupported_integration",
		"integração indisponível: a API do Airbnb é restrita a parceiros de software homologados; use a sincronização por iCal")
)

// Credentials são os dados de acesso à API do canal. Em memória ficam em
// texto puro; no banco, cifrados (veja o repositório).
type Credentials struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
}

// Empty informa se não há credenciais.
func (c Credentials) Empty() bool { return c.ClientID == "" && c.ClientSecret == "" }

// Connection é a ligação de uma propriedade com um canal.
type Connection struct {
	ID         string
	PropertyID string
	Channel    Channel
	Mode       Mode
	Name       string
	// ExternalPropertyID é o ID da propriedade no canal (Hotel ID no Booking).
	ExternalPropertyID string
	Credentials        Credentials
	// BaseURL substitui o endereço padrão da API do canal. Vazio = produção.
	// Em desenvolvimento aponta para o simulador de OTA.
	BaseURL    string
	Active     bool
	LastPushAt *time.Time
	LastPullAt *time.Time
	LastError  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ConnectionInput reúne os dados editáveis de uma conexão.
type ConnectionInput struct {
	Name               string
	ExternalPropertyID string
	// Credentials nulo = manter as atuais.
	Credentials *Credentials
	BaseURL     string
	Active      bool
}

// NewConnection cria uma conexão válida.
func NewConnection(propertyID string, channel Channel, mode Mode, input ConnectionInput) (*Connection, error) {
	if !kernel.IsID(propertyID) {
		return nil, kernel.Invalid("channel.invalid_property", "informe a propriedade").WithField("property_id", "obrigatório")
	}
	if channel != ChannelBookingCom && channel != ChannelAirbnb {
		return nil, kernel.Invalid("channel.invalid_channel", "canal inválido").WithField("channel", "use BOOKING_COM ou AIRBNB")
	}
	if mode != ModeAPI && mode != ModeICal {
		return nil, kernel.Invalid("channel.invalid_mode", "modo de integração inválido").WithField("mode", "use API ou ICAL")
	}
	if channel == ChannelAirbnb && mode == ModeAPI {
		return nil, ErrUnsupported
	}
	connection := &Connection{
		ID: kernel.NewID(), PropertyID: propertyID, Channel: channel, Mode: mode, CreatedAt: time.Now().UTC(),
	}
	if err := connection.Update(input); err != nil {
		return nil, err
	}
	return connection, nil
}

// Update valida e aplica os dados.
func (c *Connection) Update(input ConnectionInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = map[Channel]string{ChannelBookingCom: "Booking.com", ChannelAirbnb: "Airbnb"}[c.Channel]
	}
	if input.Credentials != nil {
		c.Credentials = *input.Credentials
	}
	if c.Mode == ModeAPI && input.Active && c.Credentials.Empty() {
		return kernel.Invalid("channel.credentials_required", "informe as credenciais da API do canal").
			WithField("client_id", "obrigatório")
	}
	if input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/"); input.BaseURL != "" {
		if err := ValidateURL(input.BaseURL); err != nil {
			return kernel.Invalid("channel.invalid_base_url", "endereço da API inválido").WithField("base_url", "use uma URL http(s)")
		}
	}
	c.Name, c.ExternalPropertyID = input.Name, strings.TrimSpace(input.ExternalPropertyID)
	c.BaseURL, c.Active = input.BaseURL, input.Active
	c.UpdatedAt = time.Now().UTC()
	return nil
}

// Mapping liga um tipo de quarto nosso a um anúncio do canal.
//
// O que se vende nos canais é o TIPO de quarto, não o quarto físico: "temos 2
// Quartos com Beliches disponíveis". Quem escolhe o quarto físico é o
// inventário, na hora de alocar. Se cada quarto é anunciado separadamente em
// um canal, cadastre um tipo de quarto para cada um.
type Mapping struct {
	ID           string
	ConnectionID string
	PropertyID   string
	RoomTypeID   string
	// ExternalRoomID: ID do quarto no canal (Booking) ou do anúncio (Airbnb).
	ExternalRoomID string
	// ExternalRateID: ID do plano tarifário no canal (necessário para preços).
	ExternalRateID string
	// ICalImportURL: endereço do calendário que o CANAL exporta (modo iCal).
	ICalImportURL string
	// ICalExportToken: segredo que compõe a URL do NOSSO calendário (modo iCal).
	ICalExportToken string
	CreatedAt       time.Time
}

// MappingInput reúne os dados de um mapeamento.
type MappingInput struct {
	RoomTypeID     string
	ExternalRoomID string
	ExternalRateID string
	ICalImportURL  string
}

// NewMapping cria um mapeamento válido para a conexão.
func NewMapping(connection *Connection, input MappingInput) (*Mapping, error) {
	if !kernel.IsID(input.RoomTypeID) {
		return nil, kernel.Invalid("channel.invalid_room_type", "informe o tipo de quarto").WithField("room_type_id", "obrigatório")
	}
	input.ExternalRoomID = strings.TrimSpace(input.ExternalRoomID)
	input.ICalImportURL = strings.TrimSpace(input.ICalImportURL)

	switch connection.Mode {
	case ModeAPI:
		if input.ExternalRoomID == "" {
			return nil, kernel.Invalid("channel.external_room_required", "informe o ID do quarto no canal").WithField("external_room_id", "obrigatório")
		}
	case ModeICal:
		if input.ICalImportURL != "" {
			if err := ValidateURL(input.ICalImportURL); err != nil {
				return nil, kernel.Invalid("channel.invalid_ical_url", "URL do calendário inválida").WithField("ical_import_url", "use uma URL http(s)")
			}
		}
	}

	token := make([]byte, 24)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("gerando token do calendário: %w", err)
	}
	return &Mapping{
		ID: kernel.NewID(), ConnectionID: connection.ID, PropertyID: connection.PropertyID,
		RoomTypeID: input.RoomTypeID, ExternalRoomID: input.ExternalRoomID,
		ExternalRateID: strings.TrimSpace(input.ExternalRateID), ICalImportURL: input.ICalImportURL,
		ICalExportToken: hex.EncodeToString(token), CreatedAt: time.Now().UTC(),
	}, nil
}

// ValidateURL aceita apenas URLs http/https bem formadas.
func ValidateURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("URL inválida")
	}
	return nil
}

// DayAvailability é a disponibilidade de um tipo de quarto em uma data.
type DayAvailability struct {
	Date      kernel.Date
	Available int
}

// DayRate é o preço e as restrições de um tipo de quarto em uma data.
type DayRate struct {
	Date       kernel.Date
	PriceCents int64
	MinStay    int
	Closed     bool
}

// Tipos de mensagem de reserva vindas de um canal.
const (
	ExternalNew       = "NEW"
	ExternalModified  = "MODIFIED"
	ExternalCancelled = "CANCELLED"
)

// ExternalReservation é uma reserva no formato NEUTRO do nosso domínio, já
// livre das particularidades de cada canal. Cada adaptador (Booking.com,
// iCal...) traduz o formato do parceiro para esta estrutura — é a Anti-
// Corruption Layer do DDD: o "dialeto" de fora não contamina o modelo de dentro.
type ExternalReservation struct {
	ExternalID     string
	Kind           string // ExternalNew | ExternalModified | ExternalCancelled
	ExternalRoomID string
	// MappingID identifica o mapeamento de origem quando o canal não informa
	// um ID de quarto (iCal: cada calendário pertence a um mapeamento).
	MappingID      string
	CheckIn        kernel.Date
	CheckOut       kernel.Date
	Adults         int
	Children       int
	HolderName     string
	Email          string
	Phone          string
	TotalCents     int64 // 0 = o canal não informou
	Currency       string
	PaymentCollect string
	Notes          string
	// SourceID é o identificador da mensagem no canal, usado para confirmar o
	// recebimento (pode diferir de ExternalID em reservas com vários quartos).
	SourceID string
}

// Fingerprint resume o conteúdo relevante da reserva. Se o canal reenviar a
// mesma reserva sem mudanças, a impressão digital é igual e nada é republicado.
func (r ExternalReservation) Fingerprint() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%d|%d|%d|%s",
		r.Kind, r.ExternalRoomID, r.CheckIn, r.CheckOut, r.Adults, r.Children, r.TotalCents, r.HolderName)))
	return hex.EncodeToString(sum[:12])
}

// PullResult é o resultado de uma consulta de reservas a um canal.
type PullResult struct {
	Reservations []ExternalReservation
	// Snapshot=true: a lista é o retrato COMPLETO do calendário do canal (é o
	// caso do iCal). O que sumiu em relação à leitura anterior foi cancelado.
	// Snapshot=false: a lista traz apenas novidades (APIs com fila de mensagens).
	Snapshot bool
}

// SyncLog é um registro de sincronização com um canal, para diagnóstico.
type SyncLog struct {
	ID           string
	ConnectionID string
	Direction    string // OUTBOUND (nós -> canal) | INBOUND (canal -> nós)
	Operation    string
	Success      bool
	Detail       string
	CreatedAt    time.Time
}
