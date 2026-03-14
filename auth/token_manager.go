package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	jose "gopkg.in/square/go-jose.v2"
)

type TokenManager struct {
	issuer          string
	jweKey          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewTokenManager(issuer string, jweKey string, accessTokenTTL time.Duration, refreshTokenTTL time.Duration) (*TokenManager, error) {
	key := []byte(jweKey)
	if len(key) != 32 {
		return nil, ErrInvalidJWEKey
	}

	return &TokenManager{
		issuer:          issuer,
		jweKey:          key,
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}, nil
}

func (m *TokenManager) GenerateTokenPair(user User) (TokenPair, RefreshSession, error) {
	now := time.Now().UTC()
	claims := AccessClaims{
		Subject:   user.Username,
		TokenType: "access",
		IssuedAt:  now,
		ExpiresAt: now.Add(m.accessTokenTTL),
	}

	accessToken, err := m.encrypt(claims)
	if err != nil {
		return TokenPair{}, RefreshSession{}, err
	}

	rawRefreshToken, err := randomToken(32)
	if err != nil {
		return TokenPair{}, RefreshSession{}, err
	}

	tokenID, err := randomToken(24)
	if err != nil {
		return TokenPair{}, RefreshSession{}, err
	}

	session := RefreshSession{
		TokenID:   tokenID,
		UserID:    user.ID,
		Username:  user.Username,
		ExpiresAt: now.Add(m.refreshTokenTTL),
	}

	return TokenPair{
			AccessToken:           accessToken,
			RefreshToken:          fmt.Sprintf("%s.%s", tokenID, rawRefreshToken),
			TokenType:             "Bearer",
			AccessTokenExpiresIn:  int64(m.accessTokenTTL.Seconds()),
			RefreshTokenExpiresIn: int64(m.refreshTokenTTL.Seconds()),
		},
		session,
		nil
}

func (m *TokenManager) ParseAccessToken(token string) (AccessClaims, error) {
	var claims AccessClaims
	if err := m.decrypt(token, &claims); err != nil {
		return AccessClaims{}, ErrInvalidToken
	}

	if claims.TokenType != "access" {
		return AccessClaims{}, ErrInvalidToken
	}

	if time.Now().UTC().After(claims.ExpiresAt) {
		return AccessClaims{}, ErrTokenExpired
	}

	return claims, nil
}

func (m *TokenManager) encrypt(payload any) (string, error) {
	encrypter, err := jose.NewEncrypter(jose.A256GCM, jose.Recipient{
		Algorithm: jose.DIRECT,
		Key:       m.jweKey,
	}, (&jose.EncrypterOptions{}).WithType("JWE").WithContentType("json"))
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	object, err := encrypter.Encrypt(data)
	if err != nil {
		return "", err
	}

	return object.CompactSerialize()
}

func (m *TokenManager) decrypt(token string, out any) error {
	object, err := jose.ParseEncrypted(token)
	if err != nil {
		return err
	}

	data, err := object.Decrypt(m.jweKey)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, out)
}

func randomToken(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}
