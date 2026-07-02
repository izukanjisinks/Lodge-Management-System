-- Recreate individual_profiles as the org-scoped back-office client registry.
-- It was dropped in 000075 (guest-linked design), but the back-office Individual
-- Clients page and booking-approval materialisation need an org-scoped, deduped
-- registry keyed on the guest's ID/passport number.
--
-- Guest signup no longer writes here — guest identity lives in web_users. This
-- table is populated by staff (walk-in dialog) and by booking approval, and is
-- deduplicated on (org_id, id_passport_number).

CREATE TABLE IF NOT EXISTS individual_profiles (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    full_name           VARCHAR(255) NOT NULL,
    email               VARCHAR(255),
    phone               VARCHAR(50),
    id_passport_number  VARCHAR(100) NOT NULL,
    nationality         VARCHAR(100),
    status              VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_individual_profiles_id_passport_number UNIQUE (id_passport_number, org_id)
);

CREATE INDEX idx_individual_profiles_org_id ON individual_profiles(org_id);
CREATE INDEX idx_individual_profiles_status ON individual_profiles(status);
CREATE INDEX idx_individual_profiles_id_passport ON individual_profiles(id_passport_number);
