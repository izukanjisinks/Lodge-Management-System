-- Reverse the staff role restructure: restore cleaner/guest, remove F&B roles.
-- Roles are org-scoped (uq_roles_name_org), so operate per distinct org scope.

INSERT INTO roles (org_id, name, description)
SELECT s.org_id, v.name, v.description
FROM (SELECT DISTINCT org_id FROM roles) s
CROSS JOIN (VALUES
    ('cleaner', 'Housekeeping staff — views assigned rooms and cleaning schedule'),
    ('guest',   'Guest user — limited access to view available rooms and make bookings via website')
) AS v(name, description)
WHERE NOT EXISTS (
    SELECT 1 FROM roles r
    WHERE r.name = v.name
      AND r.org_id IS NOT DISTINCT FROM s.org_id
);

DELETE FROM roles WHERE name IN ('kitchen_staff', 'waiter', 'bar_staff');
