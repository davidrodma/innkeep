# ADR 0005 — Event Sourcing + CQRS apenas no serviço de reservas

**Situação:** aceita

## Contexto

A reserva tem um ciclo de vida rico (solicitada, pendente, confirmada, hospedada, alterada,
cancelada...) e disputas comuns na hotelaria ("quem cancelou?", "qual era o valor antes da
alteração?"). Event Sourcing responde a isso por construção, mas tem custo de complexidade.

## Decisão

- O serviço `reservation` guarda os **eventos de domínio** em um Event Store (tabela `events`,
  append-only) e reconstrói o agregado por replay.
- Concorrência **otimista** pela restrição `UNIQUE (aggregate_id, version)`; o comando é repetido em
  caso de conflito.
- **CQRS**: as consultas leem a projeção `reservation_views`, atualizada na **mesma transação** que
  grava os eventos (sem atraso nem divergência). A projeção pode ser reconstruída a qualquer momento.
- Eventos de **domínio** (internos) são traduzidos para eventos de **integração** (públicos) antes de
  publicar.
- Os demais serviços usam persistência de estado comum.

## Consequências

- (+) Auditoria completa e imutável; linha do tempo de cada reserva na interface.
- (+) Novas visões (relatórios, BI) podem ser construídas relendo o histórico.
- (−) Eventos gravados são contratos eternos: evoluem só por acréscimo.
- (−) Mais conceitos para quem chega — compensado pela documentação e pelos testes "dado/quando/então".
