package auth

import "time"

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleCustomer Role = "customer"
)

type User struct {
	ID                uint
	Username          string
	Email             string
	Mobile            string
	MobileVerified    bool
	Role              Role
	IsProfileComplete bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type SSOAccount struct {
	UserID     uint
	Provider   string
	ProviderID string
	Email      string
	Data       map[string]any
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}
