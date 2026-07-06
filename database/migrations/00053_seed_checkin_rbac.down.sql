DELETE FROM role_permissions WHERE permission_id IN
    (SELECT id FROM permissions WHERE key = 'checkin.execute');
DELETE FROM permissions WHERE key = 'checkin.execute';
