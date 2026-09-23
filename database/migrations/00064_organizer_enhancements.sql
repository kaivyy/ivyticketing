-- +goose Up
-- Organizer enhancements: race category distance/COT, form answers persistence, refund columns, and payout management.

-- 1. Add distance_km, cutoff_time, and pricing_tiers to event_categories
ALTER TABLE event_categories
    ADD COLUMN IF NOT EXISTS distance_km numeric(5,2),
    ADD COLUMN IF NOT EXISTS cutoff_time text,
    ADD COLUMN IF NOT EXISTS pricing_tiers jsonb NOT NULL DEFAULT '[]'::jsonb;

-- 2. Add form_answers, refunded_amount, refund_reason, and refunded_at to orders
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS form_answers jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS refunded_amount bigint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refund_reason text,
    ADD COLUMN IF NOT EXISTS refunded_at timestamptz;

-- 3. Add form_answers to tickets
ALTER TABLE tickets
    ADD COLUMN IF NOT EXISTS form_answers jsonb NOT NULL DEFAULT '{}'::jsonb;

-- 4. Extend payments_status_check to include REFUNDED
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check CHECK (status IN ('PENDING','PAID','EXPIRED','FAILED','REFUNDED'));

-- 5. Create org_payout_accounts for organizer withdrawal bank accounts
CREATE TABLE IF NOT EXISTS org_payout_accounts (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    bank_name       text NOT NULL,
    bank_code       text NOT NULL,
    account_number  text NOT NULL,
    account_name    text NOT NULL,
    is_verified     boolean NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, bank_code, account_number)
);
CREATE INDEX IF NOT EXISTS idx_org_payout_accounts_org ON org_payout_accounts(organization_id);

-- 6. Create payout_requests for disbursement requests
CREATE TABLE IF NOT EXISTS payout_requests (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id          uuid REFERENCES events(id) ON DELETE SET NULL,
    payout_account_id uuid REFERENCES org_payout_accounts(id) ON DELETE SET NULL,
    amount            bigint NOT NULL CHECK (amount > 0),
    currency          text NOT NULL DEFAULT 'IDR',
    status            text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'PROCESSING', 'COMPLETED', 'REJECTED')),
    notes             text,
    rejection_reason  text,
    requested_by      uuid NOT NULL REFERENCES users(id),
    processed_by      uuid REFERENCES users(id),
    processed_at      timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_payout_requests_org ON payout_requests(organization_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS payout_requests;
DROP TABLE IF EXISTS org_payout_accounts;

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check CHECK (status IN ('PENDING','PAID','EXPIRED','FAILED'));

ALTER TABLE tickets DROP COLUMN IF EXISTS form_answers;

ALTER TABLE orders
    DROP COLUMN IF EXISTS refunded_at,
    DROP COLUMN IF EXISTS refund_reason,
    DROP COLUMN IF EXISTS refunded_amount,
    DROP COLUMN IF EXISTS form_answers;

ALTER TABLE event_categories
    DROP COLUMN IF EXISTS pricing_tiers,
    DROP COLUMN IF EXISTS cutoff_time,
    DROP COLUMN IF EXISTS distance_km;
