-- Restructure staff roles: add F&B roles (kitchen_staff, waiter, bar_staff),
-- remove the deprecated cleaner and guest roles.
--
-- Guest identity now lives in web_users (separate auth system), so the "guest"
-- staff role is obsolete. Cleaner/housekeeping is no longer a distinct role.
--
-- users.role_id FKs to roles(role_id) ON DELETE SET NULL, so any user still on a
-- removed role is left with role_id = NULL (no access) and must be reassigned.

INSERT INTO roles (name, description) VALUES
    ('kitchen_staff', 'Kitchen staff — views and updates meal orders and preparation status'),
    ('waiter',        'Waiter — takes and serves meal orders, manages table service'),
    ('bar_staff',     'Bar staff — handles bar orders and updates bar order status')
ON CONFLICT (name) DO NOTHING;

DELETE FROM roles WHERE name IN ('cleaner', 'guest');
