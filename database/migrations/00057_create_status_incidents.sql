-- +goose Up
-- Phase 19 — Public Status & Incident System: platform-wide component status
-- (queue / payment / registration) and an incident timeline. Read publicly on
-- the status page; managed by super-admins. Status is global, not per-org.

CREATE TABLE status_components (
    key         text PRIMARY KEY,                 -- 'queue' | 'payment' | 'registration'
    name        text NOT NULL,
    status      text NOT NULL DEFAULT 'OPERATIONAL',
    sort_order  int  NOT NULL DEFAULT 0,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT status_components_status_check CHECK (status IN ('OPERATIONAL','DEGRADED','DOWN'))
);

INSERT INTO status_components (key, name, status, sort_order) VALUES
    ('queue',        'Antrian Virtual',  'OPERATIONAL', 1),
    ('payment',      'Pembayaran',       'OPERATIONAL', 2),
    ('registration', 'Registrasi',       'OPERATIONAL', 3);

CREATE TABLE incidents (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title       text NOT NULL,
    impact      text NOT NULL DEFAULT 'MINOR',
    status      text NOT NULL DEFAULT 'INVESTIGATING',
    started_at  timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT incidents_impact_check CHECK (impact IN ('NONE','MINOR','MAJOR','CRITICAL')),
    CONSTRAINT incidents_status_check CHECK (status IN ('INVESTIGATING','IDENTIFIED','MONITORING','RESOLVED'))
);
CREATE INDEX idx_incidents_active ON incidents(started_at DESC) WHERE resolved_at IS NULL;
CREATE INDEX idx_incidents_created ON incidents(created_at DESC);

CREATE TABLE incident_updates (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id uuid NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    status      text NOT NULL,
    body        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT incident_updates_status_check CHECK (status IN ('INVESTIGATING','IDENTIFIED','MONITORING','RESOLVED'))
);
CREATE INDEX idx_incident_updates_incident ON incident_updates(incident_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS incident_updates;
DROP TABLE IF EXISTS incidents;
DROP TABLE IF EXISTS status_components;
