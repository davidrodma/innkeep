---
paths:
  - "pkg/contracts/**"
  - "services/*/internal/adapter/in/consumer/**"
  - "pkg/platform/messaging/**"
  - "pkg/platform/outbox/**"
---

# Regras para eventos de integração

- Nome: `innkeep.<contexto>.<fato_no_passado>.v<N>`. O `<contexto>` define o tópico
  (`innkeep.<contexto>.events`) — veja `contracts.TopicFor`.
- `PartitionKey()` = ID do agregado cuja ordem importa (reserva, propriedade, tipo de quarto).
- **Evolução**: acrescentar campo opcional (`omitempty`) é compatível. Renomear, remover ou mudar o
  significado exige um tipo novo `.v2`, publicado em paralelo até todos os consumidores migrarem.
- **Sem dados pessoais sensíveis** (documento, endereço). Nome e contato do titular só quando o
  consumidor precisa (e-mail de confirmação).
- Prefira carregar o estado necessário no evento (Event-Carried State Transfer) a obrigar o
  consumidor a chamar o produtor.
- Consumidores: idempotentes; toleram eventos fora de ordem **entre tópicos**; erro de validação/regra
  não é repetido (vai para a DLQ), "não encontrado" é repetido.
- Ao criar ou alterar um evento, atualize `docs/architecture/events.md`.
