package auth

import "github.com/google/uuid"

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"fullName"`
	Phone    string `json:"phone"`
}

type UserResponse struct {
	ID       uuid.UUID `json:"id"`
	Email    string    `json:"email"`
	FullName string    `json:"fullName"`
	Phone    string    `json:"phone"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResult carries the access token for the body plus the raw refresh token
// for the handler to set as an HttpOnly cookie (never serialized to JSON).
type LoginResult struct {
	AccessToken  string       `json:"accessToken"`
	ExpiresIn    int          `json:"expiresIn"`
	User         UserResponse `json:"user"`
	RefreshToken string       `json:"-"`
	RefreshTTL   int          `json:"-"`
}

type RefreshResult struct {
	AccessToken  string `json:"accessToken"`
	ExpiresIn    int    `json:"expiresIn"`
	RefreshToken string `json:"-"`
	RefreshTTL   int    `json:"-"`
}

type MembershipResponse struct {
	OrganizationID uuid.UUID `json:"organizationId"`
	MemberID       uuid.UUID `json:"memberId"`
	RoleSlugs      []string  `json:"roleSlugs"`
	Permissions    []string  `json:"permissions"`
}

type MeResponse struct {
	User        UserResponse         `json:"user"`
	Memberships []MembershipResponse `json:"memberships"`
}
