-- +goose Up
-- Phase 14 — Racepack Pickup System

CREATE TABLE racepack_counters (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name            text NOT NULL,
    location        text,
    active          boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT racepack_counters_name_per_event UNIQUE (event_id, name)
);
CREATE INDEX idx_racepack_counters_event ON racepack_counters(event_id);
CREATE INDEX idx_racepack_counters_event_active ON racepack_counters(event_id) WHERE active = true;

CREATE TABLE racepack_pickup_slots (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name            text NOT NULL,
    pickup_date     date NOT NULL,
    start_time      timestamptz NOT NULL,
    end_time        timestamptz NOT NULL,
    capacity        integer NOT NULL CHECK (capacity > 0),
    reserved_count  integer NOT NULL DEFAULT 0 CHECK (reserved_count >= 0),
    active          boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT racepack_pickup_slots_window CHECK (end_time > start_time),
    CONSTRAINT racepack_pickup_slots_capacity CHECK (reserved_count <= capacity)
);
CREATE INDEX idx_racepack_pickup_slots_event ON racepack_pickup_slots(event_id);
CREATE INDEX idx_racepack_pickup_slots_event_date ON racepack_pickup_slots(event_id, pickup_date);
CREATE INDEX idx_racepack_pickup_slots_event_active ON racepack_pickup_slots(event_id, pickup_date) WHERE active = true;

CREATE TABLE racepack_pickup_records (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id          uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    ticket_id         uuid NOT NULL REFERENCES tickets(id) ON DELETE RESTRICT,
    participant_id    uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    bib_number        text NOT NULL,
    counter_id        uuid NOT NULL REFERENCES racepack_counters(id) ON DELETE RESTRICT,
    staff_id          uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    pickup_method     text NOT NULL CHECK (pickup_method IN ('SELF','PROXY','MANUAL_OVERRIDE')),
    pickup_timestamp  timestamptz NOT NULL DEFAULT now(),
    notes             text,
    status            text NOT NULL DEFAULT 'PICKED_UP'
                      CHECK (status IN ('PICKED_UP','CANCELLED'))
);

-- Anti-duplicate guard: at most one PICKED_UP record per ticket.
CREATE UNIQUE INDEX uniq_racepack_pickup_records_ticket_active
    ON racepack_pickup_records(ticket_id)
    WHERE status = 'PICKED_UP';

CREATE INDEX idx_racepack_pickup_records_event ON racepack_pickup_records(event_id);
CREATE INDEX idx_racepack_pickup_records_counter ON racepack_pickup_records(counter_id);
CREATE INDEX idx_racepack_pickup_records_staff ON racepack_pickup_records(staff_id);
CREATE INDEX idx_racepack_pickup_records_participant ON racepack_pickup_records(participant_id);
CREATE INDEX idx_racepack_pickup_records_event_timestamp ON racepack_pickup_records(event_id, pickup_timestamp);

CREATE TABLE racepack_proxy_authorizations (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id              uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    ticket_id             uuid NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    pickup_record_id      uuid REFERENCES racepack_pickup_records(id) ON DELETE SET NULL,
    proxy_name            text NOT NULL,
    proxy_phone           text,
    proxy_identity        text NOT NULL,
    authorization_document text,
    created_by            uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_racepack_proxy_authorizations_event ON racepack_proxy_authorizations(event_id);
CREATE INDEX idx_racepack_proxy_authorizations_ticket ON racepack_proxy_authorizations(ticket_id);
CREATE INDEX idx_racepack_proxy_authorizations_pickup ON racepack_proxy_authorizations(pickup_record_id);

CREATE TABLE racepack_problem_cases (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    ticket_id       uuid REFERENCES tickets(id) ON DELETE SET NULL,
    participant_id  uuid REFERENCES users(id) ON DELETE SET NULL,
    status          text NOT NULL DEFAULT 'OPEN'
                    CHECK (status IN ('OPEN','UNDER_REVIEW','RESOLVED','ESCALATED')),
    reason          text NOT NULL,
    resolution      text,
    created_by      uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    resolved_by     uuid REFERENCES users(id) ON DELETE SET NULL,
    resolved_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_racepack_problem_cases_event ON racepack_problem_cases(event_id);
CREATE INDEX idx_racepack_problem_cases_event_status ON racepack_problem_cases(event_id, status);
CREATE INDEX idx_racepack_problem_cases_ticket ON racepack_problem_cases(ticket_id);

-- +goose Down
DROP INDEX IF EXISTS idx_racepack_problem_cases_ticket;
DROP INDEX IF EXISTS idx_racepack_problem_cases_event_status;
DROP INDEX IF EXISTS idx_racepack_problem_cases_event;
DROP TABLE IF EXISTS racepack_problem_cases;

DROP INDEX IF EXISTS idx_racepack_proxy_authorizations_pickup;
DROP INDEX IF EXISTS idx_racepack_proxy_authorizations_ticket;
DROP INDEX IF EXISTS idx_racepack_proxy_authorizations_event;
DROP TABLE IF EXISTS racepack_proxy_authorizations;

DROP INDEX IF EXISTS idx_racepack_pickup_records_event_timestamp;
DROP INDEX IF EXISTS idx_racepack_pickup_records_participant;
DROP INDEX IF EXISTS idx_racepack_pickup_records_staff;
DROP INDEX IF EXISTS idx_racepack_pickup_records_counter;
DROP INDEX IF EXISTS idx_racepack_pickup_records_event;
DROP INDEX IF EXISTS uniq_racepack_pickup_records_ticket_active;
DROP TABLE IF EXISTS racepack_pickup_records;

DROP INDEX IF EXISTS idx_racepack_pickup_slots_event_active;
DROP INDEX IF EXISTS idx_racepack_pickup_slots_event_date;
DROP INDEX IF EXISTS idx_racepack_pickup_slots_event;
DROP TABLE IF EXISTS racepack_pickup_slots;

DROP INDEX IF EXISTS idx_racepack_counters_event_active;
DROP INDEX IF EXISTS idx_racepack_counters_event;
DROP TABLE IF EXISTS racepack_counters;