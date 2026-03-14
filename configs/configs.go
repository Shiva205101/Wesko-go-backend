package configs

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	vredis "vesko/cache/redis"
	"vesko/dbase"
)

const (
	DBHost                 = "DB_HOST"
	DBPort                 = "DB_PORT"
	DBUser                 = "DB_USER"
	DBPass                 = "DB_PASS"
	DBName                 = "DB_NAME"
	DBDebug                = "DB_DEBUG"
	DBSSLMode              = "DB_SSL_MODE"
	RedisHost              = "REDIS_HOST"
	RedisPort              = "REDIS_PORT"
	RedisUser              = "REDIS_USER"
	RedisPass              = "REDIS_PASS"
	RedisDB                = "REDIS_DB"
	RedisCacheTTL          = "REDIS_CACHE_TTL_SECONDS"
	RedisDialTimeout       = "REDIS_DIAL_TIMEOUT_SECONDS"
	RedisReadTimeout       = "REDIS_READ_TIMEOUT_SECONDS"
	RedisWriteTimeout      = "REDIS_WRITE_TIMEOUT_SECONDS"
	RedisPoolTimeout       = "REDIS_POOL_TIMEOUT_MILLISECONDS"
	HTTPPort               = "HTTP_PORT"
	AuthIssuer             = "AUTH_ISSUER"
	AuthJWEKey             = "AUTH_JWE_KEY"
	AccessTokenTTL         = "AUTH_ACCESS_TOKEN_TTL_SECONDS"
	RefreshTokenTTL        = "AUTH_REFRESH_TOKEN_TTL_SECONDS"
	AuthCookieName         = "AUTH_COOKIE_NAME"
	AuthCookieDomain       = "AUTH_COOKIE_DOMAIN"
	AuthCookiePath         = "AUTH_COOKIE_PATH"
	AuthCookieSecure       = "AUTH_COOKIE_SECURE"
	AuthCookieHTTPOnly     = "AUTH_COOKIE_HTTP_ONLY"
	AuthCookieSameSite     = "AUTH_COOKIE_SAME_SITE"
	TwilioAccountSID       = "TWILIO_ACCOUNT_SID"
	TwilioAuthToken        = "TWILIO_AUTH_TOKEN"
	TwilioVerifyServiceSID = "TWILIO_VERIFY_SERVICE_SID"
	AuthPendingSignupTTL   = "AUTH_PENDING_SIGNUP_TTL_SECONDS"
	AuthOTPResendCooldown  = "AUTH_OTP_RESEND_COOLDOWN_SECONDS"
	AuthOTPMaxResends      = "AUTH_OTP_MAX_RESENDS"
	EnvConfigFilePath      = "./configs/.env.config"
)

type Configs struct {
	DBConfig    dbase.DBConfig
	RedisConfig vredis.Config
	HTTP        HTTPConfig
	Auth        AuthConfig
}

type HTTPConfig struct {
	Port string
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
}

func getEnvString(env, fallBack string) string {
	val, exists := os.LookupEnv(env)
	if !exists {
		return fallBack
	}
	return val
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

func Load() *Configs {
	return &Configs{
		DBConfig: dbase.DBConfig{
			Host:     getEnvString(DBHost, "localhost"),
			Port:     getEnvString(DBPort, "5432"),
			User:     getEnvString(DBUser, "postgres"),
			DBName:   getEnvString(DBName, "vesko-backend"),
			Password: getEnvString(DBPass, ""),
			SSLMode:  getEnvString(DBSSLMode, "disable"),
			Debug:    getEnvBool(DBDebug, false),
		},
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
			Port: getEnvString(HTTPPort, "8080"),
		},
		Auth: AuthConfig{
			Issuer:                 getEnvString(AuthIssuer, "vesko"),
			JWEKey:                 getEnvString(AuthJWEKey, ""),
			AccessTokenTTL:         time.Duration(getEnvInt(AccessTokenTTL, 900)) * time.Second,
			RefreshTokenTTL:        time.Duration(getEnvInt(RefreshTokenTTL, 86400)) * time.Second,
			CookieName:             getEnvString(AuthCookieName, "refresh_token"),
			CookieDomain:           getEnvString(AuthCookieDomain, ""),
			CookiePath:             getEnvString(AuthCookiePath, "/auth"),
			CookieSecure:           getEnvBool(AuthCookieSecure, true),
			CookieHTTPOnly:         getEnvBool(AuthCookieHTTPOnly, true),
			CookieSameSite:         getEnvString(AuthCookieSameSite, "lax"),
			TwilioAccountSID:       getEnvString(TwilioAccountSID, ""),
			TwilioAuthToken:        getEnvString(TwilioAuthToken, ""),
			TwilioVerifyServiceSID: getEnvString(TwilioVerifyServiceSID, ""),
			PendingSignupTTL:       time.Duration(getEnvInt(AuthPendingSignupTTL, 600)) * time.Second,
			OTPResendCooldown:      time.Duration(getEnvInt(AuthOTPResendCooldown, 60)) * time.Second,
			OTPMaxResends:          getEnvInt(AuthOTPMaxResends, 5),
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
	case c.Auth.TwilioAccountSID == "":
		return fmt.Errorf("%s is required", TwilioAccountSID)
	case c.Auth.TwilioAuthToken == "":
		return fmt.Errorf("%s is required", TwilioAuthToken)
	case c.Auth.TwilioVerifyServiceSID == "":
		return fmt.Errorf("%s is required", TwilioVerifyServiceSID)
	case c.Auth.PendingSignupTTL <= 0:
		return fmt.Errorf("%s must be greater than 0", AuthPendingSignupTTL)
	case c.Auth.OTPResendCooldown < 0:
		return fmt.Errorf("%s must be 0 or greater", AuthOTPResendCooldown)
	case c.Auth.OTPMaxResends < 0:
		return fmt.Errorf("%s must be 0 or greater", AuthOTPMaxResends)
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

	return nil
}
