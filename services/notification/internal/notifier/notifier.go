// Package notifier transforma eventos de reserva em e-mails para o hóspede.
package notifier

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/davidrodma/innkeep/pkg/contracts"
	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/platform/messaging"
	"github.com/davidrodma/innkeep/services/notification/internal/mailer"
)

// Service envia as notificações.
type Service struct {
	sender    mailer.Sender
	redis     *redis.Client
	publicURL string
	logger    *slog.Logger
	html      *template.Template
	text      *texttemplate.Template
}

// New monta o serviço e compila os modelos de e-mail.
func New(sender mailer.Sender, redisClient *redis.Client, publicURL string, logger *slog.Logger) (*Service, error) {
	// html/template escapa automaticamente os valores inseridos no HTML: um
	// nome de hóspede como "<script>..." vira texto inofensivo.
	html, err := template.New("email").Parse(htmlLayout)
	if err != nil {
		return nil, fmt.Errorf("notifier: modelo HTML inválido: %w", err)
	}
	text, err := texttemplate.New("email").Parse(textLayout)
	if err != nil {
		return nil, fmt.Errorf("notifier: modelo de texto inválido: %w", err)
	}
	return &Service{sender: sender, redis: redisClient, publicURL: strings.TrimRight(publicURL, "/"),
		logger: logger, html: html, text: text}, nil
}

// Register associa os eventos aos e-mails.
func (s *Service) Register(router *messaging.Router) {
	// Guarda nome e contato das propriedades para personalizar as mensagens.
	rememberProperty := func(ctx context.Context, p contracts.PropertySnapshot) error {
		return s.redis.HSet(ctx, propertyKey(p.PropertyID),
			"name", p.Name, "phone", p.Phone, "email", p.Email, "pix_key", p.PixKey,
			"check_in_time", p.CheckInTime, "check_out_time", p.CheckOutTime,
			"address", strings.Trim(p.AddressLine+" — "+p.City+"/"+p.State, " —/")).Err()
	}
	router.On(contracts.TypePropertyRegistered, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.PropertyRegistered) error {
			return rememberProperty(ctx, e.PropertySnapshot)
		}))
	router.On(contracts.TypePropertyUpdated, messaging.Typed(
		func(ctx context.Context, _ contracts.Envelope, e contracts.PropertyUpdated) error {
			return rememberProperty(ctx, e.PropertySnapshot)
		}))

	router.On(contracts.TypeReservationAccepted, messaging.Typed(
		func(ctx context.Context, env contracts.Envelope, e contracts.ReservationAccepted) error {
			if e.Confirmed {
				return nil // o e-mail de confirmação sai pelo evento "confirmed"
			}
			return s.notify(ctx, env, e.ReservationSummary, email{
				Subject: "Recebemos sua reserva — falta o sinal",
				Heading: "Sua reserva está quase garantida",
				Intro:   "Reservamos o quarto para você. Para confirmar, pague o sinal dentro do prazo abaixo.",
				Deposit: e.DepositDueCents, HoldExpiresAt: e.HoldExpiresAt, ShowPayment: true,
			})
		}))
	router.On(contracts.TypeReservationConfirmed, messaging.Typed(
		func(ctx context.Context, env contracts.Envelope, e contracts.ReservationConfirmed) error {
			return s.notify(ctx, env, e.ReservationSummary, email{
				Subject: "Reserva confirmada",
				Heading: "Sua reserva está confirmada!",
				Intro:   "Está tudo certo. Esperamos você na data combinada.",
			})
		}))
	router.On(contracts.TypeReservationRejected, messaging.Typed(
		func(ctx context.Context, env contracts.Envelope, e contracts.ReservationRejected) error {
			return s.notify(ctx, env, e.ReservationSummary, email{
				Subject: "Não foi possível concluir sua reserva",
				Heading: "Não conseguimos reservar para estas datas",
				Intro:   "O quarto escolhido deixou de estar disponível no período. Tente outras datas ou outro tipo de acomodação.",
			})
		}))
	router.On(contracts.TypeReservationCancelled, messaging.Typed(
		func(ctx context.Context, env contracts.Envelope, e contracts.ReservationCancelled) error {
			return s.notify(ctx, env, e.ReservationSummary, email{
				Subject: "Reserva cancelada",
				Heading: "Sua reserva foi cancelada",
				Intro:   "Registramos o cancelamento da reserva abaixo. Se não foi você quem pediu, fale conosco.",
			})
		}))
	router.On(contracts.TypeReservationExpired, messaging.Typed(
		func(ctx context.Context, env contracts.Envelope, e contracts.ReservationExpired) error {
			return s.notify(ctx, env, e.ReservationSummary, email{
				Subject: "Sua reserva expirou",
				Heading: "O prazo do sinal terminou",
				Intro:   "Como o sinal não foi pago dentro do prazo, a reserva expirou e o quarto voltou a ficar disponível. Você pode fazer uma nova reserva quando quiser.",
			})
		}))
}

// email é o conteúdo variável de uma notificação.
type email struct {
	Subject       string
	Heading       string
	Intro         string
	Deposit       int64
	HoldExpiresAt *time.Time
	ShowPayment   bool
}

// notify monta e envia o e-mail de um evento.
//
// IDEMPOTÊNCIA sem banco: o Kafka pode entregar o mesmo evento duas vezes.
// Antes de enviar, marcamos no Redis "evento X já notificado" com SET NX (só
// grava se não existir). Se a marca já existir, é repetição: ignoramos. Se o
// envio falhar, apagamos a marca para que a próxima tentativa possa enviar.
func (s *Service) notify(ctx context.Context, envelope contracts.Envelope, r contracts.ReservationSummary, content email) error {
	if r.Holder.Email == "" {
		return nil // reservas de canal costumam vir sem e-mail: nada a enviar
	}
	if r.Source != contracts.SourceWebsite && r.Source != contracts.SourceDirect {
		return nil // hóspedes de Booking/Airbnb são avisados pelo próprio canal
	}

	dedupeKey := "notification:sent:" + envelope.ID
	first, err := s.redis.SetNX(ctx, dedupeKey, "1", 7*24*time.Hour).Result()
	if err != nil {
		return fmt.Errorf("notifier: verificando duplicidade: %w", err)
	}
	if !first {
		return nil
	}

	property, err := s.redis.HGetAll(ctx, propertyKey(r.PropertyID)).Result()
	if err != nil {
		_ = s.redis.Del(ctx, dedupeKey).Err()
		return fmt.Errorf("notifier: lendo dados da propriedade: %w", err)
	}

	data := map[string]any{
		"Heading": content.Heading, "Intro": content.Intro,
		"GuestName": firstName(r.Holder.Name), "Code": r.Code,
		"PropertyName": orDefault(property["name"], "nossa propriedade"),
		"CheckIn":      formatDate(r.CheckIn), "CheckOut": formatDate(r.CheckOut),
		"CheckInTime": property["check_in_time"], "CheckOutTime": property["check_out_time"],
		"Guests": guestsLabel(r.Adults, r.Children), "RoomName": r.RoomName,
		"Total":       formatMoney(r.TotalCents, r.Currency),
		"ShowPayment": content.ShowPayment && content.Deposit > 0,
		"Deposit":     formatMoney(content.Deposit, r.Currency), "PixKey": property["pix_key"],
		"HoldExpiresAt": formatDeadline(content.HoldExpiresAt),
		"Phone":         property["phone"], "Email": property["email"], "Address": property["address"],
		"StatusURL": s.publicURL + "/booking/" + r.Code,
	}
	var html, text bytes.Buffer
	if err := s.html.Execute(&html, data); err != nil {
		return fmt.Errorf("notifier: montando HTML: %w", err)
	}
	if err := s.text.Execute(&text, data); err != nil {
		return fmt.Errorf("notifier: montando texto: %w", err)
	}

	err = s.sender.Send(ctx, mailer.Message{
		To: r.Holder.Email, Subject: content.Subject + " — " + r.Code, HTML: html.String(), Text: text.String(),
	})
	if err != nil {
		_ = s.redis.Del(context.WithoutCancel(ctx), dedupeKey).Err()
		if kernel.IsKind(err, kernel.KindInvalid) {
			s.logger.WarnContext(ctx, "e-mail não enviado: destinatário inválido", slog.String("reservation_id", r.ReservationID))
			return nil
		}
		return err
	}
	// O log registra QUE a notificação saiu, não PARA QUEM (sem dado pessoal).
	s.logger.InfoContext(ctx, "notificação enviada",
		slog.String("reservation_id", r.ReservationID), slog.String("event_type", envelope.Type))
	return nil
}

func propertyKey(propertyID string) string { return "notification:property:" + propertyID }

func firstName(fullName string) string {
	if fields := strings.Fields(fullName); len(fields) > 0 {
		return fields[0]
	}
	return "hóspede"
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func guestsLabel(adults, children int) string {
	label := fmt.Sprintf("%d adulto(s)", adults)
	if children > 0 {
		label += fmt.Sprintf(" e %d criança(s)", children)
	}
	return label
}

// formatDate converte AAAA-MM-DD em DD/MM/AAAA.
func formatDate(iso string) string {
	date, err := kernel.ParseDate(iso)
	if err != nil {
		return iso
	}
	return fmt.Sprintf("%02d/%02d/%04d", date.Day(), int(date.Month()), date.Year())
}

func formatDeadline(deadline *time.Time) string {
	if deadline == nil {
		return ""
	}
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		location = time.UTC
	}
	return deadline.In(location).Format("02/01/2006 às 15:04")
}

// formatMoney formata centavos como "R$ 1.234,56".
func formatMoney(cents int64, currency string) string {
	symbol := currency + " "
	if currency == "BRL" || currency == "" {
		symbol = "R$ "
	}
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	whole := fmt.Sprintf("%d", cents/100)
	var grouped []string
	for len(whole) > 3 {
		grouped = append([]string{whole[len(whole)-3:]}, grouped...)
		whole = whole[:len(whole)-3]
	}
	grouped = append([]string{whole}, grouped...)
	return fmt.Sprintf("%s%s%s,%02d", sign, symbol, strings.Join(grouped, "."), cents%100)
}
