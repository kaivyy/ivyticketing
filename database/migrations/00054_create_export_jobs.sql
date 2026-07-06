-- +goose Up
-- Phase 16 — Reporting & Export: async export jobs + report.export permission.

CREATE TABLE export_jobs (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid REFERENCES events(id) ON DELETE CASCADE,
    requested_by    uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    report_type     text NOT NULL,
    format          text NOT NULL DEFAULT 'csv',
    params          jsonb NOT NULL DEFAULT '{}',
    status          text NOT NULL DEFAULT 'PENDING',
    row_count       integer,
    file_key        text,
    file_url        text,
    error           text,
    started_at      timestamptz,
    completed_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT export_jobs_report_type_check CHECK (report_type IN
        ('participant','sales','payment','coupon','queue','ballot','racepack','revenue')),
    CONSTRAINT export_jobs_format_check CHECK (format IN ('csv')),
    CONSTRAINT export_jobs_status_check CHECK (status IN ('PENDING','PROCESSING','READY','FAILED'))
);
CREATE INDEX idx_export_jobs_org ON export_jobs(organization_id, created_at DESC);
CREATE INDEX idx_export_jobs_event ON export_jobs(event_id);
CREATE INDEX idx_export_jobs_pending ON export_jobs(status, created_at) WHERE status = 'PENDING';

INSERT INTO permissions (key, description) VALUES
    ('report.view', 'View report summaries and export history'),
    ('report.export', 'Request and download data exports')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NULL
  AND r.slug IN ('owner', 'manager', 'finance', 'customer-service')
  AND p.key = 'report.view'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NULL
  AND r.slug IN ('owner', 'manager', 'finance')
  AND p.key = 'report.export'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE permission_id IN
    (SELECT id FROM permissions WHERE key IN ('report.view', 'report.export'));
DELETE FROM permissions WHERE key IN ('report.view', 'report.export');
DROP INDEX IF EXISTS idx_export_jobs_pending;
DROP INDEX IF EXISTS idx_export_jobs_event;
DROP INDEX IF EXISTS idx_export_jobs_org;
DROP TABLE IF EXISTS export_jobs;
