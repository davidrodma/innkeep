// Package consumer é o adaptador de entrada de EVENTOS do serviço de
// propriedades: reage a fatos publicados por outros serviços no Kafka.
package consumer

import (
	"context"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/services/property/internal/app"
)

// Register associa os eventos de interesse aos casos de uso.
func Register(router *messaging.Router, service *app.Service) {
	// Check-out realizado => o quarto precisa de limpeza. O serviço de
	// reservas não sabe (nem precisa saber) que governança existe: ele só
	// anuncia o fato, e quem tiver interesse reage. Esse é o desacoplamento
	// que a arquitetura orientada a eventos oferece.
	router.On(contracts.TypeReservationCheckedOut, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, event contracts.ReservationCheckedOut) error {
			return service.MarkRoomDirty(ctx, event.RoomID)
		}))
}
