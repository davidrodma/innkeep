package contracts

// Eventos do contexto de Propriedades (catálogo). Chave de partição: o ID da
// propriedade — assim "tipo de quarto definido" chega sempre antes de "quarto
// adicionado" daquele tipo.
//
// Estes eventos carregam o ESTADO COMPLETO da entidade (padrão "Event-Carried
// State Transfer"). O consumidor não precisa chamar o serviço de origem para
// saber os detalhes: ele mantém uma cópia local somente-leitura e continua
// funcionando mesmo que o serviço de propriedades esteja fora do ar.
const (
	TypePropertyRegistered = "innkeep.property.registered.v1"
	TypePropertyUpdated    = "innkeep.property.updated.v1"
	TypeRoomTypeDefined    = "innkeep.property.room_type_defined.v1"
	TypeRoomTypeChanged    = "innkeep.property.room_type_changed.v1"
	TypeRoomAdded          = "innkeep.property.room_added.v1"
	TypeRoomChanged        = "innkeep.property.room_changed.v1"
	TypeRoomRemoved        = "innkeep.property.room_removed.v1"
)

// PropertySnapshot é a fotografia de uma propriedade no momento do evento.
type PropertySnapshot struct {
	PropertyID     string `json:"property_id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Timezone       string `json:"timezone"`
	Currency       string `json:"currency"`
	CheckInTime    string `json:"check_in_time"`   // "14:00"
	CheckOutTime   string `json:"check_out_time"`  // "12:00"
	DepositPercent int    `json:"deposit_percent"` // % do total exigido como sinal (0 = sem sinal)
	HoldMinutes    int    `json:"hold_minutes"`    // prazo para pagar o sinal antes da reserva expirar
	Email          string `json:"email,omitempty"`
	Phone          string `json:"phone,omitempty"`
	AddressLine    string `json:"address_line,omitempty"`
	City           string `json:"city,omitempty"`
	State          string `json:"state,omitempty"`
	PixKey         string `json:"pix_key,omitempty"`
	Published      bool   `json:"published"`
}

// PropertyRegistered: uma nova propriedade foi cadastrada.
type PropertyRegistered struct{ PropertySnapshot }

func (PropertyRegistered) EventType() string      { return TypePropertyRegistered }
func (e PropertyRegistered) PartitionKey() string { return e.PropertyID }

// PropertyUpdated: dados ou políticas da propriedade mudaram.
type PropertyUpdated struct{ PropertySnapshot }

func (PropertyUpdated) EventType() string      { return TypePropertyUpdated }
func (e PropertyUpdated) PartitionKey() string { return e.PropertyID }

// RoomTypeSnapshot é a fotografia de um tipo de acomodação.
type RoomTypeSnapshot struct {
	RoomTypeID     string `json:"room_type_id"`
	PropertyID     string `json:"property_id"`
	Name           string `json:"name"`
	MaxOccupancy   int    `json:"max_occupancy"`
	BasePriceCents int64  `json:"base_price_cents"` // diária padrão, em centavos
	Currency       string `json:"currency"`
	Active         bool   `json:"active"`
}

// RoomTypeDefined: um tipo de acomodação foi criado.
type RoomTypeDefined struct{ RoomTypeSnapshot }

func (RoomTypeDefined) EventType() string      { return TypeRoomTypeDefined }
func (e RoomTypeDefined) PartitionKey() string { return e.PropertyID }

// RoomTypeChanged: um tipo de acomodação foi alterado.
type RoomTypeChanged struct{ RoomTypeSnapshot }

func (RoomTypeChanged) EventType() string      { return TypeRoomTypeChanged }
func (e RoomTypeChanged) PartitionKey() string { return e.PropertyID }

// RoomSnapshot é a fotografia de um quarto (unidade habitacional).
type RoomSnapshot struct {
	RoomID     string `json:"room_id"`
	PropertyID string `json:"property_id"`
	RoomTypeID string `json:"room_type_id"`
	Name       string `json:"name"`
	Active     bool   `json:"active"` // false = fora de venda
}

// RoomAdded: um quarto foi adicionado ao inventário físico.
type RoomAdded struct{ RoomSnapshot }

func (RoomAdded) EventType() string      { return TypeRoomAdded }
func (e RoomAdded) PartitionKey() string { return e.PropertyID }

// RoomChanged: os dados de um quarto mudaram.
type RoomChanged struct{ RoomSnapshot }

func (RoomChanged) EventType() string      { return TypeRoomChanged }
func (e RoomChanged) PartitionKey() string { return e.PropertyID }

// RoomRemoved: um quarto deixou de existir.
type RoomRemoved struct {
	RoomID     string `json:"room_id"`
	PropertyID string `json:"property_id"`
	RoomTypeID string `json:"room_type_id"`
}

func (RoomRemoved) EventType() string      { return TypeRoomRemoved }
func (e RoomRemoved) PartitionKey() string { return e.PropertyID }
