-- Audit log: one row per meal collection (a card scan or typed-ID match that posted
-- a buffet charge to a room's accommodation invoice). idempotency_key makes /collect
-- and the offline /sync batch naturally dedupe accidental double-taps.
CREATE TABLE IF NOT EXISTS meal_collections (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    meal_session_id       UUID NOT NULL REFERENCES meal_sessions(id) ON DELETE CASCADE,
    booking_id            UUID REFERENCES bookings(id) ON DELETE SET NULL,
    attendee_id           UUID REFERENCES booking_attendees(id) ON DELETE SET NULL,
    resident_name         VARCHAR(255),
    identification_card   VARCHAR(128),
    method                VARCHAR(10) NOT NULL,          -- card|typed
    card_uid              VARCHAR(128),
    room_name             VARCHAR(255),
    amount                NUMERIC(12,2),
    invoice_id            UUID REFERENCES invoices(id) ON DELETE SET NULL,
    invoice_line_item_id  UUID,
    idempotency_key       VARCHAR(128) NOT NULL,
    collected_by          UUID REFERENCES users(user_id) ON DELETE SET NULL,
    collected_by_name     VARCHAR(255),
    collected_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    synced_at             TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One collection per idempotency key (per org) — the dedupe guard.
CREATE UNIQUE INDEX IF NOT EXISTS uq_meal_collections_idem
    ON meal_collections (org_id, idempotency_key);

CREATE INDEX IF NOT EXISTS idx_meal_collections_session ON meal_collections (meal_session_id);
