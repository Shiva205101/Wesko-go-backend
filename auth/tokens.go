package auth

import "time"

type AccessClaims struct {
	Subject           string    `json:"sub"`
	TokenType         string    `json:"typ"`
	Role              string    `json:"role"`
	IsProfileComplete bool      `json:"is_profile_complete"`
	IssuedAt          time.Time `json:"iat"`
	ExpiresAt         time.Time `json:"exp"`
}

type RefreshSession struct {
	TokenID   string    `json:"token_id"`
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

type TokenPair struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	TokenType             string `json:"token_type"`
	AccessTokenExpiresIn  int64  `json:"access_token_expires_in"`
	RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
}
