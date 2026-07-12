-- Restructure staff roles: add F&B roles (kitchen_staff, waiter, bar_staff),
-- remove the deprecated cleaner and guest roles.
--
-- Roles are org-scoped: uq_roles_name_org UNIQUE (org_id, name). Roles seeded by
-- the app have org_id = NULL (a shared set), but org-scoped rows may also exist,
-- so we add the new roles for EVERY distinct org_id that already has roles.
--
-- users.role_id FKs to roles(role_id) ON DELETE SET NULL, so any user still on a
-- removed role is left with role_id = NULL (no access) and must be reassigned.

-- Add the three F&B roles once per existing org scope (including the NULL scope),
-- skipping any (org_id, name) pair that already exists.
INSERT INTO roles (org_id, name, description)
SELECT s.org_id, v.name, v.description
FROM (SELECT DISTINCT org_id FROM roles) s
CROSS JOIN (VALUES
    ('kitchen_staff', 'Kitchen staff — views and updates meal orders and preparation status'),
    ('waiter',        'Waiter — takes and serves meal orders, manages table service'),
    ('bar_staff',     'Bar staff — handles bar orders and updates bar order status')
) AS v(name, description)
WHERE NOT EXISTS (
    SELECT 1 FROM roles r
    WHERE r.name = v.name
      AND r.org_id IS NOT DISTINCT FROM s.org_id
);

-- Remove the deprecated roles across all org scopes.
DELETE FROM roles WHERE name IN ('cleaner', 'guest');
