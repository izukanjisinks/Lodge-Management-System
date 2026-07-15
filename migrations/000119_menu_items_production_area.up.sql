-- Add production_area to menu_items: which station prepares this item.
-- Drives kitchen/bar order routing (see order_repository has_kitchen_items/has_bar_items).
-- Nullable so existing items are unaffected; routing falls back to category when NULL.

ALTER TABLE menu_items
    ADD COLUMN production_area VARCHAR(20)
    CHECK (production_area IN ('kitchen', 'bakery', 'bar', 'grill'));

CREATE INDEX IF NOT EXISTS idx_menu_items_production_area ON menu_items(production_area);
