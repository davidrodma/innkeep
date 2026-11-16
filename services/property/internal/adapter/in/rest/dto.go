package rest

import (
	"time"

	"github.com/davidrodma/innkeep/services/property/internal/domain"
)

// DTOs: o formato do JSON na fronteira HTTP. Ficam separados das entidades de
// domínio para que a API pública e o modelo interno evoluam sem amarrar um ao
// outro — e para que nenhum campo interno vaze por descuido.

type addressDTO struct {
	Line       string `json:"line"`
	District   string `json:"district"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

type propertyRequest struct {
	Slug               string     `json:"slug"`
	Name               string     `json:"name"`
	LegalName          string     `json:"legal_name"`
	TaxID              string     `json:"tax_id"`
	Cadastur           string     `json:"cadastur"`
	Kind               string     `json:"kind"`
	Description        string     `json:"description"`
	Email              string     `json:"email"`
	Phone              string     `json:"phone"`
	Address            addressDTO `json:"address"`
	Timezone           string     `json:"timezone"`
	Currency           string     `json:"currency"`
	CheckInTime        string     `json:"check_in_time"`
	CheckOutTime       string     `json:"check_out_time"`
	DepositPercent     int        `json:"deposit_percent"`
	HoldMinutes        int        `json:"hold_minutes"`
	CancellationPolicy string     `json:"cancellation_policy"`
	PixKey             string     `json:"pix_key"`
	Published          bool       `json:"published"`
}

func (r propertyRequest) toInput() domain.PropertyInput {
	return domain.PropertyInput{
		Slug: r.Slug, Name: r.Name, LegalName: r.LegalName, TaxID: r.TaxID, Cadastur: r.Cadastur,
		Kind: domain.Kind(r.Kind), Description: r.Description, Email: r.Email, Phone: r.Phone,
		Address: domain.Address{
			Line: r.Address.Line, District: r.Address.District, City: r.Address.City,
			State: r.Address.State, PostalCode: r.Address.PostalCode, Country: r.Address.Country,
		},
		Timezone: r.Timezone, Currency: r.Currency,
		CheckInTime: r.CheckInTime, CheckOutTime: r.CheckOutTime,
		DepositPercent: r.DepositPercent, HoldMinutes: r.HoldMinutes,
		CancellationPolicy: r.CancellationPolicy, PixKey: r.PixKey, Published: r.Published,
	}
}

type propertyResponse struct {
	ID string `json:"id"`
	propertyRequest
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toAddressDTO(a domain.Address) addressDTO {
	return addressDTO{Line: a.Line, District: a.District, City: a.City, State: a.State, PostalCode: a.PostalCode, Country: a.Country}
}

func toPropertyResponse(p *domain.Property) propertyResponse {
	return propertyResponse{
		ID: p.ID,
		propertyRequest: propertyRequest{
			Slug: p.Slug, Name: p.Name, LegalName: p.LegalName, TaxID: p.TaxID, Cadastur: p.Cadastur,
			Kind: string(p.Kind), Description: p.Description, Email: p.Email, Phone: p.Phone,
			Address: toAddressDTO(p.Address), Timezone: p.Timezone, Currency: p.Currency,
			CheckInTime: p.CheckInTime, CheckOutTime: p.CheckOutTime,
			DepositPercent: p.DepositPercent, HoldMinutes: p.HoldMinutes,
			CancellationPolicy: p.CancellationPolicy, PixKey: p.PixKey, Published: p.Published,
		},
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// publicPropertyResponse é o que o site público enxerga: nada de dados
// fiscais ou internos.
type publicPropertyResponse struct {
	ID                 string             `json:"id"`
	Slug               string             `json:"slug"`
	Name               string             `json:"name"`
	Kind               string             `json:"kind"`
	Description        string             `json:"description"`
	Email              string             `json:"email"`
	Phone              string             `json:"phone"`
	Address            addressDTO         `json:"address"`
	Timezone           string             `json:"timezone"`
	Currency           string             `json:"currency"`
	CheckInTime        string             `json:"check_in_time"`
	CheckOutTime       string             `json:"check_out_time"`
	DepositPercent     int                `json:"deposit_percent"`
	HoldMinutes        int                `json:"hold_minutes"`
	CancellationPolicy string             `json:"cancellation_policy"`
	PixKey             string             `json:"pix_key"`
	RoomTypes          []roomTypeResponse `json:"room_types,omitempty"`
}

func toPublicProperty(p *domain.Property, roomTypes []*domain.RoomType) publicPropertyResponse {
	response := publicPropertyResponse{
		ID: p.ID, Slug: p.Slug, Name: p.Name, Kind: string(p.Kind), Description: p.Description,
		Email: p.Email, Phone: p.Phone, Address: toAddressDTO(p.Address),
		Timezone: p.Timezone, Currency: p.Currency,
		CheckInTime: p.CheckInTime, CheckOutTime: p.CheckOutTime,
		DepositPercent: p.DepositPercent, HoldMinutes: p.HoldMinutes,
		CancellationPolicy: p.CancellationPolicy, PixKey: p.PixKey,
	}
	for _, roomType := range roomTypes {
		response.RoomTypes = append(response.RoomTypes, toRoomTypeResponse(roomType))
	}
	return response
}

type roomTypeRequest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Kind             string   `json:"kind"`
	MaxOccupancy     int      `json:"max_occupancy"`
	BedConfiguration string   `json:"bed_configuration"`
	BasePriceCents   int64    `json:"base_price_cents"`
	Amenities        []string `json:"amenities"`
	PhotoURLs        []string `json:"photo_urls"`
	Active           *bool    `json:"active"`
	SortOrder        int      `json:"sort_order"`
}

func (r roomTypeRequest) toInput() domain.RoomTypeInput {
	return domain.RoomTypeInput{
		Name: r.Name, Description: r.Description, Kind: domain.RoomTypeKind(r.Kind),
		MaxOccupancy: r.MaxOccupancy, BedConfiguration: r.BedConfiguration, BasePriceCents: r.BasePriceCents,
		Amenities: r.Amenities, PhotoURLs: r.PhotoURLs, Active: r.Active == nil || *r.Active, SortOrder: r.SortOrder,
	}
}

type roomTypeResponse struct {
	ID               string    `json:"id"`
	PropertyID       string    `json:"property_id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	Kind             string    `json:"kind"`
	MaxOccupancy     int       `json:"max_occupancy"`
	BedConfiguration string    `json:"bed_configuration"`
	BasePriceCents   int64     `json:"base_price_cents"`
	Amenities        []string  `json:"amenities"`
	PhotoURLs        []string  `json:"photo_urls"`
	Active           bool      `json:"active"`
	SortOrder        int       `json:"sort_order"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func toRoomTypeResponse(t *domain.RoomType) roomTypeResponse {
	return roomTypeResponse{
		ID: t.ID, PropertyID: t.PropertyID, Name: t.Name, Description: t.Description, Kind: string(t.Kind),
		MaxOccupancy: t.MaxOccupancy, BedConfiguration: t.BedConfiguration, BasePriceCents: t.BasePriceCents,
		Amenities: orEmpty(t.Amenities), PhotoURLs: orEmpty(t.PhotoURLs), Active: t.Active,
		SortOrder: t.SortOrder, UpdatedAt: t.UpdatedAt,
	}
}

type roomRequest struct {
	RoomTypeID string `json:"room_type_id"`
	Name       string `json:"name"`
	Floor      string `json:"floor"`
	Notes      string `json:"notes"`
	Active     *bool  `json:"active"`
}

func (r roomRequest) toInput() domain.RoomInput {
	return domain.RoomInput{
		RoomTypeID: r.RoomTypeID, Name: r.Name, Floor: r.Floor, Notes: r.Notes, Active: r.Active == nil || *r.Active,
	}
}

type roomResponse struct {
	ID           string    `json:"id"`
	PropertyID   string    `json:"property_id"`
	RoomTypeID   string    `json:"room_type_id"`
	Name         string    `json:"name"`
	Floor        string    `json:"floor"`
	Notes        string    `json:"notes"`
	Active       bool      `json:"active"`
	Housekeeping string    `json:"housekeeping"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toRoomResponse(r *domain.Room) roomResponse {
	return roomResponse{
		ID: r.ID, PropertyID: r.PropertyID, RoomTypeID: r.RoomTypeID, Name: r.Name, Floor: r.Floor,
		Notes: r.Notes, Active: r.Active, Housekeeping: string(r.Housekeeping), UpdatedAt: r.UpdatedAt,
	}
}

func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
