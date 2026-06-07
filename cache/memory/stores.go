package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"vesko/auth"
)

type RefreshStore struct {
	mu       sync.Mutex
	sessions map[string]auth.RefreshSession
}

func NewRefreshStore() *RefreshStore {
	return &RefreshStore{sessions: map[string]auth.RefreshSession{}}
}

func (s *RefreshStore) Save(_ context.Context, rawToken string, session auth.RefreshSession) error {
	if time.Until(session.ExpiresAt) <= 0 {
		return auth.ErrTokenExpired
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[tokenKey(rawToken)] = session
	return nil
}

func (s *RefreshStore) Get(_ context.Context, rawToken string) (auth.RefreshSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tokenKey(rawToken)
	session, ok := s.sessions[key]
	if !ok {
		return auth.RefreshSession{}, auth.ErrCacheMiss
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		delete(s.sessions, key)
		return auth.RefreshSession{}, auth.ErrCacheMiss
	}

	return session, nil
}

func (s *RefreshStore) Delete(_ context.Context, rawToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, tokenKey(rawToken))
	return nil
}

type PendingSignupStore struct {
	mu         sync.Mutex
	signups    map[string]auth.PendingSignup
	byUsername map[string]string
	byEmail    map[string]string
	byMobile   map[string]string
}

func NewPendingSignupStore() *PendingSignupStore {
	return &PendingSignupStore{
		signups:    map[string]auth.PendingSignup{},
		byUsername: map[string]string{},
		byEmail:    map[string]string{},
		byMobile:   map[string]string{},
	}
}

func (s *PendingSignupStore) Save(_ context.Context, signup auth.PendingSignup) error {
	if time.Until(signup.ExpiresAt) <= 0 {
		return auth.ErrPendingSignupExpired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.signups[signup.Mobile]; ok {
		s.deleteLocked(existing.Mobile)
	}
	s.signups[signup.Mobile] = signup
	s.byUsername[signup.Username] = signup.Mobile
	s.byEmail[signup.Email] = signup.Mobile
	s.byMobile[signup.Mobile] = signup.Mobile
	return nil
}

func (s *PendingSignupStore) GetByMobile(_ context.Context, mobile string) (auth.PendingSignup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getByMobileLocked(mobile)
}

func (s *PendingSignupStore) DeleteByMobile(_ context.Context, mobile string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteLocked(mobile)
	return nil
}

func (s *PendingSignupStore) GetMobileByUsername(_ context.Context, username string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getIndexLocked(s.byUsername, username)
}

func (s *PendingSignupStore) GetMobileByEmail(_ context.Context, email string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getIndexLocked(s.byEmail, email)
}

func (s *PendingSignupStore) GetMobileByMobile(_ context.Context, mobile string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getIndexLocked(s.byMobile, mobile)
}

func (s *PendingSignupStore) getIndexLocked(index map[string]string, key string) (string, error) {
	mobile, ok := index[key]
	if !ok {
		return "", auth.ErrCacheMiss
	}
	if _, err := s.getByMobileLocked(mobile); err != nil {
		return "", err
	}
	return mobile, nil
}

func (s *PendingSignupStore) getByMobileLocked(mobile string) (auth.PendingSignup, error) {
	signup, ok := s.signups[mobile]
	if !ok {
		return auth.PendingSignup{}, auth.ErrCacheMiss
	}
	if time.Now().UTC().After(signup.ExpiresAt) {
		s.deleteLocked(mobile)
		return auth.PendingSignup{}, auth.ErrCacheMiss
	}
	return signup, nil
}

func (s *PendingSignupStore) deleteLocked(mobile string) {
	signup, ok := s.signups[mobile]
	if !ok {
		return
	}
	delete(s.signups, mobile)
	delete(s.byUsername, signup.Username)
	delete(s.byEmail, signup.Email)
	delete(s.byMobile, signup.Mobile)
}

type OTPStateStore struct {
	mu     sync.Mutex
	states map[string]auth.OTPRequestState
}

func NewOTPStateStore() *OTPStateStore {
	return &OTPStateStore{states: map[string]auth.OTPRequestState{}}
}

func (s *OTPStateStore) Save(_ context.Context, key string, state auth.OTPRequestState) error {
	if time.Until(state.ExpiresAt) <= 0 {
		return auth.ErrTokenExpired
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[key] = state
	return nil
}

func (s *OTPStateStore) Get(_ context.Context, key string) (auth.OTPRequestState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.states[key]
	if !ok {
		return auth.OTPRequestState{}, auth.ErrCacheMiss
	}
	if time.Now().UTC().After(state.ExpiresAt) {
		delete(s.states, key)
		return auth.OTPRequestState{}, auth.ErrCacheMiss
	}

	return state, nil
}

func (s *OTPStateStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, key)
	return nil
}

type RateLimiter struct {
	mu      sync.Mutex
	windows map[string]rateLimitWindow
}

type rateLimitWindow struct {
	count     int
	expiresAt time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{windows: map[string]rateLimitWindow{}}
}

func (l *RateLimiter) Allow(_ context.Context, key string, rule auth.RateLimitRule) (bool, error) {
	if rule.Limit <= 0 || rule.Window <= 0 {
		return true, nil
	}

	now := time.Now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()

	window := l.windows[key]
	if window.expiresAt.IsZero() || now.After(window.expiresAt) {
		window = rateLimitWindow{expiresAt: now.Add(rule.Window)}
	}
	window.count++
	l.windows[key] = window

	return window.count <= rule.Limit, nil
}

func tokenKey(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
