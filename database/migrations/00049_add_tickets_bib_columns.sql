-- +goose Up
ALTER TABLE tickets
    ADD COLUMN bib_number              text,
    ADD COLUMN bib_assigned_at         timestamptz,
    ADD COLUMN bib_assigned_by         uuid REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN bib_assignment_method   text
        CHECK (bib_assignment_method IN ('AUTO','MANUAL','OVERRIDE'));

-- Partial unique index: enforces uniqueness of (event_id, bib_number) only when bib_number is set.
-- This lets multiple tickets within an event remain unassigned (NULL) while preventing duplicates once assigned.
CREATE UNIQUE INDEX uniq_tickets_event_bib
    ON tickets (event_id, bib_number)
    WHERE bib_number IS NOT NULL;

-- Lookup helper: list unassigned tickets for an event.
CREATE INDEX idx_tickets_bib_event_null
    ON tickets (event_id)
    WHERE bib_number IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_tickets_bib_event_null;
DROP INDEX IF EXISTS uniq_tickets_event_bib;
ALTER TABLE tickets
    DROP COLUMN IF EXISTS bib_assignment_method,
    DROP COLUMN IF EXISTS bib_assigned_by,
    DROP COLUMN IF EXISTS bib_assigned_at,
    DROP COLUMN IF EXISTS bib_number;