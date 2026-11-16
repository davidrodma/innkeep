# Decisões de arquitetura (ADRs)

Um ADR (*Architecture Decision Record*) registra **uma** decisão importante: o contexto, o que foi
decidido, as alternativas e as consequências. Quando uma decisão muda, não se apaga o ADR antigo —
escreve-se um novo que o substitui. Assim fica a história de **por que** o sistema é como é.

| Nº | Decisão | Situação |
|---|---|---|
| [0001](0001-microservicos-orientados-a-eventos.md) | Microserviços orientados a eventos, com banco por serviço | Aceita |
| [0002](0002-monorepo-modulo-unico.md) | Monorepo com um único módulo Go | Aceita |
| [0003](0003-kafka-como-barramento-de-eventos.md) | Apache Kafka como barramento, um tópico por contexto | Aceita |
| [0004](0004-transactional-outbox-e-inbox.md) | Transactional Outbox e Inbox em todos os serviços | Aceita |
| [0005](0005-event-sourcing-somente-em-reservas.md) | Event Sourcing + CQRS apenas no serviço de reservas | Aceita |
| [0006](0006-overbooking-impedido-pelo-banco.md) | Overbooking impedido por restrição EXCLUDE no PostgreSQL | Aceita |
| [0007](0007-autenticacao-jwt-eddsa-jwks.md) | JWT Ed25519 validado por JWKS em cada serviço | Aceita |
| [0008](0008-integracao-com-canais.md) | Canais por porta + adaptadores; iCal como caminho padrão | Aceita |
| [0009](0009-frontend-bff-server-actions.md) | Front-end como BFF com Server Actions e cookies httpOnly | Aceita |
| [0010](0010-dinheiro-em-centavos-e-datas-civis.md) | Dinheiro em centavos e datas sem fuso | Aceita |

Modelo para um ADR novo: copie qualquer um deles e numere em sequência.
