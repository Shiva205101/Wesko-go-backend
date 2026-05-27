package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"vesko/auth"
)

type fakeUserRepo struct {
	byUsername  map[string]auth.User
	byEmail     map[string]auth.User
	byMobile    map[string]auth.User
	byID        map[uint]auth.User
	passwords   map[uint]string
	ssoAccounts map[string]auth.User // key: provider:providerID
}

func (r *fakeUserRepo) GetUserDetailsByUsername(_ context.Context, username string) (auth.User, error) {
	if user, ok := r.byUsername[username]; ok {
		return user, nil
	}
	return auth.User{}, auth.ErrUserNotFound
}

func (r *fakeUserRepo) GetUserByEmail(_ context.Context, email string) (auth.User, error) {
	if user, ok := r.byEmail[email]; ok {
		return user, nil
	}
	return auth.User{}, auth.ErrUserNotFound
}

func (r *fakeUserRepo) GetUserByMobile(_ context.Context, mobile string) (auth.User, error) {
	if user, ok := r.byMobile[mobile]; ok {
		return user, nil
	}
	return auth.User{}, auth.ErrUserNotFound
}

func (r *fakeUserRepo) GetUserByID(_ context.Context, id uint) (auth.User, error) {
	if user, ok := r.byID[id]; ok {
		return user, nil
	}
	return auth.User{}, auth.ErrUserNotFound
}

func (r *fakeUserRepo) RegisterUser(_ context.Context, user auth.User, passwordHash string) (auth.User, error) {
	if _, ok := r.byUsername[user.Username]; ok {
		return auth.User{}, auth.ErrUsernameAlreadyExists
	}
	if _, ok := r.byEmail[user.Email]; ok {
		return auth.User{}, auth.ErrEmailAlreadyExists
	}
	if user.Mobile != "" {
		if _, ok := r.byMobile[user.Mobile]; ok {
			return auth.User{}, auth.ErrMobileAlreadyExists
		}
	}
	user.ID = uint(len(r.byUsername) + 1)
	r.byUsername[user.Username] = user
	r.byEmail[user.Email] = user
	if user.Mobile != "" {
		r.byMobile[user.Mobile] = user
	}
	r.byID[user.ID] = user
	if passwordHash != "" {
		r.passwords[user.ID] = passwordHash
	}
	return user, nil
}

func (r *fakeUserRepo) UpdateUser(_ context.Context, user auth.User) error {
	r.byUsername[user.Username] = user
	r.byEmail[user.Email] = user
	if user.Mobile != "" {
		r.byMobile[user.Mobile] = user
	}
	r.byID[user.ID] = user
	return nil
}

func (r *fakeUserRepo) GetPasswordHashByUserID(_ context.Context, userID uint) (string, error) {
	if hash, ok := r.passwords[userID]; ok {
		return hash, nil
	}
	return "", errors.New("not found")
}

func (r *fakeUserRepo) GetSSOAccount(_ context.Context, provider string, providerID string) (auth.User, bool, error) {
	if user, ok := r.ssoAccounts[provider+":"+providerID]; ok {
		return user, true, nil
	}
	return auth.User{}, false, nil
}

func (r *fakeUserRepo) CreateSSOUser(_ context.Context, user auth.User, account auth.SSOAccount) (auth.User, error) {
	created, err := r.RegisterUser(context.Background(), user, "")
	if err != nil {
		return auth.User{}, err
	}
	r.ssoAccounts[account.Provider+":"+account.ProviderID] = created
	return created, nil
}

func (r *fakeUserRepo) LinkSSOAccount(_ context.Context, userID uint, account auth.SSOAccount) error {
	if user, ok := r.byID[userID]; ok {
		r.ssoAccounts[account.Provider+":"+account.ProviderID] = user
		return nil
	}
	return auth.ErrUserNotFound
}

type fakeOTPProvider struct {
	sendCalls  []string
	checkValid bool
	checkErr   error
}

func (p *fakeOTPProvider) SendVerification(_ context.Context, mobile string) error {
	p.sendCalls = append(p.sendCalls, mobile)
	return nil
}

func (p *fakeOTPProvider) CheckVerification(_ context.Context, mobile string, code string) (bool, error) {
	if p.checkErr != nil {
		return false, p.checkErr
	}
	return p.checkValid && mobile != "" && code != "", nil
}

type fakePendingSignupStore struct {
	records          map[string]auth.PendingSignup
	usernameToMobile map[string]string
	emailToMobile    map[string]string
	mobileToMobile   map[string]string
}

func newFakePendingSignupStore() *fakePendingSignupStore {
	return &fakePendingSignupStore{
		records:          map[string]auth.PendingSignup{},
		usernameToMobile: map[string]string{},
		emailToMobile:    map[string]string{},
		mobileToMobile:   map[string]string{},
	}
}

func (s *fakePendingSignupStore) Save(_ context.Context, signup auth.PendingSignup) error {
	s.records[signup.Mobile] = signup
	s.usernameToMobile[signup.Username] = signup.Mobile
	s.emailToMobile[signup.Email] = signup.Mobile
	s.mobileToMobile[signup.Mobile] = signup.Mobile
	return nil
}

func (s *fakePendingSignupStore) GetByMobile(_ context.Context, mobile string) (auth.PendingSignup, error) {
	if signup, ok := s.records[mobile]; ok {
		return signup, nil
	}
	return auth.PendingSignup{}, auth.ErrCacheMiss
}

func (s *fakePendingSignupStore) DeleteByMobile(_ context.Context, mobile string) error {
	signup, ok := s.records[mobile]
	if !ok {
		return nil
	}
	delete(s.records, mobile)
	delete(s.usernameToMobile, signup.Username)
	delete(s.emailToMobile, signup.Email)
	delete(s.mobileToMobile, signup.Mobile)
	return nil
}

func (s *fakePendingSignupStore) GetMobileByUsername(_ context.Context, username string) (string, error) {
	if mobile, ok := s.usernameToMobile[username]; ok {
		return mobile, nil
	}
	return "", auth.ErrCacheMiss
}

func (s *fakePendingSignupStore) GetMobileByEmail(_ context.Context, email string) (string, error) {
	if mobile, ok := s.emailToMobile[email]; ok {
		return mobile, nil
	}
	return "", auth.ErrCacheMiss
}

func (s *fakePendingSignupStore) GetMobileByMobile(_ context.Context, mobile string) (string, error) {
	if stored, ok := s.mobileToMobile[mobile]; ok {
		return stored, nil
	}
	return "", auth.ErrCacheMiss
}

type fakeOTPStateStore struct {
	states map[string]auth.OTPRequestState
}

func newFakeOTPStateStore() *fakeOTPStateStore {
	return &fakeOTPStateStore{states: map[string]auth.OTPRequestState{}}
}

func (s *fakeOTPStateStore) Save(_ context.Context, key string, state auth.OTPRequestState) error {
	s.states[key] = state
	return nil
}

func (s *fakeOTPStateStore) Get(_ context.Context, key string) (auth.OTPRequestState, error) {
	if state, ok := s.states[key]; ok {
		return state, nil
	}
	return auth.OTPRequestState{}, auth.ErrCacheMiss
}

func (s *fakeOTPStateStore) Delete(_ context.Context, key string) error {
	delete(s.states, key)
	return nil
}

type fakeRefreshStore struct {
	sessions map[string]auth.RefreshSession
}

func newFakeRefreshStore() *fakeRefreshStore {
	return &fakeRefreshStore{sessions: map[string]auth.RefreshSession{}}
}

func (s *fakeRefreshStore) Save(_ context.Context, rawToken string, session auth.RefreshSession) error {
	s.sessions[rawToken] = session
	return nil
}

func (s *fakeRefreshStore) Get(_ context.Context, rawToken string) (auth.RefreshSession, error) {
	session, ok := s.sessions[rawToken]
	if !ok {
		return auth.RefreshSession{}, auth.ErrCacheMiss
	}
	return session, nil
}

func (s *fakeRefreshStore) Delete(_ context.Context, rawToken string) error {
	delete(s.sessions, rawToken)
	return nil
}

func newTestService(t *testing.T) (*Service, *fakeUserRepo, *fakeOTPProvider, *fakePendingSignupStore, *fakeOTPStateStore) {
	t.Helper()

	tokenManager, err := auth.NewTokenManager("vesko", "12345678901234567890123456789012", 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}

	repo := &fakeUserRepo{
		byUsername:  map[string]auth.User{},
		byEmail:     map[string]auth.User{},
		byMobile:    map[string]auth.User{},
		byID:        map[uint]auth.User{},
		passwords:   map[uint]string{},
		ssoAccounts: map[string]auth.User{},
	}
	provider := &fakeOTPProvider{checkValid: true}
	pendingStore := newFakePendingSignupStore()
	otpStore := newFakeOTPStateStore()

	service := New(
		repo,
		tokenManager,
		newFakeRefreshStore(),
		provider,
		pendingStore,
		otpStore,
		auth.OTPConfig{
			PendingSignupTTL: 10 * time.Minute,
			ResendCooldown:   time.Minute,
			MaxResends:       5,
		},
		auth.GoogleConfig{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			RedirectURI:  "redirect-uri",
		},
	)

	return service, repo, provider, pendingStore, otpStore
}

func TestRequestSignupOTPSavesPendingSignup(t *testing.T) {
	t.Parallel()

	service, _, provider, pendingStore, _ := newTestService(t)

	err := service.RequestSignupOTP(context.Background(), auth.RegisterRequest{
		Username: "john",
		Password: "Password123",
		Email:    "John@example.com",
		Mobile:   "9642560235",
	})
	if err != nil {
		t.Fatalf("request signup otp: %v", err)
	}

	if len(provider.sendCalls) != 1 || provider.sendCalls[0] != "+919642560235" {
		t.Fatalf("unexpected send calls: %#v", provider.sendCalls)
	}

	signup, err := pendingStore.GetByMobile(context.Background(), "+919642560235")
	if err != nil {
		t.Fatalf("pending signup not saved: %v", err)
	}
	if signup.Email != "john@example.com" {
		t.Fatalf("email not normalized: %q", signup.Email)
	}
	if signup.PasswordHash == "" || signup.PasswordHash == "Password123" {
		t.Fatalf("password hash not stored securely")
	}
}

func TestVerifySignupOTPCreatesVerifiedUser(t *testing.T) {
	t.Parallel()

	service, repo, _, pendingStore, _ := newTestService(t)
	err := pendingStore.Save(context.Background(), auth.PendingSignup{
		Username:     "john",
		PasswordHash: "$2a$10$012345678901234567890u5hQxJ9fP.4sKnP4rjVK6XJ7cP8tw2Ra",
		Email:        "john@example.com",
		Mobile:       "+919642560235",
		LastSentAt:   time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("save pending signup: %v", err)
	}

	user, tokens, err := service.VerifySignupOTP(context.Background(), auth.OTPVerifyRequest{
		Mobile: "+91 96425 60235",
		Code:   "123456",
	})
	if err != nil {
		t.Fatalf("verify signup otp: %v", err)
	}
	if !user.MobileVerified {
		t.Fatalf("expected mobile verified user")
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("expected issued token pair")
	}
	stored, err := repo.GetUserByMobile(context.Background(), "+919642560235")
	if err != nil {
		t.Fatalf("user not persisted: %v", err)
	}
	if !stored.MobileVerified {
		t.Fatalf("stored user not marked verified")
	}
	_, err = pendingStore.GetByMobile(context.Background(), "+919642560235")
	if !errors.Is(err, auth.ErrCacheMiss) {
		t.Fatalf("expected pending signup to be deleted, got %v", err)
	}
}

func TestResendSignupOTPEnforcesCooldown(t *testing.T) {
	t.Parallel()

	service, _, _, pendingStore, _ := newTestService(t)
	err := pendingStore.Save(context.Background(), auth.PendingSignup{
		Username:     "john",
		PasswordHash: "hash",
		Email:        "john@example.com",
		Mobile:       "+919642560235",
		LastSentAt:   time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("save pending signup: %v", err)
	}

	err = service.ResendSignupOTP(context.Background(), auth.OTPRequest{Mobile: "9642560235"})
	if !errors.Is(err, auth.ErrOTPResendTooSoon) {
		t.Fatalf("expected cooldown error, got %v", err)
	}
}

func TestRequestLoginOTPIsGenericForUnknownUser(t *testing.T) {
	t.Parallel()

	service, _, provider, _, _ := newTestService(t)
	err := service.RequestLoginOTP(context.Background(), auth.OTPRequest{Mobile: "9642560235"})
	if err != nil {
		t.Fatalf("expected generic success, got %v", err)
	}
	if len(provider.sendCalls) != 0 {
		t.Fatalf("did not expect otp send for unknown user")
	}
}

func TestVerifyLoginOTPRequiresVerifiedMobile(t *testing.T) {
	t.Parallel()

	service, repo, _, _, _ := newTestService(t)
	repo.byMobile["+919642560235"] = auth.User{
		ID:             1,
		Username:       "john",
		Mobile:         "+919642560235",
		Email:          "john@example.com",
		MobileVerified: false,
	}

	_, _, err := service.VerifyLoginOTP(context.Background(), auth.OTPVerifyRequest{
		Mobile: "9642560235",
		Code:   "123456",
	})
	if !errors.Is(err, auth.ErrMobileNotVerified) {
		t.Fatalf("expected mobile not verified, got %v", err)
	}
}
