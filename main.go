package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"vesko/auth"
	authhttp "vesko/auth/http"
	twilioprovider "vesko/auth/provider/twilio"
	authservice "vesko/auth/service"
	rediscache "vesko/cache/redis"
	"vesko/configs"
	"vesko/dbase/userdao"
	"vesko/logger"
	"vesko/server"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	env, exists := os.LookupEnv("ENV")
	if !exists {
		env = "dev"
	}

	appLogger := logger.New("vesko-auth", env)

	if env == "dev" {
		err := godotenv.Load(configs.EnvConfigFilePath)
		if err != nil {
			fatal(appLogger, "failed to load env file", err)
		}
	}

	if env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	envConfigs := configs.Load()
	if err := envConfigs.Validate(); err != nil {
		fatal(appLogger, "invalid configuration", err)
	}

	appLogger.Info("configuration loaded",
		"env", env,
		"http_port", envConfigs.HTTP.Port,
		"db_host", envConfigs.DBConfig.Host,
		"db_name", envConfigs.DBConfig.DBName,
		"redis_addr", envConfigs.RedisConfig.Addr,
		"redis_db", envConfigs.RedisConfig.DB,
		"access_token_ttl_seconds", int64(envConfigs.Auth.AccessTokenTTL.Seconds()),
		"refresh_token_ttl_seconds", int64(envConfigs.Auth.RefreshTokenTTL.Seconds()),
	)

	db, err := envConfigs.DBConfig.DBConn()
	if err != nil {
		fatal(appLogger, "database connection failed", err)
	}

	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithTimeout(appCtx, 10*time.Second)
	defer cancel()

	pgRepo := userdao.NewPostgresRepository(db)
	if err = pgRepo.AutoMigrate(ctx); err != nil {
		fatal(appLogger, "database migration failed", err)
	}

	redisClient := envConfigs.RedisConfig.NewClient()

	if err := rediscache.Ping(ctx, redisClient); err != nil {
		fatal(appLogger, "redis connection failed", err)
	}

	tokenManager, err := auth.NewTokenManager(
		envConfigs.Auth.Issuer,
		envConfigs.Auth.JWEKey,
		envConfigs.Auth.AccessTokenTTL,
		envConfigs.Auth.RefreshTokenTTL,
	)
	if err != nil {
		fatal(appLogger, "token manager initialization failed", err)
	}

	cachedRepo := auth.NewCachedUserRepository(
		pgRepo,
		rediscache.NewUserCache(redisClient, envConfigs.RedisConfig.CacheTTL),
	)
	otpProvider := twilioprovider.NewVerifyProvider(
		envConfigs.Auth.TwilioAccountSID,
		envConfigs.Auth.TwilioAuthToken,
		envConfigs.Auth.TwilioVerifyServiceSID,
		nil,
	)

	authService := authservice.New(
		cachedRepo,
		tokenManager,
		rediscache.NewRefreshStore(redisClient),
		otpProvider,
		rediscache.NewPendingSignupStore(redisClient),
		rediscache.NewOTPStateStore(redisClient),
		auth.OTPConfig{
			PendingSignupTTL: envConfigs.Auth.PendingSignupTTL,
			ResendCooldown:   envConfigs.Auth.OTPResendCooldown,
			MaxResends:       envConfigs.Auth.OTPMaxResends,
		},
		auth.GoogleConfig{
			ClientID:     envConfigs.Auth.GoogleClientID,
			ClientSecret: envConfigs.Auth.GoogleClientSecret,
			RedirectURI:  envConfigs.Auth.GoogleRedirectURI,
		},
	)
	rateLimiter := rediscache.NewRateLimiter(redisClient)
	cookieSameSite := http.SameSiteLaxMode
	switch envConfigs.Auth.CookieSameSite {
	case "strict":
		cookieSameSite = http.SameSiteStrictMode
	case "none":
		cookieSameSite = http.SameSiteNoneMode
	}
	authHandler := authhttp.NewWithLimiter(authService, appLogger, authhttp.CookieConfig{
		RefreshTokenName:   envConfigs.Auth.CookieName,
		Domain:             envConfigs.Auth.CookieDomain,
		Path:               envConfigs.Auth.CookiePath,
		Secure:             envConfigs.Auth.CookieSecure,
		HTTPOnly:           envConfigs.Auth.CookieHTTPOnly,
		SameSite:           cookieSameSite,
		CSRFCookieName:     envConfigs.Auth.CSRFCookieName,
		CSRFHeaderName:     envConfigs.Auth.CSRFHeaderName,
		CSRFCookiePath:     envConfigs.Auth.CSRFCookiePath,
		CSRFCookieSecure:   envConfigs.Auth.CSRFCookieSecure,
		CSRFCookieSameSite: mapSameSite(envConfigs.Auth.CSRFCookieSameSite),
	}, rateLimiter, auth.OTPRateLimitConfig{
		RequestIP: auth.RateLimitRule{
			Limit:  envConfigs.Auth.OTPRequestLimitIP,
			Window: envConfigs.Auth.OTPRequestLimitWindow,
		},
		RequestMobile: auth.RateLimitRule{
			Limit:  envConfigs.Auth.OTPRequestLimitMobile,
			Window: envConfigs.Auth.OTPRequestLimitWindow,
		},
		VerifyIP: auth.RateLimitRule{
			Limit:  envConfigs.Auth.OTPVerifyLimitIP,
			Window: envConfigs.Auth.OTPVerifyLimitWindow,
		},
		VerifyMobile: auth.RateLimitRule{
			Limit:  envConfigs.Auth.OTPVerifyLimitMobile,
			Window: envConfigs.Auth.OTPVerifyLimitWindow,
		},
	})
	httpServer := server.New(server.Config{
		Addr:            ":" + envConfigs.HTTP.Port,
		AllowedOrigins:  envConfigs.HTTP.AllowedOrigins,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		BaseContext:     appCtx,
		Logger:          appLogger,
	}, authHandler)

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- httpServer.Start()
	}()

	select {
	case <-appCtx.Done():
		appLogger.Info("shutdown signal received", "reason", appCtx.Err())
	case err := <-serverErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal(appLogger, "http server failed", err)
		}
		return
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		fatal(appLogger, "http server shutdown failed", err)
	}

}

func mapSameSite(value string) http.SameSite {
	switch value {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func fatal(logger *slog.Logger, message string, err error) {
	logger.Error(message, "error", err.Error())
	os.Exit(1)
}
