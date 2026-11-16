# Referência da API REST

> Gerado por `scripts/list-endpoints.py` a partir do código — não edite à mão.

Todas as rotas passam pelo **gateway** (`/v1/...`). São 82 endpoints em 7 serviços.
Rotas marcadas como *público* não exigem token; as demais exigem `Authorization: Bearer <token>`
e a permissão indicada (veja os papéis em `pkg/kernel/access/access.go`).

Erros seguem o formato **Problem Details (RFC 9457)**: `{type, title, status, detail, code, errors, trace_id}`.

## billing

| Método | Rota | Acesso |
|---|---|---|
| `GET` | `/v1/folios` | billing:read |
| `GET` | `/v1/folios/by-reservation/{reservation_id}` | billing:read |
| `GET` | `/v1/folios/{id}` | billing:read |
| `POST` | `/v1/folios/{id}/charges` | billing:write |
| `POST` | `/v1/folios/{id}/payments` | billing:write · aceita `Idempotency-Key` |
| `POST` | `/v1/folios/{id}/refunds` | billing:write |
| `POST` | `/v1/folios/{id}/entries/{entry_id}/void` | billing:write |
| `GET` | `/v1/payment-methods` | billing:read |
| `GET` | `/v1/expenses` | finance:read |
| `POST` | `/v1/expenses` | finance:write |
| `PUT` | `/v1/expenses/{id}` | finance:write |
| `POST` | `/v1/expenses/{id}/pay` | finance:write |
| `DELETE` | `/v1/expenses/{id}` | finance:write |
| `GET` | `/v1/cash/movements` | finance:read |
| `POST` | `/v1/cash/movements` | finance:write |
| `GET` | `/v1/fee-rules` | finance:read |
| `PUT` | `/v1/fee-rules` | finance:write |
| `GET` | `/v1/reports/financial` | finance:read |

## channel

| Método | Rota | Acesso |
|---|---|---|
| `GET` | `/v1/public/channels/ical/{token}` | público |
| `GET` | `/v1/channels/connections` | channel:read |
| `POST` | `/v1/channels/connections` | channel:write |
| `PUT` | `/v1/channels/connections/{id}` | channel:write |
| `DELETE` | `/v1/channels/connections/{id}` | channel:write |
| `POST` | `/v1/channels/connections/{id}/sync` | channel:write |
| `POST` | `/v1/channels/connections/{id}/pull` | channel:write |
| `GET` | `/v1/channels/connections/{id}/mappings` | channel:read |
| `POST` | `/v1/channels/connections/{id}/mappings` | channel:write |
| `GET` | `/v1/channels/connections/{id}/sync-log` | channel:read |
| `DELETE` | `/v1/channels/mappings/{id}` | channel:write |

## guest

| Método | Rota | Acesso |
|---|---|---|
| `GET` | `/v1/guests` | guest:read |
| `POST` | `/v1/guests` | guest:write |
| `GET` | `/v1/guests/{id}` | guest:read |
| `PUT` | `/v1/guests/{id}` | guest:write |
| `GET` | `/v1/guests/{id}/stays` | guest:read |
| `POST` | `/v1/guests/{id}/anonymize` | guest:write |
| `GET` | `/v1/registrations/{reservation_id}` | guest:read |
| `PUT` | `/v1/registrations/{reservation_id}` | guest:write |
| `GET` | `/v1/registration-options` | guest:read |

## identity

| Método | Rota | Acesso |
|---|---|---|
| `GET` | `/.well-known/jwks.json` | público |
| `POST` | `/v1/auth/login` | público |
| `POST` | `/v1/auth/refresh` | público |
| `POST` | `/v1/auth/logout` | público (token opcional) |
| `GET` | `/v1/auth/me` | autenticado |
| `PUT` | `/v1/auth/password` | autenticado |
| `GET` | `/v1/users` | user:manage |
| `POST` | `/v1/users` | user:manage |
| `GET` | `/v1/users/{id}` | user:manage |
| `PUT` | `/v1/users/{id}` | user:manage |
| `PUT` | `/v1/users/{id}/password` | user:manage |

## inventory

| Método | Rota | Acesso |
|---|---|---|
| `GET` | `/v1/public/availability` | público |
| `GET` | `/v1/availability` | inventory:read |
| `GET` | `/v1/free-rooms` | inventory:read |
| `PUT` | `/v1/rates` | inventory:write |
| `GET` | `/v1/blocks` | inventory:read |
| `POST` | `/v1/blocks` | inventory:write |
| `DELETE` | `/v1/blocks/{id}` | inventory:write |

## property

| Método | Rota | Acesso |
|---|---|---|
| `GET` | `/v1/public/properties` | público |
| `GET` | `/v1/public/properties/{slug}` | público |
| `GET` | `/v1/properties` | property:read |
| `POST` | `/v1/properties` | property:write |
| `GET` | `/v1/properties/{id}` | property:read |
| `PUT` | `/v1/properties/{id}` | property:write |
| `GET` | `/v1/properties/{id}/room-types` | property:read |
| `POST` | `/v1/properties/{id}/room-types` | property:write |
| `PUT` | `/v1/room-types/{id}` | property:write |
| `GET` | `/v1/properties/{id}/rooms` | property:read |
| `POST` | `/v1/properties/{id}/rooms` | property:write |
| `PUT` | `/v1/rooms/{id}` | property:write |
| `PUT` | `/v1/rooms/{id}/housekeeping` | housekeeping:write |

## reservation

| Método | Rota | Acesso |
|---|---|---|
| `POST` | `/v1/public/reservations` | público · aceita `Idempotency-Key` |
| `POST` | `/v1/public/reservations/lookup` | público |
| `GET` | `/v1/reservations` | reservation:read |
| `GET` | `/v1/reservations/dashboard` | reservation:read |
| `GET` | `/v1/reservations/{id}` | reservation:read |
| `GET` | `/v1/reservations/{id}/history` | reservation:read |
| `POST` | `/v1/reservations` | reservation:write · aceita `Idempotency-Key` |
| `POST` | `/v1/reservations/{id}/confirm` | reservation:write |
| `POST` | `/v1/reservations/{id}/cancel` | reservation:write |
| `POST` | `/v1/reservations/{id}/check-in` | reservation:write |
| `POST` | `/v1/reservations/{id}/check-out` | reservation:write |
| `POST` | `/v1/reservations/{id}/no-show` | reservation:write |
| `POST` | `/v1/reservations/{id}/change-stay` | reservation:write |

