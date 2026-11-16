package messaging

import (
	"context"

	"github.com/davidrodma/innkeep/pkg/contracts"
)

// Handler processa um evento. Devolver erro provoca nova tentativa.
type Handler func(ctx context.Context, envelope contracts.Envelope) error

// Middleware envolve um Handler para acrescentar comportamento transversal
// (idempotência, transação, logs). É o padrão Decorator — o mesmo conceito dos
// middlewares HTTP, aplicado a eventos.
type Middleware func(next Handler) Handler

// Router associa tipos de evento aos seus handlers. Eventos sem handler são
// ignorados em silêncio: um serviço só reage ao que lhe interessa, e novos
// tipos de evento podem surgir no tópico sem quebrar consumidores antigos.
type Router struct {
	handlers    map[string]Handler
	middlewares []Middleware
}

// NewRouter cria um roteador. Os middlewares são aplicados a todos os
// handlers, na ordem informada (o primeiro é o mais externo).
func NewRouter(middlewares ...Middleware) *Router {
	return &Router{handlers: make(map[string]Handler), middlewares: middlewares}
}

// On registra o handler de um tipo de evento.
func (r *Router) On(eventType string, handler Handler) *Router {
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i](handler)
	}
	r.handlers[eventType] = handler
	return r
}

// Dispatch entrega o envelope ao handler registrado. O bool informa se havia
// handler. Exposto para permitir testar handlers sem um Kafka de verdade.
func (r *Router) Dispatch(ctx context.Context, envelope contracts.Envelope) (bool, error) {
	handler, ok := r.handlers[envelope.Type]
	if !ok {
		return false, nil
	}
	return true, handler(ctx, envelope)
}

// Types lista os tipos de evento com handler registrado.
func (r *Router) Types() []string {
	types := make([]string, 0, len(r.handlers))
	for eventType := range r.handlers {
		types = append(types, eventType)
	}
	return types
}

// Typed adapta uma função que recebe o payload já decodificado para a
// assinatura de Handler, eliminando o json.Unmarshal repetido em cada handler:
//
//	router.On(contracts.TypeReservationConfirmed,
//	    messaging.Typed(func(ctx context.Context, env contracts.Envelope, evt contracts.ReservationConfirmed) error {
//	        ...
//	    }))
func Typed[T any](fn func(ctx context.Context, envelope contracts.Envelope, payload T) error) Handler {
	return func(ctx context.Context, envelope contracts.Envelope) error {
		payload, err := contracts.Decode[T](envelope)
		if err != nil {
			return err
		}
		return fn(ctx, envelope, payload)
	}
}
