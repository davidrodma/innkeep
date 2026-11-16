# ADR 0001 — Microserviços orientados a eventos, com banco por serviço

**Situação:** aceita

## Contexto

O InnKeep precisa integrar canais externos instáveis (Booking.com, Airbnb), crescer para várias
propriedades e servir de referência de arquitetura orientada a eventos. Partes do domínio mudam em
ritmos diferentes (financeiro, canais, reservas) e têm requisitos diferentes de dados (os dados pessoais
dos hóspedes exigem isolamento).

## Decisão

- Dividir o sistema em serviços por **bounded context** (DDD): identity, property, inventory,
  reservation, guest, billing, channel, notification, mais o gateway.
- **Cada serviço é dono do seu banco**; nenhum lê o banco de outro.
- Comunicação **síncrona só na borda** (navegador → gateway → serviço dono do dado). **Entre serviços,
  apenas eventos** no Kafka.
- Dados de outros contextos ficam em **cópias locais** alimentadas por eventos (Event-Carried State Transfer).

## Alternativas consideradas

- **Monólito modular**: mais simples de operar e suficiente para uma propriedade pequena. Descartado
  porque o objetivo explícito do projeto é a arquitetura distribuída e porque a falha de um canal
  externo não deve afetar a recepção.
- **Microserviços com REST entre si**: acoplaria a disponibilidade (se o inventário cai, a reserva
  cai junto) e geraria cadeias de chamadas difíceis de rastrear.

## Consequências

- (+) Serviços isolados: o check-in funciona com o serviço de hóspedes fora do ar.
- (+) Novos consumidores podem reagir a eventos existentes sem alterar os produtores.
- (−) Consistência eventual: a interface precisa mostrar estados intermediários ("processando").
- (−) Mais peças para operar: exige observabilidade (traces distribuídos) desde o início.
