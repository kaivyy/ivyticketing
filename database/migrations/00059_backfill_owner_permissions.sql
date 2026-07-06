-- +goose Up
-- Phase 22 — Security Hardening: back-fill the system Owner role.
--
-- Migration 00007 grants the Owner role EVERY permission via a CROSS JOIN, but
-- that snapshot only covered permissions that existed at the time. Later
-- migrations (ballot, check-in, racepack.execute/problemdesk) added new
-- permissions and granted them to manager/staff roles but never back-filled
-- Owner. Result: org owners silently lack ballot.apply, ballot.manage,
-- checkin.execute, racepack.execute, and racepack.problemdesk — even though
-- Owner is meant to be a superset of all capabilities.
--
-- Org creation snapshots the global Owner template into each org (copies its
-- role_permissions), so every org created since those migrations inherited the
-- gap. This re-runs the CROSS JOIN to make the global Owner template complete
-- again. Idempotent via ON CONFLICT.
--
-- NOTE: this repairs the global template for orgs created AFTER this migration.
-- Existing per-org owner roles are repaired by the second statement below.

-- 1. Repair the global Owner template (organization_id IS NULL).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE r.organization_id IS NULL AND r.slug = 'owner'
ON CONFLICT DO NOTHING;

-- 2. Repair every existing per-org Owner role so already-created orgs get the
-- missing permissions too.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE r.organization_id IS NOT NULL AND r.slug = 'owner'
ON CONFLICT DO NOTHING;

-- +goose Down
-- No-op: cannot distinguish back-filled grants from original CROSS JOIN grants,
-- and removing owner permissions would break authorization. Down is intentionally
-- empty (the Owner-has-all-permissions invariant is not something we roll back).
SELECT 1;
