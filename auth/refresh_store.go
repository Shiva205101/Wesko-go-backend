package auth

import "context"

type RefreshTokenStore interface {
	Save(ctx context.Context, rawToken string, session RefreshSession) error
	Get(ctx context.Context, rawToken string) (RefreshSession, error)
	Delete(ctx context.Context, rawToken string) error
}
