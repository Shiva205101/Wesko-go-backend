package auth

import (
	"context"
	"errors"
)

type CachedUserRepository struct {
	store UserRepository
	cache UserCache
}

func NewCachedUserRepository(store UserRepository, cache UserCache) *CachedUserRepository {
	return &CachedUserRepository{
		store: store,
		cache: cache,
	}
}

func (r *CachedUserRepository) GetUserDetailsByUsername(ctx context.Context, username string) (User, error) {
	if r.cache != nil {
		user, err := r.cache.GetUser(ctx, username)
		switch {
		case err == nil:
			return user, nil
		case errors.Is(err, ErrCacheMiss):
		default:
		}
	}

	user, err := r.store.GetUserDetailsByUsername(ctx, username)
	if err != nil {
		return User{}, err
	}

	if r.cache != nil {
		_ = r.cache.SetUser(ctx, user)
	}

	return user, nil
}

func (r *CachedUserRepository) GetUserByEmail(ctx context.Context, email string) (User, error) {
	user, err := r.store.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, err
	}

	if r.cache != nil {
		_ = r.cache.SetUser(ctx, user)
	}

	return user, nil
}

func (r *CachedUserRepository) GetUserByMobile(ctx context.Context, mobile string) (User, error) {
	user, err := r.store.GetUserByMobile(ctx, mobile)
	if err != nil {
		return User{}, err
	}

	if r.cache != nil {
		_ = r.cache.SetUser(ctx, user)
	}

	return user, nil
}

func (r *CachedUserRepository) RegisterUser(ctx context.Context, user User) (User, error) {
	createdUser, err := r.store.RegisterUser(ctx, user)
	if err != nil {
		return User{}, err
	}

	if r.cache != nil {
		_ = r.cache.SetUser(ctx, createdUser)
	}

	return createdUser, nil
}
