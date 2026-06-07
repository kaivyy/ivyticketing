package members

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrUserNotFound   = apperr.New(http.StatusNotFound, "USER_NOT_FOUND", "no user with that email")
	ErrAlreadyMember  = apperr.New(http.StatusConflict, "ALREADY_MEMBER", "user is already a member")
	ErrMemberNotFound = apperr.New(http.StatusNotFound, "MEMBER_NOT_FOUND", "member not found")
	ErrRoleNotInOrg   = apperr.New(http.StatusBadRequest, "ROLE_NOT_IN_ORG", "one or more roles do not belong to this organization")
	ErrLastOwner      = apperr.New(http.StatusConflict, "LAST_OWNER", "cannot remove or demote the last owner")
)
