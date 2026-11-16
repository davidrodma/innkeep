-- +goose Up

-- Conexões de uma propriedade com os canais de venda.
CREATE TABLE connections (
    id                   UUID        PRIMARY KEY,
    property_id          UUID        NOT NULL,
    channel              TEXT        NOT NULL,
    mode                 TEXT        NOT NULL,
    name                 TEXT        NOT NULL,
    external_property_id TEXT        NOT NULL DEFAULT '',
    -- Credenciais da API do canal, cifradas pela aplicação (AES-256-GCM).
    credentials_enc      BYTEA,
    base_url             TEXT        NOT NULL DEFAULT '',
    active               BOOLEAN     NOT NULL DEFAULT TRUE,
    last_push_at         TIMESTAMPTZ,
    last_pull_at         TIMESTAMPTZ,
    last_error           TEXT        NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL,
    updated_at           TIMESTAMPTZ NOT NULL
);
CREATE INDEX connections_property_idx ON connections (property_id);

-- Mapeamento: tipo de quarto nosso <-> anúncio no canal.
CREATE TABLE mappings (
    id                UUID        PRIMARY KEY,
    connection_id     UUID        NOT NULL REFERENCES connections (id) ON DELETE CASCADE,
    property_id       UUID        NOT NULL,
    room_type_id      UUID        NOT NULL,
    external_room_id  TEXT        NOT NULL DEFAULT '',
    external_rate_id  TEXT        NOT NULL DEFAULT '',
    ical_import_url   TEXT        NOT NULL DEFAULT '',
    -- Segredo que compõe a URL do calendário exportado.
    ical_export_token TEXT        NOT NULL UNIQUE,
    created_at        TIMESTAMPTZ NOT NULL,
    UNIQUE (connection_id, room_type_id)
);
CREATE INDEX mappings_room_type_idx ON mappings (room_type_id);

-- Retrato local da disponibilidade (eventos do inventário). Alimenta os
-- calendários iCal exportados.
CREATE TABLE availability_snapshot (
    room_type_id UUID        NOT NULL,
    day          DATE        NOT NULL,
    available    INT         NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_type_id, day)
);

-- O que já foi importado de cada canal (idempotência e detecção de mudanças).
CREATE TABLE external_reservations (
    connection_id UUID        NOT NULL REFERENCES connections (id) ON DELETE CASCADE,
    external_id   TEXT        NOT NULL,
    mapping_id    UUID,
    fingerprint   TEXT        NOT NULL,
    cancelled     BOOLEAN     NOT NULL DEFAULT FALSE,
    check_out     DATE,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (connection_id, external_id)
);

-- Histórico de sincronizações, para diagnóstico na tela de canais.
CREATE TABLE sync_log (
    id            UUID        PRIMARY KEY,
    connection_id UUID        NOT NULL REFERENCES connections (id) ON DELETE CASCADE,
    direction     TEXT        NOT NULL,
    operation     TEXT        NOT NULL,
    success       BOOLEAN     NOT NULL,
    detail        TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX sync_log_connection_idx ON sync_log (connection_id, created_at DESC);

CREATE TABLE outbox (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id      UUID        NOT NULL UNIQUE,
    event_type    TEXT        NOT NULL,
    partition_key TEXT        NOT NULL,
    payload       JSONB       NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);
CREATE INDEX outbox_pending_idx ON outbox (id) WHERE published_at IS NULL;

-- +goose Down
DROP TABLE outbox;
DROP TABLE sync_log;
DROP TABLE external_reservations;
DROP TABLE availability_snapshot;
DROP TABLE mappings;
DROP TABLE connections;
