ALTER TABLE order_items
    ADD COLUMN IF NOT EXISTS attendee_id UUID REFERENCES booking_attendees(id) ON DELETE SET NULL;

CREATE INDEX idx_order_items_attendee_id ON order_items(attendee_id) WHERE attendee_id IS NOT NULL;
