package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"vesko/auth"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo               auth.UserRepository
	tokenManager       *auth.TokenManager
	refreshStore       auth.RefreshTokenStore
	otpProvider        auth.OTPProvider
	pendingSignupStore auth.PendingSignupStore
	loginOTPStore      auth.OTPRequestStateStore
	otpConfig          auth.OTPConfig
}

func New(repo auth.UserRepository, tokenManager *auth.TokenManager, refreshStore auth.RefreshTokenStore, otpProvider auth.OTPProvider,
	pendingSignupStore auth.PendingSignupStore,
	loginOTPStore auth.OTPRequestStateStore,
	otpConfig auth.OTPConfig) *Service {
	return &Service{
		repo:               repo,
		tokenManager:       tokenManager,
		refreshStore:       refreshStore,
		otpProvider:        otpProvider,
		pendingSignupStore: pendingSignupStore,
		loginOTPStore:      loginOTPStore,
		otpConfig:          otpConfig,
	}
}

func (s *Service) ValidateLogin(ctx context.Context, username string, password string) (auth.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return auth.User{}, auth.ErrInvalidUsername
	}
	if password == "" {
		return auth.User{}, auth.ErrInvalidPassword
	}

	user, err := s.repo.GetUserDetailsByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return auth.User{}, auth.ErrInvalidCredentials
		}
		return auth.User{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return auth.User{}, auth.ErrInvalidCredentials
	}

	return user, nil
}

func (s *Service) Login(ctx context.Context, req auth.PasswordLoginRequest) (auth.User, auth.TokenPair, error) {
	user, err := s.ValidateLogin(ctx, req.Username, req.Password)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}

	tokens, err := s.IssueTokenPair(ctx, user)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}

	return user, tokens, nil
}

func (s *Service) RequestSignupOTP(ctx context.Context, req auth.RegisterRequest) error {
	normalized, err := s.normalizeSignupRequest(req)
	if err != nil {
		return err
	}

	if err := s.ensureSignupAvailability(ctx, normalized); err != nil {
		return err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(normalized.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if err := s.otpProvider.SendVerification(ctx, normalized.Mobile); err != nil {
		return err
	}

	return s.pendingSignupStore.Save(ctx, auth.PendingSignup{
		Username:     normalized.Username,
		PasswordHash: string(passwordHash),
		Email:        normalized.Email,
		Mobile:       normalized.Mobile,
		ResendCount:  0,
		LastSentAt:   now,
		ExpiresAt:    now.Add(s.otpConfig.PendingSignupTTL),
	})
}

func (s *Service) VerifySignupOTP(ctx context.Context, req auth.OTPVerifyRequest) (auth.User, auth.TokenPair, error) {
	mobile, err := auth.NormalizeIndianMobile(req.Mobile)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return auth.User{}, auth.TokenPair{}, auth.ErrInvalidOTPCode
	}

	pending, err := s.pendingSignupStore.GetByMobile(ctx, mobile)
	if err != nil {
		if isCacheMiss(err) {
			return auth.User{}, auth.TokenPair{}, auth.ErrPendingSignupExpired
		}
		return auth.User{}, auth.TokenPair{}, err
	}
	if time.Now().UTC().After(pending.ExpiresAt) {
		_ = s.pendingSignupStore.DeleteByMobile(ctx, mobile)
		return auth.User{}, auth.TokenPair{}, auth.ErrPendingSignupExpired
	}

	ok, err := s.otpProvider.CheckVerification(ctx, mobile, code)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}
	if !ok {
		return auth.User{}, auth.TokenPair{}, auth.ErrInvalidOTPCode
	}

	user, err := s.repo.RegisterUser(ctx, auth.User{
		Username:       pending.Username,
		PasswordHash:   pending.PasswordHash,
		Email:          pending.Email,
		Mobile:         pending.Mobile,
		MobileVerified: true,
	})
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}

	_ = s.pendingSignupStore.DeleteByMobile(ctx, mobile)

	tokens, err := s.IssueTokenPair(ctx, user)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}

	return user, tokens, nil
}

func (s *Service) ResendSignupOTP(ctx context.Context, req auth.OTPRequest) error {
	mobile, err := auth.NormalizeIndianMobile(req.Mobile)
	if err != nil {
		return err
	}

	pending, err := s.pendingSignupStore.GetByMobile(ctx, mobile)
	if err != nil {
		if isCacheMiss(err) {
			return auth.ErrPendingSignupExpired
		}
		return err
	}

	if err := s.validateResendWindow(pending.LastSentAt, pending.ResendCount); err != nil {
		return err
	}

	if err := s.otpProvider.SendVerification(ctx, mobile); err != nil {
		return err
	}

	pending.ResendCount++
	pending.LastSentAt = time.Now().UTC()
	return s.pendingSignupStore.Save(ctx, pending)
}

func (s *Service) SignupStatus(ctx context.Context, mobile string) (string, error) {
	normalized, err := auth.NormalizeIndianMobile(mobile)
	if err != nil {
		return "", err
	}

	pending, err := s.pendingSignupStore.GetByMobile(ctx, normalized)
	if err == nil && time.Now().UTC().Before(pending.ExpiresAt) {
		return "pending", nil
	}
	if err != nil && !isCacheMiss(err) {
		return "", err
	}

	user, err := s.repo.GetUserByMobile(ctx, normalized)
	if err == nil && user.MobileVerified {
		return "completed", nil
	}
	if err != nil && !errors.Is(err, auth.ErrUserNotFound) {
		return "", err
	}

	return "expired", nil
}

func (s *Service) RequestLoginOTP(ctx context.Context, req auth.OTPRequest) error {
	mobile, err := auth.NormalizeIndianMobile(req.Mobile)
	if err != nil {
		return nil
	}

	user, err := s.repo.GetUserByMobile(ctx, mobile)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil
		}
		return err
	}
	if !user.MobileVerified {
		return nil
	}

	stateKey := s.loginOTPStateKey(mobile)
	state, err := s.loginOTPStore.Get(ctx, stateKey)
	switch {
	case err == nil:
		if err := s.validateResendWindow(state.LastSentAt, state.ResendCount); err != nil {
			return nil
		}
	case !isCacheMiss(err):
		return err
	}

	if err := s.otpProvider.SendVerification(ctx, mobile); err != nil {
		return err
	}

	now := time.Now().UTC()
	resendCount := 0
	if err == nil {
		resendCount = state.ResendCount + 1
	}

	return s.loginOTPStore.Save(ctx, stateKey, auth.OTPRequestState{
		Mobile:      mobile,
		ResendCount: resendCount,
		LastSentAt:  now,
		ExpiresAt:   now.Add(s.otpConfig.PendingSignupTTL),
	})
}

func (s *Service) ResendLoginOTP(ctx context.Context, req auth.OTPRequest) error {
	return s.RequestLoginOTP(ctx, req)
}

func (s *Service) VerifyLoginOTP(ctx context.Context, req auth.OTPVerifyRequest) (auth.User, auth.TokenPair, error) {
	mobile, err := auth.NormalizeIndianMobile(req.Mobile)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, auth.ErrInvalidCredentials
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return auth.User{}, auth.TokenPair{}, auth.ErrInvalidOTPCode
	}

	user, err := s.repo.GetUserByMobile(ctx, mobile)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return auth.User{}, auth.TokenPair{}, auth.ErrInvalidCredentials
		}
		return auth.User{}, auth.TokenPair{}, err
	}
	if !user.MobileVerified {
		return auth.User{}, auth.TokenPair{}, auth.ErrMobileNotVerified
	}

	ok, err := s.otpProvider.CheckVerification(ctx, mobile, code)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}
	if !ok {
		return auth.User{}, auth.TokenPair{}, auth.ErrInvalidOTPCode
	}

	_ = s.loginOTPStore.Delete(ctx, s.loginOTPStateKey(mobile))

	tokens, err := s.IssueTokenPair(ctx, user)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}

	return user, tokens, nil
}

func (s *Service) IssueTokenPair(ctx context.Context, user auth.User) (auth.TokenPair, error) {
	if s.tokenManager == nil || s.refreshStore == nil {
		return auth.TokenPair{}, auth.ErrUnauthorized
	}

	tokenPair, session, err := s.tokenManager.GenerateTokenPair(user)
	if err != nil {
		return auth.TokenPair{}, err
	}

	if err := s.refreshStore.Save(ctx, tokenPair.RefreshToken, session); err != nil {
		return auth.TokenPair{}, err
	}

	return tokenPair, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (auth.User, auth.TokenPair, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return auth.User{}, auth.TokenPair{}, auth.ErrInvalidRefreshToken
	}

	session, err := s.refreshStore.Get(ctx, refreshToken)
	if err != nil {
		if isCacheMiss(err) {
			return auth.User{}, auth.TokenPair{}, auth.ErrInvalidRefreshToken
		}
		return auth.User{}, auth.TokenPair{}, err
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		_ = s.refreshStore.Delete(ctx, refreshToken)
		return auth.User{}, auth.TokenPair{}, auth.ErrTokenExpired
	}

	user, err := s.repo.GetUserDetailsByUsername(ctx, session.Username)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}

	_ = s.refreshStore.Delete(ctx, refreshToken)

	tokens, err := s.IssueTokenPair(ctx, user)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}

	return user, tokens, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return auth.ErrInvalidRefreshToken
	}

	return s.refreshStore.Delete(ctx, refreshToken)
}

func (s *Service) AuthenticateAccessToken(token string) (auth.AccessClaims, error) {
	if s.tokenManager == nil {
		return auth.AccessClaims{}, auth.ErrUnauthorized
	}

	return s.tokenManager.ParseAccessToken(token)
}

func (s *Service) ValidateCurrentUser(ctx context.Context, username string) (auth.User, error) {
	return s.repo.GetUserDetailsByUsername(ctx, strings.TrimSpace(username))
}

func (s *Service) normalizeSignupRequest(req auth.RegisterRequest) (auth.RegisterRequest, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Username == "" {
		return auth.RegisterRequest{}, auth.ErrInvalidUsername
	}
	if req.Password == "" {
		return auth.RegisterRequest{}, auth.ErrInvalidPassword
	}
	if req.Email == "" {
		return auth.RegisterRequest{}, auth.ErrInvalidEmail
	}
	mobile, err := auth.NormalizeIndianMobile(req.Mobile)
	if err != nil {
		return auth.RegisterRequest{}, err
	}
	req.Mobile = mobile
	return req, nil
}

func (s *Service) ensureSignupAvailability(ctx context.Context, req auth.RegisterRequest) error {
	if user, err := s.repo.GetUserDetailsByUsername(ctx, req.Username); err == nil && user.ID != 0 {
		return auth.ErrUsernameAlreadyExists
	} else if err != nil && !errors.Is(err, auth.ErrUserNotFound) {
		return err
	}
	if user, err := s.repo.GetUserByEmail(ctx, req.Email); err == nil && user.ID != 0 {
		return auth.ErrEmailAlreadyExists
	} else if err != nil && !errors.Is(err, auth.ErrUserNotFound) {
		return err
	}
	if user, err := s.repo.GetUserByMobile(ctx, req.Mobile); err == nil && user.ID != 0 {
		return auth.ErrMobileAlreadyExists
	} else if err != nil && !errors.Is(err, auth.ErrUserNotFound) {
		return err
	}

	usernamePendingMobile, err := s.pendingSignupStore.GetMobileByUsername(ctx, req.Username)
	if err == nil {
		return s.pendingConflict(usernamePendingMobile, req.Mobile, req.Username, req.Email, ctx)
	}
	if err != nil && !isCacheMiss(err) {
		return err
	}
	emailPendingMobile, err := s.pendingSignupStore.GetMobileByEmail(ctx, req.Email)
	if err == nil {
		return s.pendingConflict(emailPendingMobile, req.Mobile, req.Username, req.Email, ctx)
	}
	if err != nil && !isCacheMiss(err) {
		return err
	}
	mobilePending, err := s.pendingSignupStore.GetMobileByMobile(ctx, req.Mobile)
	if err == nil {
		return s.pendingConflict(mobilePending, req.Mobile, req.Username, req.Email, ctx)
	}
	if err != nil && !isCacheMiss(err) {
		return err
	}

	return nil
}

func (s *Service) pendingConflict(pendingMobile string, requestedMobile string, requestedUsername string, requestedEmail string, ctx context.Context) error {
	pending, err := s.pendingSignupStore.GetByMobile(ctx, pendingMobile)
	if err != nil {
		if isCacheMiss(err) {
			return nil
		}
		return err
	}
	if pending.Mobile == requestedMobile && pending.Username == requestedUsername && pending.Email == requestedEmail {
		return auth.ErrSignupVerificationPending
	}
	if pending.Username == requestedUsername {
		return auth.ErrUsernameAlreadyExists
	}
	if pending.Email == requestedEmail {
		return auth.ErrEmailAlreadyExists
	}
	if pending.Mobile == requestedMobile {
		return auth.ErrMobileAlreadyExists
	}
	return auth.ErrSignupVerificationPending
}

func (s *Service) validateResendWindow(lastSentAt time.Time, resendCount int) error {
	if s.otpConfig.MaxResends > 0 && resendCount >= s.otpConfig.MaxResends {
		return auth.ErrOTPResendLimitReached
	}
	if !lastSentAt.IsZero() && s.otpConfig.ResendCooldown > 0 && time.Since(lastSentAt) < s.otpConfig.ResendCooldown {
		return auth.ErrOTPResendTooSoon
	}
	return nil
}

func (s *Service) loginOTPStateKey(mobile string) string {
	return "login:" + mobile
}

func isCacheMiss(err error) bool {
	return errors.Is(err, auth.ErrCacheMiss)
}
