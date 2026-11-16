---
paths:
  - "web/**"
---

# Regras para o front-end (Next.js 16)

- **Next.js 16**: o middleware chama-se `src/proxy.ts`; `params`, `searchParams` e `cookies()` são
  `Promise` (use `await`). Em dúvida, consulte `web/node_modules/next/dist/docs/`.
- Páginas são **Server Components**. Dados via `src/lib/api.ts` (`import "server-only"`); o navegador
  nunca chama o gateway diretamente e nunca vê o token.
- Formulários com **Server Actions** em `src/features/<área>/actions.ts` (`"use server"`), devolvendo
  `ActionState` via `failure(error)`; componentes de formulário usam `useActionState` e os
  componentes de `src/components/ui/form.tsx` (`Field`, `Input`, `SubmitButton`, `FormMessage`).
- **Não duplique regras de negócio** no cliente: mostre os erros por campo devolvidos pela API.
- Criações e pagamentos enviam `idempotencyKey` (gerada no servidor ao renderizar o formulário).
- Estados intermediários de sagas: renderize o estado atual e use `<AutoRefresh />` enquanto durar.
- Dinheiro: `formatMoney(centavos)` e `parseMoney(texto)` — nunca `Number(x) * 100`.
- Datas de estadia como `"AAAA-MM-DD"`; use `addDays`, `nightsBetween` de `src/lib/format.ts`.
- Textos da interface em pt-BR; códigos da API traduzidos em `src/lib/labels.ts`.
- Acessibilidade: todo campo com rótulo (`Field`), erros com `aria-describedby`, mensagens com
  `role="alert"`/`"status"`, foco visível.
- Tailwind 4: o tema fica em `src/app/globals.css` (`@theme`); cores `brand-*` e `ink-*`.
- TypeScript fixado em 6.0.x (compatibilidade do typescript-eslint).
- Testes: Vitest + Testing Library para lógica e componentes (`*.test.ts[x]`); Playwright em `e2e/`
  com seletores acessíveis (`getByRole`, `getByLabel(..., { exact: true })`).
