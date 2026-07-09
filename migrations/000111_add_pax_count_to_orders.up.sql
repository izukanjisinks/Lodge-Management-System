-- pax_count records how many covers a meal session order is for. Buffet sessions
-- are billed as a flat package (order_items.quantity = 1), so the cover count is
-- kept here for display context ("for N guests") rather than as a price multiplier.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS pax_count INT;
