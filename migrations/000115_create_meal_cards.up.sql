-- RFID card assignments: a physical card mapped to a room and (optionally) a
-- specific checked-in resident. holder_name / identification_card are denormalized
-- from the attendee at assignment time for accountability; billing still resolves
-- to the room's current booking at collect time.
CREATE TABLE IF NOT EXISTS meal_cards (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id               UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    branch_id            UUID REFERENCES branches(id) ON DELETE SET NULL,
    card_uid             VARCHAR(128) NOT NULL,
    room_id              UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    role                 VARCHAR(20)  NOT NULL DEFAULT 'resident', -- resident|room_service
    attendee_id          UUID REFERENCES booking_attendees(id) ON DELETE SET NULL,
    holder_name          VARCHAR(255),
    identification_card  VARCHAR(128),
    status               VARCHAR(20)  NOT NULL DEFAULT 'active',   -- active|inactive|replaced|void
    replaced_card_id     UUID REFERENCES meal_cards(id) ON DELETE SET NULL,
    issued_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A card UID can only be active for one assignment at a time within an org.
CREATE UNIQUE INDEX IF NOT EXISTS uq_meal_cards_active_uid
    ON meal_cards (org_id, card_uid) WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_meal_cards_org  ON meal_cards (org_id);
CREATE INDEX IF NOT EXISTS idx_meal_cards_room ON meal_cards (room_id);
