# ADR 0009 — Front-end como BFF com Server Actions e cookies httpOnly

**Situação:** aceita

## Contexto

O painel lida com dados sensíveis (hóspedes, pagamentos). Guardar o token no `localStorage` e chamar
a API pelo navegador expõe o token a qualquer script injetado (XSS) e obriga a abrir CORS.

## Decisão

- Next.js (App Router) como **Backend for Frontend**: páginas são Server Components que chamam o
  gateway pela rede interna; formulários usam **Server Actions**.
- Tokens em **cookies `httpOnly`, `SameSite=Lax`** (e `Secure` em produção). Nenhum JavaScript da página
  enxerga o token.
- `proxy.ts` protege o `/admin` e renova o token de acesso usando o refresh token.
- A validação de negócio fica no back-end; o front-end exibe os erros por campo vindos da API
  (Problem Details).
- Estados intermediários das sagas são tratados com atualização automática (`AutoRefresh`).

## Consequências

- (+) Superfície de ataque menor; sem CORS para o painel.
- (+) HTML renderizado no servidor: rápido e acessível.
- (−) Toda interação passa pelo servidor do Next.js (escala-se horizontalmente como qualquer serviço).
