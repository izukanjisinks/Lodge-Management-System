-- Reverse the staff role restructure: restore cleaner/guest, remove F&B roles.

INSERT INTO roles (name, description) VALUES
    ('cleaner', 'Housekeeping staff — views assigned rooms and cleaning schedule'),
    ('guest',   'Guest user — limited access to view available rooms and make bookings via website')
ON CONFLICT (name) DO NOTHING;

DELETE FROM roles WHERE name IN ('kitchen_staff', 'waiter', 'bar_staff');
