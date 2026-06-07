-- +goose Up
CREATE TABLE roles (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid REFERENCES organizations(id) ON DELETE CASCADE,
    name            text NOT NULL,
    slug            text NOT NULL,
    is_system       boolean NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, slug)
);
-- Enforce uniqueness for template roles (organization_id IS NULL),
-- since UNIQUE treats NULLs as distinct.
CREATE UNIQUE INDEX idx_roles_template_slug
    ON roles(slug) WHERE organization_id IS NULL;

CREATE TABLE permissions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key         text NOT NULL UNIQUE,
    description text NOT NULL DEFAULT ''
);

CREATE TABLE role_permissions (
    role_id       uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE member_roles (
    organization_member_id uuid NOT NULL REFERENCES organization_members(id) ON DELETE CASCADE,
    role_id                uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (organization_member_id, role_id)
);

-- +goose Down
DROP TABLE member_roles;
DROP TABLE role_permissions;
DROP TABLE permissions;
DROP TABLE roles;
