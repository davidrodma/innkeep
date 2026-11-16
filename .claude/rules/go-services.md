---
paths:
  - "services/**/*.go"
  - "pkg/**/*.go"
  - "tools/**/*.go"
  - "tests/**/*.go"
---

# Regras para código Go

## Camadas (arquitetura hexagonal)

- `internal/domain/`: entidades, objetos de valor, regras. Importa **somente** `pkg/kernel` e a stdlib.
  Recebe o tempo como parâmetro (`now time.Time`, `today kernel.Date`) — nunca chama `time.Now()`.
- `internal/app/`: casos de uso e **portas** (interfaces) do que precisa do mundo externo.
- `internal/adapter/in/`: `rest/` (HTTP → caso de uso) e `consumer/` (evento → caso de uso). Sem regra de negócio.
- `internal/adapter/out/`: implementações das portas (postgres, redis, clientes externos).
- `cmd/<serviço>/main.go`: única raiz de composição; injeção de dependências manual.

## Padrões obrigatórios

- Erros de negócio com `kernel.Invalid/NotFound/Conflict/Rule/...` e `Code` estável (`"folio.closed"`);
  campos com `.WithField`. Compare com `errors.Is`/`kernel.CodeOf`, nunca por texto.
- Toda escrita que gera evento: `tx.WithinTx(ctx, func(ctx) error { repo.Save(...); publisher.Publish(...) })`.
- Consumidores registrados com `messaging.Typed` e roteador com `inbox.Middleware(tx, "<serviço>")`.
- Handlers HTTP: `httpx.HandlerFunc` devolvendo erro; `httpx.Decode` para o corpo; `httpx.PathID`/`QueryID`
  para IDs; resposta com DTOs próprios (nunca serializar entidades de domínio direto).
- Autorização: `guard.Require(access.Perm...)` na rota **e** `access.EnsureProperty` assim que a
  propriedade do recurso for conhecida — **antes** de alterar qualquer coisa.
- Dinheiro em centavos (`int64` / `kernel.Money`); porcentagem em pontos-base; datas de estadia `kernel.Date`.
- SQL sempre parametrizado (`$1`); listas com `= ANY($n)`; nada de `fmt.Sprintf` com valores do usuário.
- Logs com `slog` e atributos (`slog.String("reservation_id", id)`); **nunca** dados pessoais.
- Recursos externos (HTTP, banco, Kafka) sempre com `context.Context` e timeout.

## Testes

- Table-driven com `t.Run`; nomes de teste descrevem o comportamento (`TestCheckInRequiresRegistration`).
- Domínio: testes puros, formato dado/quando/então. Event Sourcing: provar que o replay reconstrói o estado.
- Integração com banco real: arquivo `*_integration_test.go`, build tag `integration`, `pgtest.FreshDatabase`.
- Mensagens de falha em português, mostrando obtido × esperado.

## Comentários

Em pt-BR, explicando a intenção e o risco evitado. Todo pacote tem comentário `// Package x ...`.
