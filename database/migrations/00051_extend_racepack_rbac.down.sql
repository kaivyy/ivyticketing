DELETE FROM role_permissions WHERE permission_id IN
    (SELECT id FROM permissions WHERE key IN ('racepack.execute','racepack.problemdesk'));
DELETE FROM permissions WHERE key IN ('racepack.execute','racepack.problemdesk');