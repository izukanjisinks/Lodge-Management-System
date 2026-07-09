-- service_type records how a meal session order is served (buffet, individual_order,
-- set_menu, a_la_carte, mixed). It drives how the invoice groups and labels the line.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS service_type VARCHAR(30);
