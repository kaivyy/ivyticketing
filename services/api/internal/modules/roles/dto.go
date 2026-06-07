package roles

import "github.com/google/uuid"

type PermissionResponse struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

type RoleResponse struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	IsSystem       bool      `json:"isSystem"`
	PermissionKeys []string  `json:"permissionKeys"`
}

type CreateRoleRequest struct {
	Name           string   `json:"name"`
	PermissionKeys []string `json:"permissionKeys"`
}

type UpdateRoleRequest struct {
	Name           *string   `json:"name"`
	PermissionKeys *[]string `json:"permissionKeys"`
}
