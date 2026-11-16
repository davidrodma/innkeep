-- +goose Up

-- EVENT STORE: a fonte da verdade do serviço de reservas.
-- Tabela append-only: linhas são inseridas, nunca alteradas nem apagadas.
CREATE TABLE events (
    -- Posição no log global (ordem total de todos os eventos).
    global_seq     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id       UUID        NOT NULL UNIQUE,
    aggregate_type TEXT        NOT NULL,
    aggregate_id   UUID        NOT NULL,
    -- Posição no fluxo do agregado (1, 2, 3...).
    version        INT         NOT NULL,
    event_type     TEXT        NOT NULL,
    payload        JSONB       NOT NULL,
    metadata       JSONB       NOT NULL DEFAULT '{}',
    occurred_at    TIMESTAMPTZ NOT NULL,
    -- Concorrência otimista: dois comandos não gravam a mesma versão.
    UNIQUE (aggregate_id, version)
);

-- MODELO DE LEITURA (CQRS): projeção desnormalizada dos eventos, feita para
-- as telas. Pode ser reconstruída do zero com `reservation rebuild-views`.
CREATE TABLE reservation_views (
    id                     UUID        PRIMARY KEY,
    code                   TEXT        NOT NULL UNIQUE,
    property_id            UUID        NOT NULL,
    room_type_id           UUID        NOT NULL,
    room_id                UUID,
    room_name              TEXT        NOT NULL DEFAULT '',
    check_in               DATE        NOT NULL,
    check_out              DATE        NOT NULL,
    adults                 INT         NOT NULL,
    children               INT         NOT NULL,
    holder_name            TEXT        NOT NULL,
    holder_email           TEXT        NOT NULL DEFAULT '',
    holder_phone           TEXT        NOT NULL DEFAULT '',
    source                 TEXT        NOT NULL,
    external_ref           TEXT        NOT NULL DEFAULT '',
    group_code             TEXT        NOT NULL DEFAULT '',
    notes                  TEXT        NOT NULL DEFAULT '',
    status                 TEXT        NOT NULL,
    nightly_rates          JSONB       NOT NULL DEFAULT '[]',
    total_cents            BIGINT      NOT NULL DEFAULT 0,
    deposit_due_cents      BIGINT      NOT NULL DEFAULT 0,
    currency               CHAR(3)     NOT NULL,
    hold_expires_at        TIMESTAMPTZ,
    payment_collect        TEXT        NOT NULL DEFAULT 'PROPERTY',
    overbooked             BOOLEAN     NOT NULL DEFAULT FALSE,
    registration_completed BOOLEAN     NOT NULL DEFAULT FALSE,
    primary_guest_id       UUID,
    pending_change         JSONB,
    cancel_reason          TEXT        NOT NULL DEFAULT '',
    reject_reason          TEXT        NOT NULL DEFAULT '',
    reject_detail          TEXT        NOT NULL DEFAULT '',
    requested_at           TIMESTAMPTZ NOT NULL,
    confirmed_at           TIMESTAMPTZ,
    checked_in_at          TIMESTAMPTZ,
    checked_out_at         TIMESTAMPTZ,
    closed_at              TIMESTAMPTZ,
    version                INT         NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Índices pensados para as consultas das telas (calendário, listagem, painel).
CREATE INDEX reservation_views_stay_idx ON reservation_views (property_id, check_in, check_out);
CREATE INDEX reservation_views_status_idx ON reservation_views (property_id, status);
-- Índices parciais: cobrem só as linhas que as tarefas periódicas procuram.
CREATE INDEX reservation_views_hold_idx ON reservation_views (hold_expires_at) WHERE status = 'PENDING';
CREATE INDEX reservation_views_requested_idx ON reservation_views (requested_at) WHERE status = 'REQUESTED';

-- Cópia local das políticas das propriedades (eventos de property).
CREATE TABLE property_policies (
    property_id     UUID    PRIMARY KEY,
    name            TEXT    NOT NULL,
    timezone        TEXT    NOT NULL,
    currency        CHAR(3) NOT NULL,
    deposit_percent INT     NOT NULL,
    hold_minutes    INT     NOT NULL,
    published       BOOLEAN NOT NULL
);

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

CREATE TABLE inbox (
    consumer     TEXT        NOT NULL,
    event_id     UUID        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);

-- +goose Down
DROP TABLE inbox;
DROP TABLE outbox;
DROP TABLE property_policies;
DROP TABLE reservation_views;
DROP TABLE events;
