-- +goose Up
CREATE TABLE events (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name              text NOT NULL,
    slug              text NOT NULL,
    description       text,
    event_type        text NOT NULL,
    status            text NOT NULL DEFAULT 'draft',
    banner_object_key text,
    logo_object_key   text,
    venue_name        text,
    venue_address     text,
    starts_at         timestamptz,
    ends_at           timestamptz,
    faq               text,
    terms             text,
    waiver            text,
    published_at      timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT events_status_check CHECK (status IN ('draft', 'published', 'archived')),
    UNIQUE (organization_id, slug)
);
CREATE INDEX idx_events_org ON events(organization_id);
CREATE INDEX idx_events_org_status ON events(organization_id, status);

-- +goose Down
DROP TABLE events;
