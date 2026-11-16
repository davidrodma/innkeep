-- +goose Up
-- btree_gist permite combinar igualdade (=) e sobreposição de intervalos (&&)
-- em uma mesma restrição EXCLUDE — veja `allocations_no_overlap` abaixo.
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- Cópias locais do catálogo, alimentadas pelos eventos do serviço de
-- propriedades. Não há chave estrangeira para outro serviço: cada serviço é
-- dono exclusivo do seu banco.
CREATE TABLE room_types (
    id               UUID    PRIMARY KEY,
    property_id      UUID    NOT NULL,
    name             TEXT    NOT NULL,
    max_occupancy    INT     NOT NULL,
    base_price_cents BIGINT  NOT NULL,
    currency         CHAR(3) NOT NULL,
    active           BOOLEAN NOT NULL
);
CREATE INDEX room_types_property_idx ON room_types (property_id);

CREATE TABLE rooms (
    id           UUID    PRIMARY KEY,
    property_id  UUID    NOT NULL,
    room_type_id UUID    NOT NULL,
    name         TEXT    NOT NULL,
    active       BOOLEAN NOT NULL
);
CREATE INDEX rooms_type_idx ON rooms (room_type_id);

-- Ocupações: reservas e bloqueios.
CREATE TABLE allocations (
    id             UUID        PRIMARY KEY,
    property_id    UUID        NOT NULL,
    room_type_id   UUID        NOT NULL,
    -- NULL = overbooking: reserva aceita sem quarto livre (vinda de canal).
    room_id        UUID,
    -- NULL para bloqueios. UNIQUE: no máximo uma alocação por reserva.
    reservation_id UUID        UNIQUE,
    kind           TEXT        NOT NULL CHECK (kind IN ('RESERVATION', 'BLOCK')),
    check_in       DATE        NOT NULL,
    check_out      DATE        NOT NULL,
    reason         TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (check_out > check_in),

    -- A regra de ouro do sistema, garantida pelo banco: o MESMO quarto não
    -- pode ter duas ocupações com períodos sobrepostos. O intervalo é
    -- semiaberto '[)': quem sai dia 12 não conflita com quem entra dia 12.
    CONSTRAINT allocations_no_overlap EXCLUDE USING gist (
        room_id WITH =,
        (daterange(check_in, check_out, '[)')) WITH &&
    ) WHERE (room_id IS NOT NULL)
);
CREATE INDEX allocations_property_dates_idx ON allocations (property_id, check_in, check_out);

-- Tarifas e restrições por data. Ausência de linha = diária padrão do tipo,
-- estadia mínima de 1 noite e venda aberta.
CREATE TABLE rate_overrides (
    room_type_id UUID    NOT NULL,
    day          DATE    NOT NULL,
    price_cents  BIGINT  CHECK (price_cents > 0),
    min_stay     INT     CHECK (min_stay >= 1),
    closed       BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (room_type_id, day)
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
DROP TABLE rate_overrides;
DROP TABLE allocations;
DROP TABLE rooms;
DROP TABLE room_types;
