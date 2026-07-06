-- Buffet packages are stored as menu_items with category = 'buffet'. The
-- structured buffet payload (buffet_type, min_covers, dishes) lives in this
-- JSONB column so the free-text description column can hold real prose.
ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS buffet_data JSONB;

-- Only buffet items may carry buffet_data; everything else must leave it NULL.
ALTER TABLE menu_items ADD CONSTRAINT buffet_data_only_for_buffet
    CHECK (category = 'buffet' OR buffet_data IS NULL);

-- Support filtering/lookups by buffet_type and other keys.
CREATE INDEX IF NOT EXISTS idx_menu_items_buffet_data
    ON menu_items USING GIN (buffet_data);
