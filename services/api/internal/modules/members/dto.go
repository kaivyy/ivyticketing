package members

import "github.com/google/uuid"

type MemberResponse struct {
	ID       uuid.UUID   `json:"id"`
	UserID   uuid.UUID   `json:"userId"`
	Email    string      `json:"email"`
	FullName string      `json:"fullName"`
	RoleIDs  []uuid.UUID `json:"roleIds"`
}

type AddMemberRequest struct {
	Email   string      `json:"email"`
	RoleIDs []uuid.UUID `json:"roleIds"`
}

type UpdateRolesRequest struct {
	RoleIDs []uuid.UUID `json:"roleIds"`
}
