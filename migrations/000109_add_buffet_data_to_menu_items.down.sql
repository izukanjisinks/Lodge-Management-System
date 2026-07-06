DROP INDEX IF EXISTS idx_menu_items_buffet_data;
ALTER TABLE menu_items DROP CONSTRAINT IF EXISTS buffet_data_only_for_buffet;
ALTER TABLE menu_items DROP COLUMN IF EXISTS buffet_data;
