-- +goose Up
-- Phase 22 — Security Hardening: make audit_logs append-only at the database
-- level. The application already only ever INSERTs (no UPDATE/DELETE queries
-- exist), but that is a code-level convention. This trigger enforces it in the
-- engine so a compromised or buggy caller — or any role with table privileges —
-- cannot silently rewrite or erase the audit trail.
--
-- INSERT stays allowed; UPDATE and DELETE raise an exception. TRUNCATE is also
-- blocked. Integration-test teardown truncates on a separate test database, so
-- the trigger is guarded to allow disabling via session_replication_role only
-- for superuser maintenance (never set in the app).

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_logs_prevent_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only: % is not permitted', TG_OP
        USING ERRCODE = 'restrict_violation';
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER audit_logs_no_update
    BEFORE UPDATE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_prevent_mutation();

CREATE TRIGGER audit_logs_no_delete
    BEFORE DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_prevent_mutation();

CREATE TRIGGER audit_logs_no_truncate
    BEFORE TRUNCATE ON audit_logs
    EXECUTE FUNCTION audit_logs_prevent_mutation();

-- +goose Down
DROP TRIGGER IF EXISTS audit_logs_no_truncate ON audit_logs;
DROP TRIGGER IF EXISTS audit_logs_no_delete ON audit_logs;
DROP TRIGGER IF EXISTS audit_logs_no_update ON audit_logs;
DROP FUNCTION IF EXISTS audit_logs_prevent_mutation();
