package security

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID          uuid.UUID
	IsPlatformAdmin bool
}

type JWTSigner struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTSigner(secret string, ttl time.Duration) *JWTSigner {
	return &JWTSigner{secret: []byte(secret), ttl: ttl}
}

func (s *JWTSigner) Sign(userID uuid.UUID, isPlatformAdmin bool) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":               userID.String(),
		"is_platform_admin": isPlatformAdmin,
		"iat":               now.Unix(),
		"exp":               now.Add(s.ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *JWTSigner) Verify(tokenStr string) (Claims, error) {
	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil || !tok.Valid {
		return Claims{}, fmt.Errorf("invalid token: %w", err)
	}
	mc, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, fmt.Errorf("invalid claims")
	}
	sub, _ := mc["sub"].(string)
	uid, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid sub claim: %w", err)
	}
	isAdmin, _ := mc["is_platform_admin"].(bool)
	return Claims{UserID: uid, IsPlatformAdmin: isAdmin}, nil
}
