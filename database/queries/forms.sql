-- name: GetFormSchemaByEvent :one
SELECT * FROM form_schemas WHERE event_id = $1;

-- name: CreateFormSchema :one
INSERT INTO form_schemas (organization_id, event_id, name)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateFormSchemaName :one
UPDATE form_schemas SET name = $2, updated_at = now()
WHERE id = $1 AND organization_id = $3
RETURNING *;

-- name: ListFieldsBySchema :many
SELECT * FROM form_fields WHERE form_schema_id = $1 ORDER BY display_order;

-- name: GetFieldByID :one
SELECT * FROM form_fields WHERE id = $1;

-- name: CreateField :one
INSERT INTO form_fields (organization_id, form_schema_id, field_type, label, field_key,
    help_text, is_required, display_order, options, validation, conditional, category_scope)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdateField :one
UPDATE form_fields SET
    field_type = $2, label = $3, field_key = $4, help_text = $5, is_required = $6,
    options = $7, validation = $8, conditional = $9, category_scope = $10, updated_at = now()
WHERE id = $1 AND form_schema_id = $11
RETURNING *;

-- name: UpdateFieldOrder :exec
UPDATE form_fields SET display_order = $2, updated_at = now()
WHERE id = $1 AND form_schema_id = $3;

-- name: DeleteField :exec
DELETE FROM form_fields WHERE id = $1 AND form_schema_id = $2;

-- name: MaxFieldOrder :one
SELECT COALESCE(MAX(display_order), 0)::int FROM form_fields WHERE form_schema_id = $1;
