DROP INDEX IF EXISTS idx_invoice_line_items_line_type;
ALTER TABLE invoice_line_items DROP COLUMN IF EXISTS line_type;

ALTER TABLE booking_room_assignments
    DROP COLUMN IF EXISTS checked_in_at,
    DROP COLUMN IF EXISTS checked_out_at;
