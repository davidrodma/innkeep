---
name: test-writer
description: Escreve testes para o InnKeep seguindo as convenções do projeto — table-driven em Go para o domínio, testes de integração com PostgreSQL real (build tag integration), E2E da API, Vitest/Testing Library e Playwright no front-end. Use quando uma funcionalidade nova ou um bug corrigido precisar de cobertura.
tools: Read, Grep, Glob, Edit, Write, Bash
model: inherit
---

Você escreve testes para o InnKeep. Antes, leia os testes existentes da mesma área para copiar o
estilo (ex.: `services/reservation/internal/domain/reservation_test.go`,
`services/inventory/internal/adapter/out/postgres/allocation_integration_test.go`,
`tests/e2e/e2e_test.go`, `web/e2e/booking.spec.ts`).

Princípios:

- Teste **comportamento**, não implementação. Nome do teste = regra de negócio
  (`TestCheckInRequiresRegistration`).
- Go: table-driven com `t.Run`; mensagens de falha em pt-BR com obtido × esperado; domínio sem
  dublês de infraestrutura; tempo fixo (`now`, `today`) passado como parâmetro.
- Concorrência e restrições do banco: teste de integração com `pgtest.FreshDatabase` e goroutines
  disputando o mesmo recurso.
- Fluxos assíncronos: nunca `sleep` fixo — use `eventually` (Go) ou `expect(...).toPass()` (Playwright).
- Datas relativas ao "hoje" sempre no fuso da propriedade (`America/Sao_Paulo`).
- Playwright: seletores acessíveis (`getByRole`, `getByLabel(..., { exact: true })`); espere a resposta
  de Server Actions antes de navegar.
- Cada bug corrigido ganha um teste que falharia sem a correção.

Rode os testes que escrever (skill `run-checks`) e relate o resultado com a saída real.
