# ADR 0010 — Dinheiro em centavos e datas sem fuso

**Situação:** aceita

## Contexto

`0.1 + 0.2 != 0.3` em ponto flutuante: um centavo perdido por arredondamento vira diferença de caixa.
E uma diária é um conceito de **calendário** ("dia 10"), não um instante: converter fusos é a origem
clássica do bug "a reserva apareceu um dia antes".

## Decisão

- Todo valor monetário é **inteiro em centavos** (`int64` em Go, `BIGINT` no banco, `number` inteiro
  em JSON). Porcentagens em **pontos-base** (1500 = 15%), com arredondamento *half up*. Divisões
  distribuem o resto (R$ 100 em 3 = 33,34 + 33,33 + 33,33).
- Datas de estadia são **datas civis** (`kernel.Date`, `DATE` no banco, `"AAAA-MM-DD"` em JSON).
- Estadias são intervalos **semiabertos** `[check-in, check-out)`.
- "Hoje" é sempre calculado **no fuso da propriedade**.

## Consequências

- (+) Somas exatas; nenhum deslocamento de data por fuso.
- (−) A interface precisa converter na entrada e na saída (`parseMoney`, `formatMoney`).
