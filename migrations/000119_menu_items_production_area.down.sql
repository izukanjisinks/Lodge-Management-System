DROP INDEX IF EXISTS idx_menu_items_production_area;

ALTER TABLE menu_items DROP COLUMN IF EXISTS production_area;
