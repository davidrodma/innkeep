// Package kernel é o "Shared Kernel" do DDD: um conjunto pequeno e estável de
// tipos de valor que todos os bounded contexts compartilham (dinheiro, datas,
// período de estadia e erros de domínio).
//
// Regra de ouro: este pacote só pode importar a biblioteca padrão do Go. Ele é
// o centro da arquitetura — tudo depende dele e ele não depende de ninguém.
package kernel

import (
	"errors"
	"fmt"
)

// ErrorKind classifica um erro de forma independente de transporte. Os
// adaptadores (HTTP, Kafka) traduzem o Kind para o que fizer sentido no seu
// mundo: um status HTTP, uma decisão de retry, etc. O domínio nunca conhece
// status HTTP.
type ErrorKind string

const (
	KindInvalid       ErrorKind = "invalid"        // entrada malformada ou inválida
	KindNotFound      ErrorKind = "not_found"      // recurso inexistente
	KindConflict      ErrorKind = "conflict"       // estado atual impede a operação (ex.: concorrência)
	KindRuleViolation ErrorKind = "rule_violation" // regra de negócio violada
	KindUnauthorized  ErrorKind = "unauthorized"   // não autenticado
	KindForbidden     ErrorKind = "forbidden"      // autenticado, porém sem permissão
	KindRateLimited   ErrorKind = "rate_limited"   // excesso de requisições
	KindUnavailable   ErrorKind = "unavailable"    // dependência fora do ar (erro transitório)
	KindInternal      ErrorKind = "internal"       // erro inesperado
)

// Error é o erro padrão do domínio e da aplicação.
//
// Code é um identificador estável e legível por máquina (ex.:
// "reservation.invalid_transition"); clientes de API podem tomar decisões com
// base nele sem depender do texto de Message.
type Error struct {
	Kind    ErrorKind
	Code    string
	Message string
	// Fields aponta problemas por campo (validação de formulários).
	Fields map[string]string
	cause  error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap permite usar errors.Is / errors.As através da causa original.
func (e *Error) Unwrap() error { return e.cause }

// Is faz errors.Is reconhecer um erro de domínio mesmo depois de Wrap ou
// WithField — que devolvem CÓPIAS, ou seja, ponteiros diferentes. Sem este
// método, errors.Is(err, ErrConcurrency) compararia ponteiros e falharia em
// silêncio. Dois *Error são equivalentes quando têm o mesmo Kind e o mesmo Code.
func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && e.Kind == other.Kind && e.Code == other.Code
}

// WithField devolve uma cópia do erro com o detalhe de um campo.
func (e *Error) WithField(field, message string) *Error {
	clone := *e
	clone.Fields = make(map[string]string, len(e.Fields)+1)
	for k, v := range e.Fields {
		clone.Fields[k] = v
	}
	clone.Fields[field] = message
	return &clone
}

// Wrap devolve uma cópia do erro guardando a causa original (para logs).
func (e *Error) Wrap(cause error) *Error {
	clone := *e
	clone.cause = cause
	return &clone
}

func newError(kind ErrorKind, code, format string, args ...any) *Error {
	return &Error{Kind: kind, Code: code, Message: fmt.Sprintf(format, args...)}
}

// Invalid sinaliza entrada inválida.
func Invalid(code, format string, args ...any) *Error {
	return newError(KindInvalid, code, format, args...)
}

// NotFound sinaliza recurso inexistente.
func NotFound(code, format string, args ...any) *Error {
	return newError(KindNotFound, code, format, args...)
}

// Conflict sinaliza conflito com o estado atual.
func Conflict(code, format string, args ...any) *Error {
	return newError(KindConflict, code, format, args...)
}

// Rule sinaliza violação de regra de negócio.
func Rule(code, format string, args ...any) *Error {
	return newError(KindRuleViolation, code, format, args...)
}

// Unauthorized sinaliza ausência de autenticação válida.
func Unauthorized(code, format string, args ...any) *Error {
	return newError(KindUnauthorized, code, format, args...)
}

// Forbidden sinaliza falta de permissão.
func Forbidden(code, format string, args ...any) *Error {
	return newError(KindForbidden, code, format, args...)
}

// Unavailable sinaliza falha transitória de uma dependência.
func Unavailable(code, format string, args ...any) *Error {
	return newError(KindUnavailable, code, format, args...)
}

// Internal sinaliza erro inesperado, preservando a causa.
func Internal(cause error, format string, args ...any) *Error {
	e := newError(KindInternal, "internal", format, args...)
	e.cause = cause
	return e
}

// KindOf devolve o Kind de err, ou KindInternal se err não for um *Error.
func KindOf(err error) ErrorKind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}

// CodeOf devolve o Code de err, ou "internal" se err não for um *Error.
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "internal"
}

// IsKind informa se err (ou alguma causa encadeada) é um *Error do kind dado.
func IsKind(err error, kind ErrorKind) bool {
	var e *Error
	return errors.As(err, &e) && e.Kind == kind
}
