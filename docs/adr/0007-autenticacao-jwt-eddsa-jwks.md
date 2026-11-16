# ADR 0007 — JWT Ed25519 validado por JWKS em cada serviço

**Situação:** aceita

## Contexto

Nove serviços precisam saber quem está chamando e o que ele pode fazer, sem consultar um servidor
central a cada requisição e sem um segredo compartilhado que, vazando de um serviço, permitisse forjar
acessos.

## Decisão

- O `identity` emite **JWT de 15 minutos** assinados com **Ed25519** (EdDSA). Só ele tem a chave privada.
- Os demais validam com a chave **pública**, obtida em `/.well-known/jwks.json` (com `kid`, permitindo
  rotação). **Todos validam**, inclusive atrás do gateway.
- Somente `EdDSA` é aceito na validação.
- **Refresh token** opaco, guardado como SHA-256, com **rotação** e detecção de reuso por família.
- **Logout imediato** por lista de revogação no Redis (TTL = vida restante do token), consultada pelo gateway.
- Autorização **RBAC** (papéis → permissões) + restrição por propriedade, definida em `pkg/kernel/access`.

## Alternativas consideradas

- **HS256 (segredo compartilhado)**: qualquer serviço comprometido poderia emitir tokens de administrador.
- **Sessão no servidor para tudo**: cada requisição a cada serviço exigiria uma consulta central.
- **Provedor externo (Cognito, Keycloak)**: viável no futuro; o formato JWT + JWKS facilita a troca.

## Consequências

- (+) Validação local, rápida e sem segredo compartilhado.
- (−) Um token roubado vale até 15 minutos se o gateway for contornado — mitigado pela vida curta e
  pela NetworkPolicy que isola os serviços.
