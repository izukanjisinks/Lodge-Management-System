-- Resident meal collection: recurring buffet schedule templates. A session is not
-- a dated instance but a repeating rule (e.g. "Breakfast Buffet, 06:00-09:00, daily").
CREATE TABLE IF NOT EXISTS meal_sessions (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id               UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    branch_id            UUID REFERENCES branches(id) ON DELETE SET NULL,
    meal_period          VARCHAR(20)  NOT NULL,          -- breakfast|brunch|lunch|dinner|supper
    buffet_menu_item_id  UUID NOT NULL REFERENCES menu_items(id) ON DELETE RESTRICT,
    start_time           TIME NOT NULL,
    end_time             TIME NOT NULL,
    days_of_week         TEXT[] NOT NULL DEFAULT '{}',   -- ['mon','tue',...]
    auto_open_close      BOOLEAN NOT NULL DEFAULT FALSE,
    status               VARCHAR(20)  NOT NULL DEFAULT 'scheduled', -- scheduled|open|closed|cancelled
    grace_period_minutes INT NOT NULL DEFAULT 15,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_meal_sessions_org    ON meal_sessions (org_id);
CREATE INDEX IF NOT EXISTS idx_meal_sessions_status ON meal_sessions (org_id, status);
