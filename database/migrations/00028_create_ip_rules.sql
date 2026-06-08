-- +goose Up
CREATE TABLE ip_rules (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cidr       text NOT NULL,
    rule       text NOT NULL,
    note       text,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ir_rule_check CHECK (rule IN ('allow','deny')),
    CONSTRAINT ir_unique UNIQUE (cidr, rule)
);

-- +goose Down
DROP TABLE ip_rules;
