-- +goose Up
-- Usuários internos (funcionários com acesso ao painel administrativo).
CREATE TABLE users (
    id            UUID        PRIMARY KEY,
    -- O e-mail é sempre normalizado em minúsculas pela aplicação.
    email         TEXT        NOT NULL UNIQUE,
    name          TEXT        NOT NULL,
    -- Hash Argon2id no formato PHC. A senha em texto puro nunca é gravada.
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL,
    -- Propriedades às quais o usuário tem acesso. Vazio = todas.
    property_ids  UUID[]      NOT NULL DEFAULT '{}',
    active        BOOLEAN     NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sessões: um registro por refresh token emitido.
CREATE TABLE sessions (
    id         UUID        PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Tokens derivados do mesmo login compartilham a família (rotação).
    family_id  UUID        NOT NULL,
    -- SHA-256 do refresh token; o token em si não é armazenado.
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_family_idx ON sessions (family_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
DROP TABLE users;
