# Glossário

## Hotelaria

| Termo | Significado |
|---|---|
| **PMS** | *Property Management System*: o sistema central de gestão de um meio de hospedagem. |
| **Tipo de acomodação** (*room type*) | Categoria vendida ("Suíte", "Quarto casal"). É o tipo que tem preço e que se vende nos canais. |
| **Quarto / unidade** | O espaço físico ocupado (quarto 101, cama A do dormitório). |
| **Diária** | O preço de uma noite. No InnKeep, uma diária pertence à noite que **começa** naquela data. |
| **Sinal / depósito** | Parte do valor paga para garantir a reserva. Configurável por propriedade (percentual e prazo). |
| **Folio** | A conta corrente da reserva: diárias, extras, descontos, pagamentos e devoluções. |
| **No-show** | O hóspede não apareceu. Libera o quarto; multas são lançadas manualmente. |
| **Walk-in** | Hóspede que chega sem reserva (uma reserva direta lançada na hora). |
| **Overbooking** | Mais reservas do que quartos. Impedido internamente; sinalizado quando vem de um canal externo. |
| **Stop-sell** | Fechar as vendas de uma data (no InnKeep: "fechar vendas" na grade de tarifas). |
| **Estadia mínima** (*min stay / MinLOS*) | Número mínimo de noites, avaliado no dia da chegada. |
| **Bloqueio** | Tirar um quarto de venda por um período (manutenção, uso do proprietário). |
| **Governança** (*housekeeping*) | Equipe e situação de limpeza dos quartos (limpo/sujo). |
| **Tape chart** | Calendário de ocupação em grade quarto × dia. |
| **OTA** | *Online Travel Agency*: Booking.com, Expedia, Decolar. |
| **Channel manager** | Sistema que sincroniza disponibilidade, preços e reservas com as OTAs. |
| **ARI** | *Availability, Rates and Inventory*: o que um channel manager envia às OTAs. |
| **iCal** | Formato de calendário (`.ics`) usado para sincronização simples entre plataformas. |
| **Taxa de ocupação** | Quartos-noite vendidos ÷ quartos-noite disponíveis. |
| **ADR** | *Average Daily Rate*: receita de hospedagem ÷ quartos-noite vendidos (diária média). |
| **RevPAR** | *Revenue per Available Room*: receita de hospedagem ÷ quartos-noite disponíveis. |
| **FNRH** | Ficha Nacional de Registro de Hóspedes, exigida pela Lei Geral do Turismo (Lei 11.771/2008). |
| **Cadastur** | Cadastro obrigatório dos prestadores de serviços turísticos no Ministério do Turismo. |
| **Sangria / suprimento** | Retirada / reforço de dinheiro no caixa físico. |

## Arquitetura

| Termo | Significado |
|---|---|
| **Bounded context** | Fronteira de um modelo do domínio, onde cada termo tem um significado único (DDD). Vira um serviço. |
| **Agregado** | Grupo de objetos tratado como uma unidade de consistência, com uma raiz (ex.: `Reservation`, `Folio`). |
| **Objeto de valor** | Imutável e comparado por valor (`Money`, `StayPeriod`). |
| **Shared Kernel** | Pequeno conjunto de tipos compartilhado por todos os contextos (`pkg/kernel`). |
| **Anti-Corruption Layer** | Camada que traduz o modelo de um sistema externo para o modelo interno (adaptador da Booking.com). |
| **Evento de domínio** | Fato interno de um agregado, gravado no Event Store. |
| **Evento de integração** | Fato publicado no Kafka para outros serviços (`pkg/contracts`). |
| **Event Sourcing** | Guardar a sequência de eventos em vez do estado atual. |
| **CQRS** | Separar o modelo de escrita (comandos) do de leitura (consultas). |
| **Projeção** | Modelo de leitura construído a partir dos eventos (`reservation_views`). |
| **Saga** | Transação de negócio dividida em transações locais coordenadas por eventos, com compensações. |
| **Outbox / Inbox** | Tabelas que garantem publicação sem perda e consumo sem duplicidade. |
| **Idempotência** | Repetir a operação não muda o resultado. |
| **Consistência eventual** | Os serviços concordam depois de um breve intervalo, não no mesmo instante. |
| **DLQ** | *Dead Letter Queue*: destino das mensagens que falharam repetidamente. |
| **Porta / adaptador** | Interface que o núcleo declara / implementação técnica dela (arquitetura hexagonal). |
| **BFF** | *Backend for Frontend*: servidor dedicado a uma interface (o servidor do Next.js). |
| **JWKS** | Conjunto de chaves públicas para validar JWTs (`/.well-known/jwks.json`). |
| **OTLP** | Protocolo do OpenTelemetry para traces, métricas e logs. |
| **Trace / span** | O caminho de uma requisição / cada etapa dele. |
