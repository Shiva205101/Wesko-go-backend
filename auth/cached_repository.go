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

func (r *CachedUserRepository) GetUserByID(ctx context.Context, id uint) (User, error) {
	// Not caching by ID for now to keep it simple, or we could add another cache
	return r.store.GetUserByID(ctx, id)
}

func (r *CachedUserRepository) RegisterUser(ctx context.Context, user User, passwordHash string) (User, error) {
	createdUser, err := r.store.RegisterUser(ctx, user, passwordHash)
	if err != nil {
		return User{}, err
	}

	if r.cache != nil {
		_ = r.cache.SetUser(ctx, createdUser)
	}

	return createdUser, nil
}

func (r *CachedUserRepository) UpdateUser(ctx context.Context, user User) error {
	err := r.store.UpdateUser(ctx, user)
	if err != nil {
		return err
	}

	if r.cache != nil {
		_ = r.cache.DeleteUser(ctx, user.Username)
	}

	return nil
}

func (r *CachedUserRepository) GetPasswordHashByUserID(ctx context.Context, userID uint) (string, error) {
	return r.store.GetPasswordHashByUserID(ctx, userID)
}

func (r *CachedUserRepository) GetSSOAccount(ctx context.Context, provider string, providerID string) (User, bool, error) {
	return r.store.GetSSOAccount(ctx, provider, providerID)
}

func (r *CachedUserRepository) CreateSSOUser(ctx context.Context, user User, account SSOAccount) (User, error) {
	createdUser, err := r.store.CreateSSOUser(ctx, user, account)
	if err != nil {
		return User{}, err
	}

	if r.cache != nil {
		_ = r.cache.SetUser(ctx, createdUser)
	}

	return createdUser, nil
}

func (r *CachedUserRepository) LinkSSOAccount(ctx context.Context, userID uint, account SSOAccount) error {
	return r.store.LinkSSOAccount(ctx, userID, account)
}
