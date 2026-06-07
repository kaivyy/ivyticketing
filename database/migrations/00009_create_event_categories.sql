-- +goose Up
CREATE TABLE event_categories (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id              uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name                  text NOT NULL,
    price                 bigint NOT NULL,
    capacity              integer NOT NULL,
    registration_opens_at  timestamptz NOT NULL,
    registration_closes_at timestamptz NOT NULL,
    bib_prefix            text,
    min_age               integer,
    max_order_per_user    integer NOT NULL DEFAULT 1,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT event_categories_price_check CHECK (price >= 0),
    CONSTRAINT event_categories_capacity_check CHECK (capacity > 0),
    CONSTRAINT event_categories_min_age_check CHECK (min_age IS NULL OR min_age >= 0),
    CONSTRAINT event_categories_max_order_check CHECK (max_order_per_user >= 1),
    UNIQUE (event_id, name)
);
CREATE INDEX idx_event_categories_event ON event_categories(event_id);
CREATE INDEX idx_event_categories_org ON event_categories(organization_id);

-- +goose Down
DROP TABLE event_categories;
