# ADR 0008 — Canais por porta + adaptadores; iCal como caminho padrão

**Situação:** aceita

## Contexto

O Booking.com é o principal parceiro, seguido do Airbnb. Consulta à documentação oficial (novembro de
2026): a Connectivity API da Booking.com exige ser **parceiro de conectividade certificado**; a API do
Airbnb é restrita a parceiros homologados. Ambas as plataformas oferecem **iCal** para qualquer conta.

## Decisão

- Porta `Connector` no serviço `channel` (enviar disponibilidade, enviar tarifas, buscar reservas,
  confirmar recebimento) e um adaptador por canal/modo (padrão **Strategy**):
  - `bookingcom`: XML OTA (`OTA_HotelAvailNotif`, `OTA_HotelRateAmountNotif`, `OTA_HotelResNotif`,
    `OTA_HotelResModifyNotif`), autenticação por token;
  - `ical`: exporta as datas esgotadas por tipo de quarto (URL com token secreto) e importa o calendário
    do canal.
- O mapeamento é por **tipo de quarto** (é o tipo que se vende). O inventário escolhe o quarto físico.
- Reservas de canal recebem **ID determinístico** (UUID v5 de propriedade + canal + número externo).
- Um **simulador** (`tools/otasim`) implementa o subconjunto do protocolo usado, para desenvolvimento e testes.
- URLs informadas por usuários passam por um cliente HTTP com proteção contra **SSRF**.

## Consequências

- (+) Funciona hoje com iCal; quando a certificação sair, basta trocar o modo da conexão.
- (+) Um canal novo é um adaptador novo, sem mexer no restante.
- (−) iCal não é tempo real: há uma janela (minutos a horas) em que um overbooking é possível. O
  sistema aceita a reserva do canal sinalizando o overbooking para a equipe resolver.
- (−) Dois detalhes da API da Booking.com foram implementados por inferência e precisam ser
  confirmados na certificação (estão marcados no código).
