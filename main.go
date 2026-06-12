package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"vesko/auth"
	authhttp "vesko/auth/http"
	twilioprovider "vesko/auth/provider/twilio"
	authservice "vesko/auth/service"
	memorycache "vesko/cache/memory"
	rediscache "vesko/cache/redis"
	cataloghttp "vesko/catalog/http"
	"vesko/catalog/productdao"
	catalogservice "vesko/catalog/service"
	"vesko/configs"
	"vesko/dbase/userdao"
	"vesko/internal/observability"
	"vesko/logger"
	"vesko/server"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	env := normalizeEnv(os.Getenv("ENV"))

	appLogger := logger.New("wesko-api", env)

	if isDotEnvEnvironment(env) {
		err := godotenv.Load(configs.EnvConfigFilePath)
		if err != nil {
			fatal(appLogger, "failed to load env file", err)
		}
	}

	if isReleaseEnvironment(env) {
		gin.SetMode(gin.ReleaseMode)
	}
	observability.Configure("wesko-api")

	envConfigs := configs.Load()
	if err := envConfigs.Validate(); err != nil {
		fatal(appLogger, "invalid configuration", err)
	}

	appLogger.Info("configuration loaded",
		"env", env,
		"http_port", envConfigs.HTTP.Port,
		"db_host", envConfigs.DBConfig.Host,
		"db_name", envConfigs.DBConfig.DBName,
		"redis_enabled", envConfigs.RedisConfig.Enabled,
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
	catalogRepo := productdao.NewPostgresRepository(db)
	if err = catalogRepo.AutoMigrate(ctx); err != nil {
		// Catalog schema is migration-managed; avoid blocking startup on
		// AutoMigrate drift against pre-existing SQL constraints.
		appLogger.Warn("catalog auto-migrate failed; continuing with existing schema",
			"error", err.Error(),
			"hint", "apply SQL migrations under dbase/migrations",
		)
	}

	sqlDB, err := db.DB()
	if err != nil {
		fatal(appLogger, "database sql handle failed", err)
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

	userRepo := auth.UserRepository(pgRepo)
	refreshStore := auth.RefreshTokenStore(memorycache.NewRefreshStore())
	pendingSignupStore := auth.PendingSignupStore(memorycache.NewPendingSignupStore())
	otpStateStore := auth.OTPRequestStateStore(memorycache.NewOTPStateStore())
	rateLimiter := auth.RateLimiter(memorycache.NewRateLimiter())
	var redisReadinessCheck func(context.Context) error

	if envConfigs.RedisConfig.Enabled {
		redisClient := envConfigs.RedisConfig.NewClient()
		if err := rediscache.Ping(ctx, redisClient); err != nil {
			fatal(appLogger, "redis connection failed", err)
		}

		userRepo = auth.NewCachedUserRepository(
			pgRepo,
			rediscache.NewUserCache(redisClient, envConfigs.RedisConfig.CacheTTL),
		)
		refreshStore = rediscache.NewRefreshStore(redisClient)
		pendingSignupStore = rediscache.NewPendingSignupStore(redisClient)
		otpStateStore = rediscache.NewOTPStateStore(redisClient)
		rateLimiter = rediscache.NewRateLimiter(redisClient)
		redisReadinessCheck = func(ctx context.Context) error {
			return rediscache.Ping(ctx, redisClient)
		}
		appLogger.Info("redis enabled; using redis-backed auth cache and state")
	} else {
		appLogger.Info("redis disabled; using in-memory auth state and postgres user repository")
	}

	otpProvider := twilioprovider.NewVerifyProvider(
		envConfigs.Auth.TwilioAccountSID,
		envConfigs.Auth.TwilioAuthToken,
		envConfigs.Auth.TwilioVerifyServiceSID,
		nil,
	)

	authService := authservice.New(
		userRepo,
		tokenManager,
		refreshStore,
		otpProvider,
		pendingSignupStore,
		otpStateStore,
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
		appLogger,
	)
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
	catalogSvc := catalogservice.New(catalogRepo, appLogger)
	catalogHandler := cataloghttp.New(catalogSvc, appLogger)

	httpServer := server.New(server.Config{
		Addr:            ":" + envConfigs.HTTP.Port,
		AllowedOrigins:  envConfigs.HTTP.AllowedOrigins,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		BaseContext:     appCtx,
		ServiceName:     "wesko-api",
		Environment:     env,
		ReadinessCheck: func(ctx context.Context) error {
			if err := sqlDB.PingContext(ctx); err != nil {
				return err
			}

			if redisReadinessCheck != nil {
				return redisReadinessCheck(ctx)
			}

			return nil
		},
		Logger: appLogger,
	}, routeRegistrar{
		registrars: []server.RouteRegistrar{
			authHandler,
			catalogHandler,
		},
	})

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

func normalizeEnv(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "dev", "development":
		return "development"
	case "local":
		return "local"
	case "stage", "staging":
		return "staging"
	case "prod", "production":
		return "production"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func isDotEnvEnvironment(env string) bool {
	return env == "local" || env == "development"
}

func isReleaseEnvironment(env string) bool {
	return env == "staging" || env == "production"
}

func fatal(logger *slog.Logger, message string, err error) {
	logger.Error(message, "error", err.Error())
	os.Exit(1)
}

type routeRegistrar struct {
	registrars []server.RouteRegistrar
}

func (r routeRegistrar) RegisterRoutes(router gin.IRouter) {
	for _, registrar := range r.registrars {
		if registrar == nil {
			continue
		}
		registrar.RegisterRoutes(router)
	}
}
