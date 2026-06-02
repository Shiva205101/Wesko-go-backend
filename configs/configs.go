package configs

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	vredis "vesko/cache/redis"
	"vesko/dbase"
)

const (
	DatabaseURL                      = "DATABASE_URL"
	DBHost                           = "DB_HOST"
	DBPort                           = "DB_PORT"
	DBUser                           = "DB_USER"
	DBPass                           = "DB_PASS"
	DBName                           = "DB_NAME"
	DBDebug                          = "DB_DEBUG"
	DBSSLMode                        = "DB_SSL_MODE"
	RedisHost                        = "REDIS_HOST"
	RedisPort                        = "REDIS_PORT"
	RedisUser                        = "REDIS_USER"
	RedisPass                        = "REDIS_PASS"
	RedisDB                          = "REDIS_DB"
	RedisCacheTTL                    = "REDIS_CACHE_TTL_SECONDS"
	RedisDialTimeout                 = "REDIS_DIAL_TIMEOUT_SECONDS"
	RedisReadTimeout                 = "REDIS_READ_TIMEOUT_SECONDS"
	RedisWriteTimeout                = "REDIS_WRITE_TIMEOUT_SECONDS"
	RedisPoolTimeout                 = "REDIS_POOL_TIMEOUT_MILLISECONDS"
	HTTPPort                         = "HTTP_PORT"
	Port                             = "PORT"
	HTTPAllowedOrigins               = "HTTP_ALLOWED_ORIGINS"
	AuthIssuer                       = "AUTH_ISSUER"
	AuthJWEKey                       = "AUTH_JWE_KEY"
	AccessTokenTTL                   = "AUTH_ACCESS_TOKEN_TTL_SECONDS"
	RefreshTokenTTL                  = "AUTH_REFRESH_TOKEN_TTL_SECONDS"
	AuthCookieName                   = "AUTH_COOKIE_NAME"
	AuthCookieDomain                 = "AUTH_COOKIE_DOMAIN"
	AuthCookiePath                   = "AUTH_COOKIE_PATH"
	AuthCookieSecure                 = "AUTH_COOKIE_SECURE"
	AuthCookieHTTPOnly               = "AUTH_COOKIE_HTTP_ONLY"
	AuthCookieSameSite               = "AUTH_COOKIE_SAME_SITE"
	TwilioAccountSID                 = "TWILIO_ACCOUNT_SID"
	TwilioAuthToken                  = "TWILIO_AUTH_TOKEN"
	TwilioVerifyServiceSID           = "TWILIO_VERIFY_SERVICE_SID"
	AuthPendingSignupTTL             = "AUTH_PENDING_SIGNUP_TTL_SECONDS"
	AuthOTPResendCooldown            = "AUTH_OTP_RESEND_COOLDOWN_SECONDS"
	AuthOTPMaxResends                = "AUTH_OTP_MAX_RESENDS"
	AuthOTPRequestLimitIP            = "AUTH_OTP_REQUEST_LIMIT_IP"
	AuthOTPRequestLimitWindowSeconds = "AUTH_OTP_REQUEST_LIMIT_WINDOW_SECONDS"
	AuthOTPRequestLimitMobile        = "AUTH_OTP_REQUEST_LIMIT_MOBILE"
	AuthOTPVerifyLimitIP             = "AUTH_OTP_VERIFY_LIMIT_IP"
	AuthOTPVerifyLimitWindowSeconds  = "AUTH_OTP_VERIFY_LIMIT_WINDOW_SECONDS"
	AuthOTPVerifyLimitMobile         = "AUTH_OTP_VERIFY_LIMIT_MOBILE"
	AuthCSRFCookieName               = "AUTH_CSRF_COOKIE_NAME"
	AuthCSRFHeaderName               = "AUTH_CSRF_HEADER_NAME"
	AuthCSRFCookiePath               = "AUTH_CSRF_COOKIE_PATH"
	AuthCSRFCookieSecure             = "AUTH_CSRF_COOKIE_SECURE"
	AuthCSRFCookieSameSite           = "AUTH_CSRF_COOKIE_SAME_SITE"
	GoogleClientID                   = "GOOGLE_CLIENT_ID"
	GoogleClientSecret               = "GOOGLE_CLIENT_SECRET"
	GoogleRedirectURI                = "GOOGLE_REDIRECT_URI"
	EnvConfigFilePath                = "./configs/.env.config"
)

type Configs struct {
	DBConfig    dbase.DBConfig
	RedisConfig vredis.Config
	HTTP        HTTPConfig
	Auth        AuthConfig
}

type HTTPConfig struct {
	Port           string
	AllowedOrigins []string
}

type AuthConfig struct {
	Issuer                 string
	JWEKey                 string
	AccessTokenTTL         time.Duration
	RefreshTokenTTL        time.Duration
	CookieName             string
	CookieDomain           string
	CookiePath             string
	CookieSecure           bool
	CookieHTTPOnly         bool
	CookieSameSite         string
	TwilioAccountSID       string
	TwilioAuthToken        string
	TwilioVerifyServiceSID string
	PendingSignupTTL       time.Duration
	OTPResendCooldown      time.Duration
	OTPMaxResends          int
	OTPRequestLimitIP      int
	OTPRequestLimitWindow  time.Duration
	OTPRequestLimitMobile  int
	OTPVerifyLimitIP       int
	OTPVerifyLimitWindow   time.Duration
	OTPVerifyLimitMobile   int
	CSRFCookieName         string
	CSRFHeaderName         string
	CSRFCookiePath         string
	CSRFCookieSecure       bool
	CSRFCookieSameSite     string
	GoogleClientID         string
	GoogleClientSecret     string
	GoogleRedirectURI      string
}

func getEnvString(env, fallBack string) string {
	val, exists := os.LookupEnv(env)
	if !exists {
		return fallBack
	}
	return val
}

func getFirstEnvString(fallBack string, envs ...string) string {
	for _, env := range envs {
		if value, ok := os.LookupEnv(env); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}

	return fallBack
}

func getEnvBool(env string, fallBack bool) bool {
	b, err := strconv.ParseBool(os.Getenv(env))
	if err != nil {
		return fallBack
	}
	return b
}

func getEnvInt(env string, fallBack int) int {
	i, err := strconv.Atoi(os.Getenv(env))
	if err != nil {
		return fallBack
	}
	return i
}

func getEnvCSV(env string) []string {
	raw := strings.TrimSpace(os.Getenv(env))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}

	return values
}

func loadDBConfig() dbase.DBConfig {
	cfg := dbase.DBConfig{
		Host:     getEnvString(DBHost, "localhost"),
		Port:     getEnvString(DBPort, "5432"),
		User:     getEnvString(DBUser, "postgres"),
		DBName:   getEnvString(DBName, "vesko-backend"),
		Password: getEnvString(DBPass, ""),
		SSLMode:  getEnvString(DBSSLMode, "disable"),
		Debug:    getEnvBool(DBDebug, false),
	}

	rawURL := strings.TrimSpace(os.Getenv(DatabaseURL))
	if rawURL == "" {
		return cfg
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return cfg
	}

	if host := parsed.Hostname(); host != "" {
		cfg.Host = host
	}
	if port := parsed.Port(); port != "" {
		cfg.Port = port
	}
	if parsed.User != nil {
		if user := parsed.User.Username(); user != "" {
			cfg.User = user
		}
		if password, ok := parsed.User.Password(); ok {
			cfg.Password = password
		}
	}
	if dbName := strings.TrimPrefix(parsed.Path, "/"); dbName != "" {
		cfg.DBName = dbName
	}
	if sslmode := parsed.Query().Get("sslmode"); sslmode != "" {
		cfg.SSLMode = sslmode
	}
	if host := strings.TrimSpace(parsed.Query().Get("host")); host != "" {
		cfg.Host = host
	}
	if port := strings.TrimSpace(parsed.Query().Get("port")); port != "" {
		cfg.Port = port
	}

	return cfg
}

func Load() *Configs {
	return &Configs{
		DBConfig: loadDBConfig(),
		RedisConfig: vredis.Config{
			Addr:         net.JoinHostPort(getEnvString(RedisHost, "localhost"), getEnvString(RedisPort, "6379")),
			Username:     getEnvString(RedisUser, ""),
			Password:     getEnvString(RedisPass, ""),
			DB:           getEnvInt(RedisDB, 0),
			CacheTTL:     time.Duration(getEnvInt(RedisCacheTTL, 300)) * time.Second,
			WriteTimeout: time.Duration(getEnvInt(RedisWriteTimeout, 2)) * time.Second,
			ReadTimeout:  time.Duration(getEnvInt(RedisReadTimeout, 1)) * time.Second,
			PoolTimeout:  time.Duration(getEnvInt(RedisPoolTimeout, 500)) * time.Millisecond,
			DialTimeout:  time.Duration(getEnvInt(RedisDialTimeout, 5)) * time.Second,
		},
		HTTP: HTTPConfig{
			Port:           getFirstEnvString("8080", HTTPPort, Port),
			AllowedOrigins: getEnvCSV(HTTPAllowedOrigins),
		},
		Auth: AuthConfig{
			Issuer:                 getEnvString(AuthIssuer, "vesko"),
			JWEKey:                 getEnvString(AuthJWEKey, ""),
			AccessTokenTTL:         time.Duration(getEnvInt(AccessTokenTTL, 900)) * time.Second,
			RefreshTokenTTL:        time.Duration(getEnvInt(RefreshTokenTTL, 86400)) * time.Second,
			CookieName:             getEnvString(AuthCookieName, "refresh_token"),
			CookieDomain:           getEnvString(AuthCookieDomain, ""),
			CookiePath:             getEnvString(AuthCookiePath, "/auth"),
			CookieSecure:           getEnvBool(AuthCookieSecure, false),
			CookieHTTPOnly:         getEnvBool(AuthCookieHTTPOnly, false),
			CookieSameSite:         getEnvString(AuthCookieSameSite, "lax"),
			TwilioAccountSID:       getEnvString(TwilioAccountSID, ""),
			TwilioAuthToken:        getEnvString(TwilioAuthToken, ""),
			TwilioVerifyServiceSID: getEnvString(TwilioVerifyServiceSID, ""),
			PendingSignupTTL:       time.Duration(getEnvInt(AuthPendingSignupTTL, 600)) * time.Second,
			OTPResendCooldown:      time.Duration(getEnvInt(AuthOTPResendCooldown, 60)) * time.Second,
			OTPMaxResends:          getEnvInt(AuthOTPMaxResends, 5),
			OTPRequestLimitIP:      getEnvInt(AuthOTPRequestLimitIP, 10),
			OTPRequestLimitWindow:  time.Duration(getEnvInt(AuthOTPRequestLimitWindowSeconds, 600)) * time.Second,
			OTPRequestLimitMobile:  getEnvInt(AuthOTPRequestLimitMobile, 3),
			OTPVerifyLimitIP:       getEnvInt(AuthOTPVerifyLimitIP, 20),
			OTPVerifyLimitWindow:   time.Duration(getEnvInt(AuthOTPVerifyLimitWindowSeconds, 600)) * time.Second,
			OTPVerifyLimitMobile:   getEnvInt(AuthOTPVerifyLimitMobile, 5),
			CSRFCookieName:         getEnvString(AuthCSRFCookieName, "csrf_token"),
			CSRFHeaderName:         getEnvString(AuthCSRFHeaderName, "X-CSRF-Token"),
			CSRFCookiePath:         getEnvString(AuthCSRFCookiePath, "/auth"),
			CSRFCookieSecure:       getEnvBool(AuthCSRFCookieSecure, true),
			CSRFCookieSameSite:     getEnvString(AuthCSRFCookieSameSite, "lax"),
			GoogleClientID:         getEnvString(GoogleClientID, ""),
			GoogleClientSecret:     getEnvString(GoogleClientSecret, ""),
			GoogleRedirectURI:      getEnvString(GoogleRedirectURI, ""),
		},
	}
}

func (c *Configs) Validate() error {
	switch {
	case c.DBConfig.Host == "":
		return fmt.Errorf("%s is required", DBHost)
	case c.DBConfig.Port == "":
		return fmt.Errorf("%s is required", DBPort)
	case c.DBConfig.User == "":
		return fmt.Errorf("%s is required", DBUser)
	case c.DBConfig.DBName == "":
		return fmt.Errorf("%s is required", DBName)
	case c.DBConfig.Password == "":
		return fmt.Errorf("%s is required", DBPass)
	}

	if _, err := strconv.Atoi(c.DBConfig.Port); err != nil {
		return fmt.Errorf("%s must be a valid port", DBPort)
	}

	switch {
	case c.HTTP.Port == "":
		return fmt.Errorf("%s is required", HTTPPort)
	case c.Auth.Issuer == "":
		return fmt.Errorf("%s is required", AuthIssuer)
	case c.Auth.JWEKey == "":
		return fmt.Errorf("%s is required", AuthJWEKey)
	case len(c.Auth.JWEKey) != 32:
		return fmt.Errorf("%s must be exactly 32 bytes", AuthJWEKey)
	case c.Auth.AccessTokenTTL <= 0:
		return fmt.Errorf("%s must be greater than 0", AccessTokenTTL)
	case c.Auth.RefreshTokenTTL <= 0:
		return fmt.Errorf("%s must be greater than 0", RefreshTokenTTL)
	case c.Auth.CookieName == "":
		return fmt.Errorf("%s is required", AuthCookieName)
	case c.Auth.CookiePath == "":
		return fmt.Errorf("%s is required", AuthCookiePath)
	case c.Auth.CSRFCookieName == "":
		return fmt.Errorf("%s is required", AuthCSRFCookieName)
	case c.Auth.CSRFHeaderName == "":
		return fmt.Errorf("%s is required", AuthCSRFHeaderName)
	case c.Auth.CSRFCookiePath == "":
		return fmt.Errorf("%s is required", AuthCSRFCookiePath)
	case c.Auth.PendingSignupTTL <= 0:
		return fmt.Errorf("%s must be greater than 0", AuthPendingSignupTTL)
	case c.Auth.OTPResendCooldown < 0:
		return fmt.Errorf("%s must be 0 or greater", AuthOTPResendCooldown)
	case c.Auth.OTPMaxResends < 0:
		return fmt.Errorf("%s must be 0 or greater", AuthOTPMaxResends)
	case c.Auth.OTPRequestLimitIP < 0:
		return fmt.Errorf("%s must be 0 or greater", AuthOTPRequestLimitIP)
	case c.Auth.OTPRequestLimitWindow < 0:
		return fmt.Errorf("%s must be 0 or greater", AuthOTPRequestLimitWindowSeconds)
	case c.Auth.OTPRequestLimitMobile < 0:
		return fmt.Errorf("%s must be 0 or greater", AuthOTPRequestLimitMobile)
	case c.Auth.OTPVerifyLimitIP < 0:
		return fmt.Errorf("%s must be 0 or greater", AuthOTPVerifyLimitIP)
	case c.Auth.OTPVerifyLimitWindow < 0:
		return fmt.Errorf("%s must be 0 or greater", AuthOTPVerifyLimitWindowSeconds)
	case c.Auth.OTPVerifyLimitMobile < 0:
		return fmt.Errorf("%s must be 0 or greater", AuthOTPVerifyLimitMobile)
	case c.RedisConfig.Addr == "":
		return errors.New("redis address is required")
	case c.RedisConfig.CacheTTL <= 0:
		return fmt.Errorf("%s must be greater than 0", RedisCacheTTL)
	case c.RedisConfig.DialTimeout <= 0:
		return fmt.Errorf("%s must be greater than 0", RedisDialTimeout)
	case c.RedisConfig.ReadTimeout <= 0:
		return fmt.Errorf("%s must be greater than 0", RedisReadTimeout)
	case c.RedisConfig.WriteTimeout <= 0:
		return fmt.Errorf("%s must be greater than 0", RedisWriteTimeout)
	case c.RedisConfig.PoolTimeout <= 0:
		return fmt.Errorf("%s must be greater than 0", RedisPoolTimeout)
	}

	if _, err := strconv.Atoi(c.HTTP.Port); err != nil {
		return fmt.Errorf("%s must be a valid port", HTTPPort)
	}

	if host, port, err := net.SplitHostPort(c.RedisConfig.Addr); err != nil || host == "" || port == "" {
		return errors.New("redis address must include host and port")
	}

	switch c.Auth.CookieSameSite {
	case "lax", "strict", "none":
	default:
		return fmt.Errorf("%s must be one of lax, strict, none", AuthCookieSameSite)
	}

	switch c.Auth.CSRFCookieSameSite {
	case "lax", "strict", "none":
	default:
		return fmt.Errorf("%s must be one of lax, strict, none", AuthCSRFCookieSameSite)
	}

	return nil
}
