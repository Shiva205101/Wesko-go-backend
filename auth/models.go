package auth

import "time"

type User struct {
	ID             uint
	Username       string
	PasswordHash   string
	Email          string
	Mobile         string
	MobileVerified bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
