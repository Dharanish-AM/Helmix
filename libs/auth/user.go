package auth

import (
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

// User contains the authenticated Helmix identity propagated between services.
type User struct {
	UserID         string    `json:"user_id"`
	OrgID          string    `json:"org_id"`
	Role           string    `json:"role"`
	Email          string    `json:"email"`
	GitHubUsername string    `json:"github_username"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// Claims is the canonical JWT payload used across Helmix services.
type Claims struct {
	UserID         string `json:"user_id"`
	OrgID          string `json:"org_id"`
	Role           string `json:"role"`
	Email          string `json:"email"`
	GitHubUsername string `json:"github_username"`
	jwt.RegisteredClaims
}

// ToUser converts claims into the shared user model.
func (c Claims) ToUser() *User {
	var expiresAt time.Time
	if c.ExpiresAt != nil {
		expiresAt = c.ExpiresAt.Time
	}

	return &User{
		UserID:         c.UserID,
		OrgID:          c.OrgID,
		Role:           c.Role,
		Email:          c.Email,
		GitHubUsername: c.GitHubUsername,
		ExpiresAt:      expiresAt,
	}
}
