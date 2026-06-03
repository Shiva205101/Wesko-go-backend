package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"vesko/auth"
	applogger "vesko/logger"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleoauth "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

type Service struct {
	repo               auth.UserRepository
	tokenManager       *auth.TokenManager
	refreshStore       auth.RefreshTokenStore
	otpProvider        auth.OTPProvider
	pendingSignupStore auth.PendingSignupStore
	loginOTPStore      auth.OTPRequestStateStore
	otpConfig          auth.OTPConfig
	googleConfig       auth.GoogleConfig
	logger             *slog.Logger
}

func New(repo auth.UserRepository,
	tokenManager *auth.TokenManager,
	refreshStore auth.RefreshTokenStore,
	otpProvider auth.OTPProvider,
	pendingSignupStore auth.PendingSignupStore,
	loginOTPStore auth.OTPRequestStateStore,
	otpConfig auth.OTPConfig,
	googleConfig auth.GoogleConfig,
	logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		repo:               repo,
		tokenManager:       tokenManager,
		refreshStore:       refreshStore,
		otpProvider:        otpProvider,
		pendingSignupStore: pendingSignupStore,
		loginOTPStore:      loginOTPStore,
		otpConfig:          otpConfig,
		googleConfig:       googleConfig,
		logger:             logger.With("component", "auth_service"),
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

	passwordHash, err := s.repo.GetPasswordHashByUserID(ctx, user.ID)
	if err != nil {
		return auth.User{}, auth.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
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
		Username:          pending.Username,
		Email:             pending.Email,
		Mobile:            pending.Mobile,
		MobileVerified:    true,
		Role:              auth.RoleCustomer,
		IsProfileComplete: true,
	}, pending.PasswordHash)
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

func (s *Service) GetGoogleAuthURL() string {
	conf := &oauth2.Config{
		ClientID:     s.googleConfig.ClientID,
		ClientSecret: s.googleConfig.ClientSecret,
		RedirectURL:  s.googleConfig.RedirectURI,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
	}

	return conf.AuthCodeURL("state") // In production, "state" should be a random string for CSRF protection
}

func (s *Service) GoogleLogin(ctx context.Context, code string) (auth.User, auth.TokenPair, error) {
	l := applogger.FromContext(ctx).With("method", "GoogleLogin")
	l.Info("starting google login flow")

	conf := &oauth2.Config{
		ClientID:     s.googleConfig.ClientID,
		ClientSecret: s.googleConfig.ClientSecret,
		RedirectURL:  s.googleConfig.RedirectURI,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
	}

	l.Info("exchanging auth code for token")
	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		l.Error("failed to exchange auth code", "error", err)
		return auth.User{}, auth.TokenPair{}, err
	}

	l.Info("fetching user info from google")
	oauth2Service, err := googleoauth.NewService(ctx, option.WithTokenSource(conf.TokenSource(ctx, tok)))
	if err != nil {
		l.Error("failed to create oauth2 service", "error", err)
		return auth.User{}, auth.TokenPair{}, err
	}

	userInfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		l.Error("failed to get user info", "error", err)
		return auth.User{}, auth.TokenPair{}, err
	}

	l.Info("google identity resolved",
		"provider_id", userInfo.Id,
		"email", userInfo.Email,
	)

	// 1. Check if SSO account exists
	l.Info("checking for existing sso account")
	user, exists, err := s.repo.GetSSOAccount(ctx, "google", userInfo.Id)
	if err != nil {
		l.Error("failed to check sso account", "error", err)
		return auth.User{}, auth.TokenPair{}, err
	}

	if exists {
		l.Info("existing sso account found, issuing tokens",
			"user_id", user.ID,
			"is_profile_complete", user.IsProfileComplete,
		)
		tokens, err := s.IssueTokenPair(ctx, user)
		if err != nil {
			l.Error("failed to issue tokens", "error", err)
		}
		return user, tokens, err
	}

	// 2. Check if user with same email exists (Option A: Auto-link)
	l.Info("searching for user by email for auto-linking", "email", userInfo.Email)
	user, err = s.repo.GetUserByEmail(ctx, userInfo.Email)
	if err == nil {
		l.Info("user with email found, linking sso account", "user_id", user.ID)
		err = s.repo.LinkSSOAccount(ctx, user.ID, auth.SSOAccount{
			Provider:   "google",
			ProviderID: userInfo.Id,
			Email:      userInfo.Email,
		})
		if err != nil {
			l.Error("failed to link sso account", "error", err)
			return auth.User{}, auth.TokenPair{}, err
		}

		l.Info("sso account linked successfully, issuing tokens",
			"is_profile_complete", user.IsProfileComplete,
		)
		tokens, err := s.IssueTokenPair(ctx, user)
		return user, tokens, err
	}

	if !errors.Is(err, auth.ErrUserNotFound) {
		l.Error("failed to fetch user by email", "error", err)
		return auth.User{}, auth.TokenPair{}, err
	}

	// 3. New User: Create with IsProfileComplete = false
	l.Info("no existing user found, creating new sso user")
	var username string
	usernameArray := strings.Split(userInfo.Email, "@")
	if len(usernameArray) > 1 {
		username = usernameArray[0]
	}

	if _, err := s.repo.GetUserDetailsByUsername(ctx, username); err == nil {
		username = fmt.Sprintf("%s_%d", username, time.Now().Unix()%1000)
	}

	user, err = s.repo.CreateSSOUser(ctx, auth.User{
		Username:          username,
		Email:             userInfo.Email,
		Role:              auth.RoleCustomer,
		IsProfileComplete: false,
	}, auth.SSOAccount{
		Provider:   "google",
		ProviderID: userInfo.Id,
		Email:      userInfo.Email,
	})
	if err != nil {
		l.Error("failed to create sso user", "error", err)
		return auth.User{}, auth.TokenPair{}, err
	}

	l.Info("new sso user created successfully", "user_id", user.ID, "username", username)
	tokens, err := s.IssueTokenPair(ctx, user)
	return user, tokens, err
}

func (s *Service) CompleteProfile(ctx context.Context, userID uint, username string, mobile string) (auth.User, auth.TokenPair, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}

	if user.IsProfileComplete {
		return auth.User{}, auth.TokenPair{}, auth.ErrProfileAlreadyComplete
	}

	username = strings.TrimSpace(username)
	if username == "" {
		return auth.User{}, auth.TokenPair{}, auth.ErrInvalidUsername
	}

	normalizedMobile, err := auth.NormalizeIndianMobile(mobile)
	if err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}

	// Check if username is taken by another user
	if existing, err := s.repo.GetUserDetailsByUsername(ctx, username); err == nil && existing.ID != userID {
		return auth.User{}, auth.TokenPair{}, auth.ErrUsernameAlreadyExists
	}

	// Check if mobile is taken
	if existing, err := s.repo.GetUserByMobile(ctx, normalizedMobile); err == nil && existing.ID != userID {
		return auth.User{}, auth.TokenPair{}, auth.ErrMobileAlreadyExists
	}

	user.Username = username
	user.Mobile = normalizedMobile
	user.IsProfileComplete = true

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return auth.User{}, auth.TokenPair{}, err
	}

	tokens, err := s.IssueTokenPair(ctx, user)
	return user, tokens, err
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
	if !isCacheMiss(err) {
		return err
	}
	emailPendingMobile, err := s.pendingSignupStore.GetMobileByEmail(ctx, req.Email)
	if err == nil {
		return s.pendingConflict(emailPendingMobile, req.Mobile, req.Username, req.Email, ctx)
	}
	if !isCacheMiss(err) {
		return err
	}
	mobilePending, err := s.pendingSignupStore.GetMobileByMobile(ctx, req.Mobile)
	if err == nil {
		return s.pendingConflict(mobilePending, req.Mobile, req.Username, req.Email, ctx)
	}
	if !isCacheMiss(err) {
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
	return err != nil && errors.Is(err, auth.ErrCacheMiss)
}
