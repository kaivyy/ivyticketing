-- +goose Up
CREATE TABLE corporate_accounts (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name             text NOT NULL,
    billing_email    text NOT NULL,
    invoice_required boolean NOT NULL DEFAULT false,
    status           text NOT NULL DEFAULT 'PENDING'
                         CHECK (status IN ('PENDING','ACTIVE','SUSPENDED')),
    approved_at      timestamptz,
    approved_by      uuid REFERENCES users(id),
    created_by       uuid NOT NULL REFERENCES users(id),
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX corporate_accounts_org_idx ON corporate_accounts(organization_id, status);

-- +goose Down
DROP TABLE corporate_accounts;
