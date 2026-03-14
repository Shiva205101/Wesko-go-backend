package auth

import "context"

type UserRepository interface {
	GetUserDetailsByUsername(ctx context.Context, username string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByMobile(ctx context.Context, mobile string) (User, error)
	RegisterUser(ctx context.Context, user User) (User, error)
}

type UserCache interface {
	GetUser(ctx context.Context, username string) (User, error)
	SetUser(ctx context.Context, user User) error
	DeleteUser(ctx context.Context, username string) error
}
