package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// RoomTypeKind distingue acomodações privativas de compartilhadas.
type RoomTypeKind string

const (
	// RoomTypePrivate: o quarto inteiro é vendido para uma reserva.
	RoomTypePrivate RoomTypeKind = "PRIVATE"
	// RoomTypeShared: dormitório de hostel. Cadastre cada CAMA como uma
	// unidade (Room) deste tipo — o sistema passa a vender camas.
	RoomTypeShared RoomTypeKind = "SHARED"
)

// RoomType é uma categoria de acomodação ("Suíte", "Quarto com beliches").
// É o tipo, e não o quarto físico, que é vendido nos canais e que tem preço.
type RoomType struct {
	ID               string
	PropertyID       string
	Name             string
	Description      string
	Kind             RoomTypeKind
	MaxOccupancy     int
	BedConfiguration string // ex.: "1 cama de casal + 1 de solteiro"
	// BasePriceCents é a diária padrão. O inventário permite sobrescrevê-la
	// por data (alta temporada, feriados).
	BasePriceCents int64
	Amenities      []string
	PhotoURLs      []string
	Active         bool
	SortOrder      int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// RoomTypeInput reúne os dados editáveis de um tipo de acomodação.
type RoomTypeInput struct {
	Name             string
	Description      string
	Kind             RoomTypeKind
	MaxOccupancy     int
	BedConfiguration string
	BasePriceCents   int64
	Amenities        []string
	PhotoURLs        []string
	Active           bool
	SortOrder        int
}

// NewRoomType cria um tipo de acomodação válido.
func NewRoomType(propertyID string, input RoomTypeInput) (*RoomType, error) {
	roomType := &RoomType{ID: kernel.NewID(), PropertyID: propertyID, CreatedAt: time.Now().UTC()}
	if err := roomType.Update(input); err != nil {
		return nil, err
	}
	return roomType, nil
}

// Update valida e aplica os dados.
func (t *RoomType) Update(input RoomTypeInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if n := utf8.RuneCountInString(input.Name); n < 2 || n > 80 {
		return kernel.Invalid("room_type.invalid_name", "informe o nome do tipo de acomodação").WithField("name", "entre 2 e 80 caracteres")
	}
	if input.Kind == "" {
		input.Kind = RoomTypePrivate
	}
	if input.Kind != RoomTypePrivate && input.Kind != RoomTypeShared {
		return kernel.Invalid("room_type.invalid_kind", "tipo inválido").WithField("kind", "use PRIVATE ou SHARED")
	}
	if input.MaxOccupancy < 1 || input.MaxOccupancy > 50 {
		return kernel.Invalid("room_type.invalid_occupancy", "capacidade inválida").WithField("max_occupancy", "entre 1 e 50")
	}
	if input.BasePriceCents <= 0 {
		return kernel.Invalid("room_type.invalid_price", "informe a diária padrão").WithField("base_price_cents", "deve ser maior que zero")
	}
	for _, url := range input.PhotoURLs {
		if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "/") {
			return kernel.Invalid("room_type.invalid_photo", "URL de foto inválida").WithField("photo_urls", "use URLs http(s)")
		}
	}

	t.Name = input.Name
	t.Description = strings.TrimSpace(input.Description)
	t.Kind = input.Kind
	t.MaxOccupancy = input.MaxOccupancy
	t.BedConfiguration = strings.TrimSpace(input.BedConfiguration)
	t.BasePriceCents = input.BasePriceCents
	t.Amenities = cleanList(input.Amenities)
	t.PhotoURLs = cleanList(input.PhotoURLs)
	t.Active = input.Active
	t.SortOrder = input.SortOrder
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// HousekeepingStatus é a situação de limpeza de um quarto.
type HousekeepingStatus string

const (
	HousekeepingClean HousekeepingStatus = "CLEAN"
	HousekeepingDirty HousekeepingStatus = "DIRTY"
)

// Room é uma unidade habitacional física: o quarto 101, a cama A do
// dormitório. É o que o hóspede efetivamente ocupa.
type Room struct {
	ID         string
	PropertyID string
	RoomTypeID string
	Name       string // número ou nome, ex.: "101", "Suíte Ipê"
	Floor      string
	Notes      string
	// Active=false tira o quarto de venda (reforma, desativação) sem apagar
	// o histórico de reservas associado a ele.
	Active       bool
	Housekeeping HousekeepingStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RoomInput reúne os dados editáveis de um quarto.
type RoomInput struct {
	RoomTypeID string
	Name       string
	Floor      string
	Notes      string
	Active     bool
}

// NewRoom cria um quarto válido (nasce limpo).
func NewRoom(propertyID string, input RoomInput) (*Room, error) {
	room := &Room{
		ID:           kernel.NewID(),
		PropertyID:   propertyID,
		Housekeeping: HousekeepingClean,
		CreatedAt:    time.Now().UTC(),
	}
	if err := room.Update(input); err != nil {
		return nil, err
	}
	return room, nil
}

// Update valida e aplica os dados.
func (r *Room) Update(input RoomInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if n := utf8.RuneCountInString(input.Name); n < 1 || n > 40 {
		return kernel.Invalid("room.invalid_name", "informe o nome ou número do quarto").WithField("name", "entre 1 e 40 caracteres")
	}
	if !kernel.IsID(input.RoomTypeID) {
		return kernel.Invalid("room.invalid_room_type", "informe o tipo de acomodação").WithField("room_type_id", "obrigatório")
	}
	r.RoomTypeID = input.RoomTypeID
	r.Name = input.Name
	r.Floor = strings.TrimSpace(input.Floor)
	r.Notes = strings.TrimSpace(input.Notes)
	r.Active = input.Active
	r.UpdatedAt = time.Now().UTC()
	return nil
}

// SetHousekeeping altera a situação de limpeza.
func (r *Room) SetHousekeeping(status HousekeepingStatus) error {
	if status != HousekeepingClean && status != HousekeepingDirty {
		return kernel.Invalid("room.invalid_housekeeping", "situação de limpeza inválida").WithField("status", "use CLEAN ou DIRTY")
	}
	r.Housekeeping = status
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func cleanList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}
