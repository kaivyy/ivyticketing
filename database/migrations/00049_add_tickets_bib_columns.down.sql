DROP INDEX IF EXISTS idx_tickets_bib_event_null;
DROP INDEX IF EXISTS uniq_tickets_event_bib;
ALTER TABLE tickets
    DROP COLUMN IF EXISTS bib_assignment_method,
    DROP COLUMN IF EXISTS bib_assigned_by,
    DROP COLUMN IF EXISTS bib_assigned_at,
    DROP COLUMN IF EXISTS bib_number;