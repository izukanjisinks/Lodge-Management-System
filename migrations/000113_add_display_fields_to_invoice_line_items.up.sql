-- Structured display context for meal invoice lines so the frontend can render
-- grouped-by-diner layouts and buffet package lines without parsing the description.
--   attendee_name  — the diner this line belongs to (individual orders); NULL for buffet/headcount
--   pax_count      — cover count, for buffet "for N guests" context; NULL for individual lines
--   service_type   — buffet | individual_order | set_menu | a_la_carte | mixed
ALTER TABLE invoice_line_items ADD COLUMN IF NOT EXISTS attendee_name VARCHAR(255);
ALTER TABLE invoice_line_items ADD COLUMN IF NOT EXISTS pax_count INT;
ALTER TABLE invoice_line_items ADD COLUMN IF NOT EXISTS service_type VARCHAR(30);
