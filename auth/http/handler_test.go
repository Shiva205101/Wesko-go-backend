package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vesko/auth"
	authservice "vesko/auth/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type fakeUserRepo struct {
	byUsername  map[string]auth.User
	byEmail     map[string]auth.User
	byMobile    map[string]auth.User
	byID        map[uint]auth.User
	passwords   map[uint]string
	ssoAccounts map[string]auth.User
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
	return "", auth.ErrUserNotFound
}

func (r *fakeUserRepo) GetSSOAccount(_ context.Context, provider string, providerID string) (auth.User, bool, error) {
	user, ok := r.ssoAccounts[provider+":"+providerID]
	return user, ok, nil
}

func (r *fakeUserRepo) CreateSSOUser(_ context.Context, user auth.User, account auth.SSOAccount) (auth.User, error) {
	user.ID = uint(len(r.byUsername) + 1)
	r.byUsername[user.Username] = user
	r.byEmail[user.Email] = user
	if user.Mobile != "" {
		r.byMobile[user.Mobile] = user
	}
	r.byID[user.ID] = user
	r.ssoAccounts[account.Provider+":"+account.ProviderID] = user
	return user, nil
}

func (r *fakeUserRepo) LinkSSOAccount(_ context.Context, userID uint, account auth.SSOAccount) error {
	user, ok := r.byID[userID]
	if !ok {
		return auth.ErrUserNotFound
	}
	r.ssoAccounts[account.Provider+":"+account.ProviderID] = user
	return nil
}

type fakeOTPProvider struct{}

func (p *fakeOTPProvider) SendVerification(_ context.Context, _ string) error {
	return nil
}

func (p *fakeOTPProvider) CheckVerification(_ context.Context, _ string, _ string) (bool, error) {
	return true, nil
}

type fakePendingSignupStore struct{}

func (s *fakePendingSignupStore) Save(_ context.Context, _ auth.PendingSignup) error { return nil }
func (s *fakePendingSignupStore) GetByMobile(_ context.Context, _ string) (auth.PendingSignup, error) {
	return auth.PendingSignup{}, auth.ErrCacheMiss
}
func (s *fakePendingSignupStore) DeleteByMobile(_ context.Context, _ string) error { return nil }
func (s *fakePendingSignupStore) GetMobileByUsername(_ context.Context, _ string) (string, error) {
	return "", auth.ErrCacheMiss
}
func (s *fakePendingSignupStore) GetMobileByEmail(_ context.Context, _ string) (string, error) {
	return "", auth.ErrCacheMiss
}
func (s *fakePendingSignupStore) GetMobileByMobile(_ context.Context, _ string) (string, error) {
	return "", auth.ErrCacheMiss
}

type fakeOTPStateStore struct{}

func (s *fakeOTPStateStore) Save(_ context.Context, _ string, _ auth.OTPRequestState) error {
	return nil
}
func (s *fakeOTPStateStore) Get(_ context.Context, _ string) (auth.OTPRequestState, error) {
	return auth.OTPRequestState{}, auth.ErrCacheMiss
}
func (s *fakeOTPStateStore) Delete(_ context.Context, _ string) error { return nil }

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

type fakeRateLimiter struct {
	denyKeys map[string]bool
}

func (l *fakeRateLimiter) Allow(_ context.Context, key string, _ auth.RateLimitRule) (bool, error) {
	if l.denyKeys[key] {
		return false, nil
	}
	return true, nil
}

func newTestHTTPHandler(t *testing.T) *Handler {
	return newTestHTTPHandlerWithLimiter(t, nil)
}

func newTestHTTPHandlerWithLimiter(t *testing.T, limiter auth.RateLimiter) *Handler {
	t.Helper()

	tokenManager, err := auth.NewTokenManager("vesko", "12345678901234567890123456789012", 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}

	passwordBytes, err := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("password hash: %v", err)
	}
	passwordHash := string(passwordBytes)
	repo := &fakeUserRepo{
		byUsername: map[string]auth.User{
			"john": {
				ID:                1,
				Username:          "john",
				Email:             "john@example.com",
				Mobile:            "+919642560235",
				MobileVerified:    true,
				Role:              auth.RoleCustomer,
				IsProfileComplete: true,
			},
		},
		byEmail: map[string]auth.User{
			"john@example.com": {
				ID:                1,
				Username:          "john",
				Email:             "john@example.com",
				Mobile:            "+919642560235",
				MobileVerified:    true,
				Role:              auth.RoleCustomer,
				IsProfileComplete: true,
			},
		},
		byMobile: map[string]auth.User{
			"+919642560235": {
				ID:                1,
				Username:          "john",
				Email:             "john@example.com",
				Mobile:            "+919642560235",
				MobileVerified:    true,
				Role:              auth.RoleCustomer,
				IsProfileComplete: true,
			},
		},
		byID: map[uint]auth.User{
			1: {
				ID:                1,
				Username:          "john",
				Email:             "john@example.com",
				Mobile:            "+919642560235",
				MobileVerified:    true,
				Role:              auth.RoleCustomer,
				IsProfileComplete: true,
			},
		},
		passwords: map[uint]string{
			1: passwordHash,
		},
		ssoAccounts: map[string]auth.User{},
	}

	svc := authservice.New(
		repo,
		tokenManager,
		newFakeRefreshStore(),
		&fakeOTPProvider{},
		&fakePendingSignupStore{},
		&fakeOTPStateStore{},
		auth.OTPConfig{
			PendingSignupTTL: 10 * time.Minute,
			ResendCooldown:   time.Minute,
			MaxResends:       5,
		},
		auth.GoogleConfig{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			RedirectURI:  "http://localhost:8080/auth/google/callback",
		},
	)

	return NewWithLimiter(svc, nil, CookieConfig{}, limiter, auth.OTPRateLimitConfig{
		RequestIP:     auth.RateLimitRule{Limit: 10, Window: 10 * time.Minute},
		RequestMobile: auth.RateLimitRule{Limit: 3, Window: 10 * time.Minute},
		VerifyIP:      auth.RateLimitRule{Limit: 20, Window: 10 * time.Minute},
		VerifyMobile:  auth.RateLimitRule{Limit: 5, Window: 10 * time.Minute},
	})
}

func newJSONRequest(t *testing.T, method string, path string, body any) *http.Request {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestPasswordLoginWebSetsCookiesAndOmitsRefreshToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestHTTPHandler(t)

	router := gin.New()
	handler.RegisterRoutes(router)

	req := newJSONRequest(t, http.MethodPost, "/auth/login", map[string]string{
		"username":    "john",
		"password":    "Password123",
		"client_type": "web",
	})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	tokens := resp["tokens"].(map[string]any)
	if _, ok := tokens["refresh_token"]; ok {
		t.Fatalf("expected refresh token omitted from json for web client")
	}

	cookies := rr.Result().Cookies()
	var hasRefresh, hasCSRF bool
	for _, cookie := range cookies {
		if cookie.Name == "refresh_token" {
			hasRefresh = true
		}
		if cookie.Name == "csrf_token" {
			hasCSRF = true
		}
	}
	if !hasRefresh || !hasCSRF {
		t.Fatalf("expected refresh and csrf cookies, got %#v", cookies)
	}
}

func TestRefreshWebRequiresCSRFFromCookieAndHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestHTTPHandler(t)
	router := gin.New()
	handler.RegisterRoutes(router)

	loginReq := newJSONRequest(t, http.MethodPost, "/auth/login", map[string]string{
		"username":    "john",
		"password":    "Password123",
		"client_type": "web",
	})
	loginRR := httptest.NewRecorder()
	router.ServeHTTP(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		t.Fatalf("login failed: %d", loginRR.Code)
	}

	var refreshCookie, csrfCookie *http.Cookie
	for _, cookie := range loginRR.Result().Cookies() {
		switch cookie.Name {
		case "refresh_token":
			refreshCookie = cookie
		case "csrf_token":
			csrfCookie = cookie
		}
	}

	req := newJSONRequest(t, http.MethodPost, "/auth/refresh", map[string]string{
		"client_type": "web",
	})
	req.AddCookie(refreshCookie)
	req.AddCookie(csrfCookie)
	req.Header.Set("X-CSRF-Token", csrfCookie.Value)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	reqMissingCSRF := newJSONRequest(t, http.MethodPost, "/auth/refresh", map[string]string{
		"client_type": "web",
	})
	reqMissingCSRF.AddCookie(refreshCookie)
	reqMissingCSRF.AddCookie(csrfCookie)

	rrMissing := httptest.NewRecorder()
	router.ServeHTTP(rrMissing, reqMissingCSRF)
	if rrMissing.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without csrf header, got %d", rrMissing.Code)
	}
}

func TestPasswordLoginMobileReturnsRefreshTokenInJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestHTTPHandler(t)
	router := gin.New()
	handler.RegisterRoutes(router)

	req := newJSONRequest(t, http.MethodPost, "/auth/login", map[string]string{
		"username":    "john",
		"password":    "Password123",
		"client_type": "android",
	})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	tokens := resp["tokens"].(map[string]any)
	if _, ok := tokens["refresh_token"]; !ok {
		t.Fatalf("expected refresh token in json for mobile client")
	}
}

func TestLogoutWebRequiresCSRFFromCookieAndHeaderAndClearsCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestHTTPHandler(t)
	router := gin.New()
	handler.RegisterRoutes(router)

	loginReq := newJSONRequest(t, http.MethodPost, "/auth/login", map[string]string{
		"username":    "john",
		"password":    "Password123",
		"client_type": "web",
	})
	loginRR := httptest.NewRecorder()
	router.ServeHTTP(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		t.Fatalf("login failed: %d", loginRR.Code)
	}

	var refreshCookie, csrfCookie *http.Cookie
	for _, cookie := range loginRR.Result().Cookies() {
		switch cookie.Name {
		case "refresh_token":
			refreshCookie = cookie
		case "csrf_token":
			csrfCookie = cookie
		}
	}

	logoutReq := newJSONRequest(t, http.MethodPost, "/auth/logout", map[string]string{
		"client_type": "web",
	})
	logoutReq.AddCookie(refreshCookie)
	logoutReq.AddCookie(csrfCookie)
	logoutReq.Header.Set("X-CSRF-Token", csrfCookie.Value)

	logoutRR := httptest.NewRecorder()
	router.ServeHTTP(logoutRR, logoutReq)
	if logoutRR.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", logoutRR.Code)
	}

	var clearedRefresh, clearedCSRF bool
	for _, cookie := range logoutRR.Result().Cookies() {
		if cookie.MaxAge < 0 && cookie.Name == "refresh_token" {
			clearedRefresh = true
		}
		if cookie.MaxAge < 0 && cookie.Name == "csrf_token" {
			clearedCSRF = true
		}
	}
	if !clearedRefresh || !clearedCSRF {
		t.Fatalf("expected refresh and csrf cookies to be cleared")
	}

	missingCSRFReq := newJSONRequest(t, http.MethodPost, "/auth/logout", map[string]string{
		"client_type": "web",
	})
	missingCSRFReq.AddCookie(refreshCookie)
	missingCSRFReq.AddCookie(csrfCookie)

	missingCSRFRR := httptest.NewRecorder()
	router.ServeHTTP(missingCSRFRR, missingCSRFReq)
	if missingCSRFRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without csrf header, got %d", missingCSRFRR.Code)
	}
}

func TestOTPRequestReturnsTooManyRequestsWhenRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestHTTPHandlerWithLimiter(t, &fakeRateLimiter{
		denyKeys: map[string]bool{
			"otp:request:ip:192.0.2.1": true,
		},
	})
	router := gin.New()
	handler.RegisterRoutes(router)

	req := newJSONRequest(t, http.MethodPost, "/auth/login/request-otp", map[string]string{
		"mobile":      "9642560235",
		"client_type": "web",
	})
	req.RemoteAddr = "192.0.2.1:1234"

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
}
