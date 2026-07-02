-- Actual check-in/out timestamps on room assignments.
-- check_in / check_out remain the BOOKED dates (drive availability + the booked
-- invoice). checked_in_at / checked_out_at record when staff actually pressed the
-- check-in / check-out buttons, so the invoice can be regenerated from actual nights.
ALTER TABLE booking_room_assignments
    ADD COLUMN IF NOT EXISTS checked_in_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS checked_out_at TIMESTAMPTZ;

-- Positive type marker on invoice line items so regeneration can target exactly the
-- room lines (delete WHERE line_type = 'room') without inferring from NULL order ids.
-- Existing rows: order lines carry an order_item_id, everything else is a room/event
-- line — default to 'room' and re-tag order lines as 'meal'.
ALTER TABLE invoice_line_items
    ADD COLUMN IF NOT EXISTS line_type VARCHAR(20) NOT NULL DEFAULT 'room';

UPDATE invoice_line_items SET line_type = 'meal' WHERE order_item_id IS NOT NULL OR order_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_line_items_line_type ON invoice_line_items(invoice_id, line_type);
