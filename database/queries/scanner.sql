-- name: ListScannableEventsForUser :many
-- Returns the set of events (across all of the user's organizations) for which
-- the user holds a role granting either `racepack.execute` or `checkin.execute`.
-- These are the staff member's Permitted_Events for the Scanner PWA.
SELECT DISTINCT e.id, e.organization_id, e.name, e.status
FROM events e
JOIN organization_members om ON om.organization_id = e.organization_id
JOIN member_roles mr ON mr.organization_member_id = om.id
JOIN role_permissions rp ON rp.role_id = mr.role_id
JOIN permissions p ON p.id = rp.permission_id
WHERE om.user_id = $1
  AND p.key IN ('racepack.execute', 'checkin.execute')
ORDER BY e.name;

-- name: UserCanScanEvent :one
-- Reports whether the user holds `racepack.execute` or `checkin.execute` in the
-- organization that owns the given event. Used as the per-operation
-- authorization guard (defense-in-depth behind the route RBAC middleware).
SELECT EXISTS (
    SELECT 1
    FROM events e
    JOIN organization_members om ON om.organization_id = e.organization_id
    JOIN member_roles mr ON mr.organization_member_id = om.id
    JOIN role_permissions rp ON rp.role_id = mr.role_id
    JOIN permissions p ON p.id = rp.permission_id
    WHERE e.id = $1
      AND om.user_id = $2
      AND p.key IN ('racepack.execute', 'checkin.execute')
) AS can_scan;
