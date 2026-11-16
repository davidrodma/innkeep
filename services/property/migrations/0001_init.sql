-- +goose Up
CREATE TABLE properties (
    id                  UUID        PRIMARY KEY,
    slug                TEXT        NOT NULL UNIQUE,
    name                TEXT        NOT NULL,
    legal_name          TEXT        NOT NULL DEFAULT '',
    tax_id              TEXT        NOT NULL DEFAULT '',
    cadastur            TEXT        NOT NULL DEFAULT '',
    kind                TEXT        NOT NULL,
    description         TEXT        NOT NULL DEFAULT '',
    email               TEXT        NOT NULL DEFAULT '',
    phone               TEXT        NOT NULL DEFAULT '',
    address_line        TEXT        NOT NULL DEFAULT '',
    address_district    TEXT        NOT NULL DEFAULT '',
    address_city        TEXT        NOT NULL DEFAULT '',
    address_state       TEXT        NOT NULL DEFAULT '',
    address_postal_code TEXT        NOT NULL DEFAULT '',
    address_country     TEXT        NOT NULL DEFAULT 'BR',
    timezone            TEXT        NOT NULL,
    currency            CHAR(3)     NOT NULL,
    check_in_time       TEXT        NOT NULL,
    check_out_time      TEXT        NOT NULL,
    deposit_percent     INT         NOT NULL CHECK (deposit_percent BETWEEN 0 AND 100),
    hold_minutes        INT         NOT NULL CHECK (hold_minutes >= 0),
    cancellation_policy TEXT        NOT NULL DEFAULT '',
    pix_key             TEXT        NOT NULL DEFAULT '',
    published           BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL
);

CREATE TABLE room_types (
    id                UUID        PRIMARY KEY,
    property_id       UUID        NOT NULL REFERENCES properties (id),
    name              TEXT        NOT NULL,
    description       TEXT        NOT NULL DEFAULT '',
    kind              TEXT        NOT NULL,
    max_occupancy     INT         NOT NULL CHECK (max_occupancy > 0),
    bed_configuration TEXT        NOT NULL DEFAULT '',
    -- Dinheiro sempre em centavos (inteiro), nunca em ponto flutuante.
    base_price_cents  BIGINT      NOT NULL CHECK (base_price_cents > 0),
    amenities         TEXT[]      NOT NULL DEFAULT '{}',
    photo_urls        TEXT[]      NOT NULL DEFAULT '{}',
    active            BOOLEAN     NOT NULL DEFAULT TRUE,
    sort_order        INT         NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL
);
CREATE INDEX room_types_property_idx ON room_types (property_id);

CREATE TABLE rooms (
    id           UUID        PRIMARY KEY,
    property_id  UUID        NOT NULL REFERENCES properties (id),
    room_type_id UUID        NOT NULL REFERENCES room_types (id),
    name         TEXT        NOT NULL,
    floor        TEXT        NOT NULL DEFAULT '',
    notes        TEXT        NOT NULL DEFAULT '',
    active       BOOLEAN     NOT NULL DEFAULT TRUE,
    housekeeping TEXT        NOT NULL DEFAULT 'CLEAN',
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    UNIQUE (property_id, name)
);

-- Transactional Outbox: eventos aguardando publicação no Kafka.
-- Veja pkg/platform/outbox para a explicação do padrão.
CREATE TABLE outbox (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id      UUID        NOT NULL UNIQUE,
    event_type    TEXT        NOT NULL,
    partition_key TEXT        NOT NULL,
    payload       JSONB       NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);
-- Índice parcial: só contém as linhas pendentes, então fica minúsculo.
CREATE INDEX outbox_pending_idx ON outbox (id) WHERE published_at IS NULL;

-- Inbox: eventos já processados (idempotência). Veja pkg/platform/inbox.
CREATE TABLE inbox (
    consumer     TEXT        NOT NULL,
    event_id     UUID        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);

-- +goose Down
DROP TABLE inbox;
DROP TABLE outbox;
DROP TABLE rooms;
DROP TABLE room_types;
DROP TABLE properties;
