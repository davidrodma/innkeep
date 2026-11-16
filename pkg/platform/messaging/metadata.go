package messaging

import "context"

// Metadata carrega os identificadores que ligam eventos de um mesmo fluxo.
//
//   - CorrelationID: o mesmo valor em TODOS os eventos disparados por uma
//     ação de negócio (ex.: a criação de uma reserva e tudo o que ela
//     desencadeia em inventário, financeiro e canais).
//   - CausationID: o ID do evento que causou diretamente o próximo.
//
// Com os dois dá para reconstruir a árvore de causa e efeito de uma saga.
type Metadata struct {
	CorrelationID string
	CausationID   string
}

type metadataKey struct{}

// WithMetadata devolve um contexto que carrega os metadados informados.
func WithMetadata(ctx context.Context, metadata Metadata) context.Context {
	return context.WithValue(ctx, metadataKey{}, metadata)
}

// MetadataFrom lê os metadados do contexto (struct vazia se não houver).
func MetadataFrom(ctx context.Context) Metadata {
	metadata, _ := ctx.Value(metadataKey{}).(Metadata)
	return metadata
}
