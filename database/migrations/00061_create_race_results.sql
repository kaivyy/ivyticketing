-- +goose Up
-- Phase 24 — Result, Certificate & Timing Integration

-- race_results holds one finisher row per (event, bib). Demographics (gender,
-- age_group) live on the row rather than on users because they are recorded at
-- race time from the registration form and drive gender/age-group ranking; the
-- users table carries no gender/DOB. ticket_id is a soft link (SET NULL) so a
-- result survives ticket cleanup, and bib_number is the stable join key used by
-- both CSV import and the timing-vendor API.
CREATE TABLE race_results (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id      uuid REFERENCES event_categories(id) ON DELETE SET NULL,
    ticket_id        uuid REFERENCES tickets(id) ON DELETE SET NULL,
    bib_number       text NOT NULL,
    participant_name text NOT NULL,
    gender           text CHECK (gender IN ('M','F','X')),
    age              integer CHECK (age IS NULL OR (age >= 0 AND age <= 130)),
    age_group        text,
    status           text NOT NULL DEFAULT 'FINISHED'
        CHECK (status IN ('FINISHED','DNF','DNS')),
    -- Times stored as elapsed milliseconds so ranking is a plain integer sort.
    -- chip_time is net (start-mat to finish); gun_time is gross (gun to finish).
    chip_time_ms     bigint CHECK (chip_time_ms IS NULL OR chip_time_ms >= 0),
    gun_time_ms      bigint CHECK (gun_time_ms IS NULL OR gun_time_ms >= 0),
    -- Ranks are computed by the ranking pass (NULL for DNF/DNS or before compute).
    rank_overall     integer,
    rank_gender      integer,
    rank_category    integer,
    rank_age_group   integer,
    source           text NOT NULL DEFAULT 'CSV'
        CHECK (source IN ('CSV','TIMING_API')),
    finished_at      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id, bib_number)
);
CREATE INDEX idx_race_results_event ON race_results(event_id);
CREATE INDEX idx_race_results_category ON race_results(category_id);
CREATE INDEX idx_race_results_ticket ON race_results(ticket_id);
-- Ranking sort helper: finished rows for an event ordered by net time.
CREATE INDEX idx_race_results_event_time
    ON race_results(event_id, chip_time_ms)
    WHERE status = 'FINISHED';

-- certificate_templates holds a per-event, customizable certificate layout. The
-- body is a text template with {{placeholders}} ({{name}}, {{time}}, {{rank}},
-- {{category}}, {{bib}}) rendered at download time. background_url points at an
-- uploaded image the certificate is drawn over. One active template per event
-- is enforced by a partial unique index.
CREATE TABLE certificate_templates (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name             text NOT NULL,
    title            text NOT NULL DEFAULT 'Certificate of Completion',
    subtitle         text NOT NULL DEFAULT '',
    body_template    text NOT NULL DEFAULT '',
    background_url   text,
    is_active        boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_certificate_templates_event ON certificate_templates(event_id);
CREATE UNIQUE INDEX uniq_certificate_templates_active
    ON certificate_templates(event_id)
    WHERE is_active;

-- RBAC: organizer permission to import results and manage certificate templates.
INSERT INTO permissions (key, description) VALUES
    ('results.manage', 'Import race results and manage certificate templates')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NULL
  AND r.slug IN ('owner', 'manager')
  AND p.key = 'results.manage'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE permission_id IN
    (SELECT id FROM permissions WHERE key = 'results.manage');
DELETE FROM permissions WHERE key = 'results.manage';
DROP TABLE certificate_templates;
DROP TABLE race_results;
