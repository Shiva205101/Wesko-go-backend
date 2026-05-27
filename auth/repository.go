package auth

import "context"

type UserRepository interface {
	GetUserDetailsByUsername(ctx context.Context, username string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByMobile(ctx context.Context, mobile string) (User, error)
	GetUserByID(ctx context.Context, id uint) (User, error)
	RegisterUser(ctx context.Context, user User, passwordHash string) (User, error)
	UpdateUser(ctx context.Context, user User) error

	GetPasswordHashByUserID(ctx context.Context, userID uint) (string, error)

	GetSSOAccount(ctx context.Context, provider string, providerID string) (User, bool, error)
	CreateSSOUser(ctx context.Context, user User, account SSOAccount) (User, error)
	LinkSSOAccount(ctx context.Context, userID uint, account SSOAccount) error
}

type UserCache interface {
	GetUser(ctx context.Context, username string) (User, error)
	SetUser(ctx context.Context, user User) error
	DeleteUser(ctx context.Context, username string) error
}
