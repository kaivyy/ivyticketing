package roles

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrRoleNotFound = apperr.New(http.StatusNotFound, "ROLE_NOT_FOUND", "role not found")
	ErrSystemRole   = apperr.New(http.StatusForbidden, "ROLE_IS_SYSTEM", "cannot modify or delete a system role")
	ErrRoleInUse    = apperr.New(http.StatusConflict, "ROLE_IN_USE", "role is still assigned to members")
	ErrUnknownPerm  = apperr.New(http.StatusBadRequest, "UNKNOWN_PERMISSION", "unknown permission key")
	ErrLastOwner    = apperr.New(http.StatusConflict, "LAST_OWNER", "cannot remove the last owner")
	ErrSlugConflict = apperr.New(http.StatusConflict, "ROLE_SLUG_TAKEN", "a role with this name already exists")
)
