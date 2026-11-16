---
name: add-endpoint
description: Acrescenta um endpoint REST a um serviço do InnKeep, de ponta a ponta — caso de uso, handler com DTO e permissão, rota no gateway, referência da API e, se for o caso, a consulta/Server Action e a tela no front-end. Use quando pedirem uma operação nova na API ou no painel.
---

# Novo endpoint REST

1. **Caso de uso** em `services/<serviço>/internal/app/` — regra de negócio no domínio; o caso de uso
   orquestra (carrega, valida acesso, altera, grava + publica na mesma transação).
2. **Handler** em `internal/adapter/in/rest/handler.go`:
   - rota no `Register`: `mux.Handle("POST /v1/<recurso>/{id}/<ação>", write(h.metodo))` — escolha a
     permissão certa (`access.Perm...`); rotas públicas só sob `/v1/public/`;
   - leia com `httpx.Decode` (DTO próprio), IDs com `httpx.PathID`;
   - **antes de alterar**, descubra a propriedade do recurso e chame `access.EnsureProperty`;
   - responda com `httpx.JSON` e um DTO de saída (nunca a entidade de domínio);
   - criações e pagamentos: envolva com o middleware `idempotent(...)`.
3. **Gateway**: se o prefixo da rota for novo, acrescente em `services/gateway/cmd/gateway/main.go`
   (rotas públicas sensíveis com `Sensitive: true`).
4. **Referência**: `python3 scripts/list-endpoints.py > docs/api.md`.
5. **Front-end** (se houver tela):
   - leitura: função em `web/src/features/<área>/queries.ts` usando `api<T>()`;
   - escrita: Server Action em `actions.ts` devolvendo `ActionState` com `failure(error)`;
   - formulário com `useActionState` + `Field`/`Input`/`SubmitButton`/`FormMessage`;
   - tipos em `web/src/lib/types.ts`, rótulos em `web/src/lib/labels.ts`.
6. **Testes**: unitário do caso de uso; E2E da API em `tests/e2e` se atravessar serviços; Playwright
   se for um fluxo importante da interface. Finalize com a skill `run-checks`.
