-- +goose Up

-- Cadastro de hóspedes: a tabela com os dados pessoais do sistema.
CREATE TABLE guests (
    id                  UUID        PRIMARY KEY,
    full_name           TEXT        NOT NULL,
    email               TEXT        NOT NULL DEFAULT '',
    phone               TEXT        NOT NULL DEFAULT '',
    birth_date          DATE,
    gender              TEXT        NOT NULL DEFAULT '',
    nationality         TEXT        NOT NULL DEFAULT '',
    profession          TEXT        NOT NULL DEFAULT '',
    document_type       TEXT        NOT NULL,
    -- Número do documento cifrado com AES-256-GCM pela aplicação. O banco
    -- nunca vê o valor em texto puro (veja pkg/platform/fieldcrypt).
    document_enc        BYTEA,
    -- Índice cego (HMAC) do documento: permite buscar por igualdade sem
    -- armazenar nem revelar o número.
    document_index      TEXT,
    document_issuer     TEXT        NOT NULL DEFAULT '',
    address_line        TEXT        NOT NULL DEFAULT '',
    address_district    TEXT        NOT NULL DEFAULT '',
    address_city        TEXT        NOT NULL DEFAULT '',
    address_state       TEXT        NOT NULL DEFAULT '',
    address_postal_code TEXT        NOT NULL DEFAULT '',
    address_country     TEXT        NOT NULL DEFAULT '',
    notes               TEXT        NOT NULL DEFAULT '',
    marketing_consent   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    anonymized_at       TIMESTAMPTZ
);
CREATE INDEX guests_document_idx ON guests (document_index) WHERE document_index IS NOT NULL;
CREATE INDEX guests_name_idx ON guests (lower(full_name));

-- Ficha Nacional de Registro de Hóspedes (FNRH): uma por pessoa, por estadia.
CREATE TABLE registration_cards (
    id                  UUID        PRIMARY KEY,
    reservation_id      UUID        NOT NULL,
    property_id         UUID        NOT NULL,
    guest_id            UUID        NOT NULL REFERENCES guests (id),
    is_primary          BOOLEAN     NOT NULL DEFAULT FALSE,
    purpose_of_trip     TEXT        NOT NULL,
    transport           TEXT        NOT NULL,
    origin_city         TEXT        NOT NULL DEFAULT '',
    origin_state        TEXT        NOT NULL DEFAULT '',
    origin_country      TEXT        NOT NULL DEFAULT '',
    destination_city    TEXT        NOT NULL DEFAULT '',
    destination_state   TEXT        NOT NULL DEFAULT '',
    destination_country TEXT        NOT NULL DEFAULT '',
    vehicle_plate       TEXT        NOT NULL DEFAULT '',
    check_in            DATE        NOT NULL,
    check_out           DATE        NOT NULL,
    checked_in_at       TIMESTAMPTZ,
    checked_out_at      TIMESTAMPTZ,
    consent_at          TIMESTAMPTZ NOT NULL,
    created_by          TEXT        NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL
);
CREATE INDEX registration_cards_reservation_idx ON registration_cards (reservation_id);
CREATE INDEX registration_cards_guest_idx ON registration_cards (guest_id);

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
DROP TABLE registration_cards;
DROP TABLE guests;
