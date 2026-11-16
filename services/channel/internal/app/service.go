package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/services/channel/internal/domain"
)

// horizonDays é até onde, no futuro, a disponibilidade é sincronizada.
const horizonDays = 365

// Service reúne os casos de uso do channel manager.
type Service struct {
	tx         TxRunner
	repository Repository
	connectors ConnectorRegistry
	publisher  EventPublisher
	logger     *slog.Logger
	// Intervalos mínimos entre buscas de reservas, por modo de integração.
	apiInterval  time.Duration
	icalInterval time.Duration
}

// NewService monta o serviço.
func NewService(tx TxRunner, repository Repository, connectors ConnectorRegistry, publisher EventPublisher,
	logger *slog.Logger, apiInterval, icalInterval time.Duration) *Service {
	return &Service{tx: tx, repository: repository, connectors: connectors, publisher: publisher,
		logger: logger, apiInterval: apiInterval, icalInterval: icalInterval}
}

// ---------------------------------------------------------------------------
// Conexões e mapeamentos
// ---------------------------------------------------------------------------

// CreateConnection cadastra a ligação de uma propriedade com um canal.
func (s *Service) CreateConnection(ctx context.Context, propertyID string, channel domain.Channel, mode domain.Mode, input domain.ConnectionInput) (*domain.Connection, error) {
	connection, err := domain.NewConnection(propertyID, channel, mode, input)
	if err != nil {
		return nil, err
	}
	if _, ok := s.connectors.For(channel, mode); !ok {
		return nil, domain.ErrUnsupported
	}
	return connection, s.repository.CreateConnection(ctx, connection)
}

// UpdateConnection altera uma conexão.
func (s *Service) UpdateConnection(ctx context.Context, id string, input domain.ConnectionInput) (*domain.Connection, error) {
	connection, err := s.repository.Connection(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := connection.Update(input); err != nil {
		return nil, err
	}
	return connection, s.repository.UpdateConnection(ctx, connection)
}

// DeleteConnection remove uma conexão e seus mapeamentos.
func (s *Service) DeleteConnection(ctx context.Context, id string) error {
	return s.repository.DeleteConnection(ctx, id)
}

// Connection devolve uma conexão.
func (s *Service) Connection(ctx context.Context, id string) (*domain.Connection, error) {
	return s.repository.Connection(ctx, id)
}

// Connections lista as conexões de uma propriedade.
func (s *Service) Connections(ctx context.Context, propertyID string) ([]*domain.Connection, error) {
	return s.repository.Connections(ctx, propertyID)
}

// Mappings lista os mapeamentos de uma conexão.
func (s *Service) Mappings(ctx context.Context, connectionID string) ([]domain.Mapping, error) {
	return s.repository.MappingsByConnection(ctx, connectionID)
}

// Mapping devolve um mapeamento.
func (s *Service) Mapping(ctx context.Context, id string) (*domain.Mapping, error) {
	return s.repository.Mapping(ctx, id)
}

// CreateMapping liga um tipo de quarto a um anúncio do canal e pede ao
// inventário o reenvio completo da disponibilidade daquele tipo — um canal
// recém-mapeado nasce sem saber de nada.
func (s *Service) CreateMapping(ctx context.Context, connectionID string, input domain.MappingInput) (*domain.Mapping, error) {
	var mapping *domain.Mapping
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		connection, err := s.repository.Connection(ctx, connectionID)
		if err != nil {
			return err
		}
		if mapping, err = domain.NewMapping(connection, input); err != nil {
			return err
		}
		if err := s.repository.CreateMapping(ctx, mapping); err != nil {
			return err
		}
		return s.publisher.Publish(ctx, syncRequest(connection.ID, *mapping))
	})
	return mapping, err
}

// DeleteMapping remove um mapeamento.
func (s *Service) DeleteMapping(ctx context.Context, id string) error {
	return s.repository.DeleteMapping(ctx, id)
}

// RequestSync pede o reenvio completo de disponibilidade e tarifas de todos
// os tipos de quarto mapeados na conexão (botão "sincronizar tudo").
func (s *Service) RequestSync(ctx context.Context, connectionID string) (int, error) {
	mappings, err := s.repository.MappingsByConnection(ctx, connectionID)
	if err != nil {
		return 0, err
	}
	events := make([]contracts.Event, 0, len(mappings))
	for _, mapping := range mappings {
		events = append(events, syncRequest(connectionID, mapping))
	}
	return len(mappings), s.tx.WithinTx(ctx, func(ctx context.Context) error {
		return s.publisher.Publish(ctx, events...)
	})
}

func syncRequest(connectionID string, mapping domain.Mapping) contracts.ChannelSyncRequested {
	today := kernel.Today(time.UTC)
	return contracts.ChannelSyncRequested{
		ConnectionID: connectionID, PropertyID: mapping.PropertyID, RoomTypeID: mapping.RoomTypeID,
		From: today.String(), To: today.AddDays(horizonDays).String(),
	}
}

// SyncLogs devolve os registros recentes de sincronização de uma conexão.
func (s *Service) SyncLogs(ctx context.Context, connectionID string, limit int) ([]domain.SyncLog, error) {
	return s.repository.SyncLogs(ctx, connectionID, limit)
}

// ---------------------------------------------------------------------------
// Saída: nossa disponibilidade e nossos preços -> canais
// ---------------------------------------------------------------------------

// OnAvailabilityChanged é a reação ao evento central do sistema: a
// disponibilidade de um tipo de quarto mudou.
//
//  1. Atualiza o retrato local (usado para montar os calendários iCal).
//  2. Envia a mudança a cada canal de API em que o tipo está mapeado.
//
// O envio é IDEMPOTENTE por natureza: manda-se o valor absoluto ("2 quartos
// no dia 10"), não um incremento. Receber o mesmo evento duas vezes apenas
// reenvia o mesmo número — por isso este handler dispensa a tabela inbox.
// Se algum canal falhar, o erro é devolvido e o consumidor tenta de novo.
func (s *Service) OnAvailabilityChanged(ctx context.Context, event contracts.AvailabilityChanged) error {
	days := make([]domain.DayAvailability, 0, len(event.Days))
	for _, day := range event.Days {
		if date, err := kernel.ParseDate(day.Date); err == nil {
			days = append(days, domain.DayAvailability{Date: date, Available: day.Available})
		}
	}
	if err := s.repository.SaveAvailability(ctx, event.RoomTypeID, days); err != nil {
		return err
	}
	return s.push(ctx, event.RoomTypeID, "PUSH_AVAILABILITY", fmt.Sprintf("%d dia(s)", len(days)),
		func(connector Connector, connection domain.Connection, mapping domain.Mapping) error {
			return connector.PushAvailability(ctx, connection, mapping, days)
		})
}

// OnRateChanged envia aos canais a mudança de preços e restrições.
func (s *Service) OnRateChanged(ctx context.Context, event contracts.RateChanged) error {
	days := make([]domain.DayRate, 0, len(event.Days))
	for _, day := range event.Days {
		if date, err := kernel.ParseDate(day.Date); err == nil {
			days = append(days, domain.DayRate{Date: date, PriceCents: day.PriceCents, MinStay: day.MinStay, Closed: day.Closed})
		}
	}
	return s.push(ctx, event.RoomTypeID, "PUSH_RATES", fmt.Sprintf("%d dia(s)", len(days)),
		func(connector Connector, connection domain.Connection, mapping domain.Mapping) error {
			return connector.PushRates(ctx, connection, mapping, event.Currency, days)
		})
}

func (s *Service) push(ctx context.Context, roomTypeID, operation, summary string,
	send func(connector Connector, connection domain.Connection, mapping domain.Mapping) error) error {
	mappings, err := s.repository.MappingsByRoomType(ctx, roomTypeID)
	if err != nil {
		return err
	}
	var failures []error
	for _, mapping := range mappings {
		connection, err := s.repository.Connection(ctx, mapping.ConnectionID)
		if err != nil || !connection.Active || connection.Mode != domain.ModeAPI {
			continue // iCal não recebe envio: é o canal que vem buscar o nosso calendário
		}
		connector, ok := s.connectors.For(connection.Channel, connection.Mode)
		if !ok {
			continue
		}
		pushErr := send(connector, *connection, mapping)
		s.record(ctx, connection.ID, "OUTBOUND", operation, summary, pushErr)
		if err := s.repository.MarkPush(ctx, connection.ID, time.Now().UTC(), errorText(pushErr)); err != nil {
			return err
		}
		if pushErr != nil {
			failures = append(failures, fmt.Errorf("%s: %w", connection.Name, pushErr))
		}
	}
	return errors.Join(failures...)
}

// ---------------------------------------------------------------------------
// Entrada: reservas feitas nos canais -> nosso sistema
// ---------------------------------------------------------------------------

// PullDue busca reservas nas conexões cujo intervalo de consulta venceu.
// Roda periodicamente; uma trava no banco garante que só uma réplica do
// serviço execute cada rodada.
func (s *Service) PullDue(ctx context.Context) error {
	connections, err := s.repository.ActiveConnections(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, connection := range connections {
		interval := s.apiInterval
		if connection.Mode == domain.ModeICal {
			interval = s.icalInterval
		}
		if connection.LastPullAt != nil && now.Sub(*connection.LastPullAt) < interval {
			continue
		}
		if _, err := s.Pull(ctx, connection.ID); err != nil {
			s.logger.WarnContext(ctx, "falha ao buscar reservas no canal",
				slog.String("connection_id", connection.ID), slog.Any("erro", err))
		}
	}
	return nil
}

// Pull busca agora as reservas de uma conexão e publica as novidades.
//
// Ordem das operações, pensada para nunca perder nem duplicar uma reserva:
//
//  1. busca as reservas no canal;
//  2. em UMA transação: registra o que já foi visto e grava os eventos na
//     outbox;
//  3. só depois do commit confirma o recebimento ao canal.
//
// Se o passo 3 falhar, o canal reenvia as mesmas reservas na próxima rodada;
// a "impressão digital" guardada no passo 2 mostra que nada mudou e nada é
// publicado de novo — apenas confirmamos outra vez.
func (s *Service) Pull(ctx context.Context, connectionID string) (int, error) {
	connection, err := s.repository.Connection(ctx, connectionID)
	if err != nil {
		return 0, err
	}
	connector, ok := s.connectors.For(connection.Channel, connection.Mode)
	if !ok {
		return 0, domain.ErrUnsupported
	}
	mappings, err := s.repository.MappingsByConnection(ctx, connectionID)
	if err != nil {
		return 0, err
	}

	var (
		published int
		toAck     []domain.ExternalReservation
		locked    = true
	)
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if locked, err = s.repository.TryPullLock(ctx); err != nil || !locked {
			return err
		}
		result, err := connector.Pull(ctx, *connection, mappings)
		if err != nil {
			return err
		}
		known, err := s.repository.KnownReservations(ctx, connectionID)
		if err != nil {
			return err
		}
		events, changed := s.reconcile(*connection, mappings, known, result)
		for _, entry := range changed {
			if err := s.repository.SaveKnownReservation(ctx, connectionID, entry); err != nil {
				return err
			}
		}
		published, toAck = len(events), result.Reservations
		return s.publisher.Publish(ctx, events...)
	})
	if !locked {
		return 0, nil // outra réplica está cuidando desta rodada
	}

	if err == nil && len(toAck) > 0 {
		err = connector.Acknowledge(ctx, *connection, toAck)
	}
	// Só registra quando houve novidade ou falha: a busca roda a cada poucos
	// segundos e encheria o histórico com linhas "nada de novo".
	if published > 0 || err != nil {
		s.record(ctx, connectionID, "INBOUND", "PULL_RESERVATIONS", fmt.Sprintf("%d novidade(s)", published), err)
	}
	if markErr := s.repository.MarkPull(ctx, connectionID, time.Now().UTC(), errorText(err)); markErr != nil {
		return published, markErr
	}
	return published, err
}

// reconcile compara o que o canal informou com o que já conhecemos e decide
// quais eventos publicar.
func (s *Service) reconcile(connection domain.Connection, mappings []domain.Mapping, known []KnownReservation,
	result domain.PullResult) ([]contracts.Event, []KnownReservation) {
	knownByID := make(map[string]KnownReservation, len(known))
	for _, entry := range known {
		knownByID[entry.ExternalID] = entry
	}
	findMapping := func(r domain.ExternalReservation) *domain.Mapping {
		for i := range mappings {
			if (r.MappingID != "" && mappings[i].ID == r.MappingID) ||
				(r.MappingID == "" && r.ExternalRoomID != "" && mappings[i].ExternalRoomID == r.ExternalRoomID) {
				return &mappings[i]
			}
		}
		return nil
	}

	var (
		events  []contracts.Event
		changed []KnownReservation
		seen    = make(map[string]bool, len(result.Reservations))
	)
	for _, reservation := range result.Reservations {
		seen[reservation.ExternalID] = true
		previous, exists := knownByID[reservation.ExternalID]
		fingerprint := reservation.Fingerprint()
		if exists && previous.Fingerprint == fingerprint {
			continue // nada mudou desde a última leitura
		}

		if reservation.Kind == domain.ExternalCancelled {
			if exists && !previous.Cancelled {
				events = append(events, contracts.ChannelReservationCancelled{
					ConnectionID: connection.ID, Channel: string(connection.Channel),
					PropertyID: connection.PropertyID, ExternalID: reservation.ExternalID, Reason: "cancelada no canal",
				})
			}
			changed = append(changed, KnownReservation{ExternalID: reservation.ExternalID, MappingID: previous.MappingID,
				Fingerprint: fingerprint, Cancelled: true, CheckOut: reservation.CheckOut})
			continue
		}

		mapping := findMapping(reservation)
		if mapping == nil {
			s.logger.Warn("reserva de canal ignorada: quarto sem mapeamento",
				slog.String("connection_id", connection.ID), slog.String("external_room_id", reservation.ExternalRoomID))
			continue
		}
		payload := contracts.ChannelReservation{
			ConnectionID: connection.ID, Channel: string(connection.Channel), PropertyID: connection.PropertyID,
			ExternalID: reservation.ExternalID, RoomTypeID: mapping.RoomTypeID,
			CheckIn: reservation.CheckIn.String(), CheckOut: reservation.CheckOut.String(),
			Adults: max(reservation.Adults, 1), Children: reservation.Children,
			Holder:     contracts.Holder{Name: holderName(reservation, connection), Email: reservation.Email, Phone: reservation.Phone},
			TotalCents: reservation.TotalCents, Currency: reservation.Currency,
			PaymentCollect: orDefault(reservation.PaymentCollect, contracts.CollectChannel), Notes: reservation.Notes,
		}
		if exists && !previous.Cancelled {
			events = append(events, contracts.ChannelReservationModified{ChannelReservation: payload})
		} else {
			events = append(events, contracts.ChannelReservationReceived{ChannelReservation: payload})
		}
		changed = append(changed, KnownReservation{ExternalID: reservation.ExternalID, MappingID: mapping.ID,
			Fingerprint: fingerprint, CheckOut: reservation.CheckOut})
	}

	// Em um retrato completo (iCal), reserva futura que sumiu foi cancelada.
	// Reservas passadas somem do calendário naturalmente e não contam.
	if result.Snapshot {
		today := kernel.Today(time.UTC)
		for _, entry := range known {
			if seen[entry.ExternalID] || entry.Cancelled || !entry.CheckOut.After(today) {
				continue
			}
			events = append(events, contracts.ChannelReservationCancelled{
				ConnectionID: connection.ID, Channel: string(connection.Channel),
				PropertyID: connection.PropertyID, ExternalID: entry.ExternalID, Reason: "removida do calendário do canal",
			})
			entry.Cancelled = true
			changed = append(changed, entry)
		}
	}
	return events, changed
}

// ---------------------------------------------------------------------------
// Calendário iCal exportado (o canal vem buscar)
// ---------------------------------------------------------------------------

// BusyRange é um intervalo [From, To) em que o tipo de quarto está esgotado.
type BusyRange struct {
	From kernel.Date
	To   kernel.Date
}

// CalendarFeed devolve os períodos esgotados do tipo de quarto ligado ao
// token — o conteúdo do calendário que Airbnb/Booking importam.
func (s *Service) CalendarFeed(ctx context.Context, token string) (*domain.Mapping, []BusyRange, error) {
	mapping, err := s.repository.MappingByToken(ctx, strings.TrimSuffix(token, ".ics"))
	if err != nil {
		return nil, nil, err
	}
	today := kernel.Today(time.UTC)
	soldOut, err := s.repository.SoldOutDays(ctx, mapping.RoomTypeID, today, today.AddDays(horizonDays))
	if err != nil {
		return nil, nil, err
	}
	// Junta dias consecutivos em intervalos: 10, 11, 12 => [10, 13).
	var ranges []BusyRange
	for _, day := range soldOut {
		if n := len(ranges); n > 0 && ranges[n-1].To.Equal(day) {
			ranges[n-1].To = day.AddDays(1)
			continue
		}
		ranges = append(ranges, BusyRange{From: day, To: day.AddDays(1)})
	}
	return mapping, ranges, nil
}

// ---------------------------------------------------------------------------

func (s *Service) record(ctx context.Context, connectionID, direction, operation, summary string, err error) {
	detail := summary
	if err != nil {
		detail = summary + " — " + err.Error()
	}
	if len(detail) > 500 {
		detail = detail[:500]
	}
	logErr := s.repository.AddSyncLog(context.WithoutCancel(ctx), domain.SyncLog{
		ID: kernel.NewID(), ConnectionID: connectionID, Direction: direction, Operation: operation,
		Success: err == nil, Detail: detail, CreatedAt: time.Now().UTC(),
	})
	if logErr != nil {
		s.logger.WarnContext(ctx, "falha ao gravar o log de sincronização", slog.Any("erro", logErr))
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	if text := err.Error(); len(text) > 300 {
		return text[:300]
	} else {
		return text
	}
}

func holderName(reservation domain.ExternalReservation, connection domain.Connection) string {
	if name := strings.TrimSpace(reservation.HolderName); len(name) >= 2 {
		return name
	}
	return "Hóspede " + connection.Name
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
