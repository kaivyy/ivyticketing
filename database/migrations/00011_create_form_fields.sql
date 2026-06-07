-- +goose Up
CREATE TABLE form_fields (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    form_schema_id  uuid NOT NULL REFERENCES form_schemas(id) ON DELETE CASCADE,
    field_type      text NOT NULL,
    label           text NOT NULL,
    field_key       text NOT NULL,
    help_text       text,
    is_required     boolean NOT NULL DEFAULT false,
    display_order   integer NOT NULL,
    options         jsonb,
    validation      jsonb,
    conditional     jsonb,
    category_scope  jsonb,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (form_schema_id, field_key)
);
CREATE INDEX idx_form_fields_schema ON form_fields(form_schema_id);
CREATE INDEX idx_form_fields_org ON form_fields(organization_id);

-- +goose Down
DROP TABLE form_fields;
