-- +goose Up

-- Folio: a conta corrente de uma reserva.
CREATE TABLE folios (
    id                 UUID        PRIMARY KEY,
    reservation_id     UUID        NOT NULL UNIQUE,
    reservation_code   TEXT        NOT NULL,
    property_id        UUID        NOT NULL,
    guest_name         TEXT        NOT NULL,
    source             TEXT        NOT NULL,
    status             TEXT        NOT NULL,
    currency           CHAR(3)     NOT NULL,
    deposit_due_cents  BIGINT      NOT NULL DEFAULT 0,
    deposit_settled_at TIMESTAMPTZ,
    check_in           DATE        NOT NULL,
    check_out          DATE        NOT NULL,
    opened_at          TIMESTAMPTZ NOT NULL,
    closed_at          TIMESTAMPTZ,
    -- Totais desnormalizados (derivados dos lançamentos) para as listagens.
    total_due_cents    BIGINT      NOT NULL DEFAULT 0,
    paid_cents         BIGINT      NOT NULL DEFAULT 0,
    balance_cents      BIGINT      NOT NULL DEFAULT 0
);
CREATE INDEX folios_property_idx ON folios (property_id, check_in DESC);
CREATE INDEX folios_outstanding_idx ON folios (property_id) WHERE balance_cents > 0;

-- Lançamentos do folio: um livro-razão. Linhas não são editadas nem apagadas;
-- correções são feitas anulando (voided_at) e lançando de novo.
CREATE TABLE folio_entries (
    id            UUID        PRIMARY KEY,
    folio_id      UUID        NOT NULL REFERENCES folios (id),
    property_id   UUID        NOT NULL,
    kind          TEXT        NOT NULL CHECK (kind IN ('CHARGE', 'DISCOUNT', 'PAYMENT', 'REFUND')),
    category      TEXT        NOT NULL DEFAULT '',
    description   TEXT        NOT NULL,
    -- Sempre em centavos e nunca negativo: o sinal vem de `kind`.
    amount_cents  BIGINT      NOT NULL CHECK (amount_cents >= 0),
    method        TEXT        NOT NULL DEFAULT '',
    fee_cents     BIGINT      NOT NULL DEFAULT 0 CHECK (fee_cents >= 0),
    reference     TEXT        NOT NULL DEFAULT '',
    business_date DATE        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL,
    created_by    TEXT        NOT NULL,
    voided_at     TIMESTAMPTZ,
    voided_by     TEXT        NOT NULL DEFAULT '',
    void_reason   TEXT        NOT NULL DEFAULT ''
);
CREATE INDEX folio_entries_folio_idx ON folio_entries (folio_id);
CREATE INDEX folio_entries_report_idx ON folio_entries (property_id, business_date) WHERE voided_at IS NULL;

-- Taxa por forma de pagamento (maquininha, comissão de canal), em pontos-base.
CREATE TABLE fee_rules (
    property_id UUID NOT NULL,
    method      TEXT NOT NULL,
    bps         INT  NOT NULL CHECK (bps BETWEEN 0 AND 5000),
    PRIMARY KEY (property_id, method)
);

CREATE TABLE expenses (
    id           UUID        PRIMARY KEY,
    property_id  UUID        NOT NULL,
    category     TEXT        NOT NULL,
    description  TEXT        NOT NULL,
    supplier     TEXT        NOT NULL DEFAULT '',
    amount_cents BIGINT      NOT NULL CHECK (amount_cents > 0),
    due_date     DATE        NOT NULL,
    status       TEXT        NOT NULL CHECK (status IN ('PENDING', 'PAID')),
    paid_on      DATE,
    method       TEXT        NOT NULL DEFAULT '',
    created_by   TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX expenses_property_idx ON expenses (property_id, due_date);

-- Movimentos do caixa em espécie (dinheiro físico).
CREATE TABLE cash_movements (
    id           UUID        PRIMARY KEY,
    property_id  UUID        NOT NULL,
    kind         TEXT        NOT NULL CHECK (kind IN ('IN', 'OUT')),
    amount_cents BIGINT      NOT NULL CHECK (amount_cents > 0),
    reason       TEXT        NOT NULL,
    ref_type     TEXT        NOT NULL DEFAULT '',
    ref_id       TEXT        NOT NULL DEFAULT '',
    occurred_on  DATE        NOT NULL,
    created_by   TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX cash_movements_property_idx ON cash_movements (property_id, occurred_on);

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
DROP TABLE cash_movements;
DROP TABLE expenses;
DROP TABLE fee_rules;
DROP TABLE folio_entries;
DROP TABLE folios;
