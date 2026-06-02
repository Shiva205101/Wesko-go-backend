package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"vesko/internal/observability"
	applogger "vesko/logger"
	"vesko/pkg/buildinfo"
	"vesko/requestctx"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

type RouteRegistrar interface {
	RegisterRoutes(router gin.IRouter)
}

type Config struct {
	Addr            string
	AllowedOrigins  []string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	BaseContext     context.Context
	ServiceName     string
	Environment     string
	ReadinessCheck  func(context.Context) error
	Logger          *slog.Logger
}

type HTTPServer struct {
	server *http.Server
	logger *slog.Logger
}

func New(cfg Config, routeRegistrar RouteRegistrar) *HTTPServer {
	loggerInternal := cfg.Logger
	if loggerInternal == nil {
		loggerInternal = slog.Default()
	}
	serverLogger := loggerInternal.With("component", "http_server")
	baseContext := cfg.BaseContext
	if baseContext == nil {
		baseContext = context.Background()
	}

	router := gin.New()
	router.Use(
		requestContextMiddleware(serverLogger),
		observability.HTTPMiddleware(cfg.ServiceName),
		traceIDMiddleware(),
		corsMiddleware(cfg.AllowedOrigins),
		accessLogMiddleware(),
		gin.CustomRecovery(func(c *gin.Context, recovered any) {
			l := applogger.FromContext(c.Request.Context())
			l.Error("panic recovered",
				"method", c.Request.Method,
				"path", c.FullPath(),
				"error", fmt.Sprint(recovered),
			)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}),
	)

	healthResponse := gin.H{
		"status":     "ok",
		"service":    cfg.ServiceName,
		"env":        cfg.Environment,
		"version":    buildinfo.Version,
		"commit":     buildinfo.Commit,
		"build_date": buildinfo.BuildDate,
	}
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, healthResponse)
	})
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, healthResponse)
	})
	router.GET("/ready", func(c *gin.Context) {
		if cfg.ReadinessCheck != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()

			if err := cfg.ReadinessCheck(ctx); err != nil {
				serverLogger.Warn("readiness check failed", "error", err.Error())
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	routeRegistrar.RegisterRoutes(router)

	return &HTTPServer{
		server: &http.Server{
			Addr: cfg.Addr,
			BaseContext: func(_ net.Listener) context.Context {
				return baseContext
			},
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		logger: serverLogger,
	}
}

func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Vary", "Origin")
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, X-CSRF-Token")
				c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			}
		}

		if c.Request.Method == http.MethodOptions {
			if origin == "" {
				c.Status(http.StatusNoContent)
				return
			}
			if _, ok := allowed[origin]; ok {
				c.Status(http.StatusNoContent)
				return
			}

			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}

func (s *HTTPServer) Start() error {
	s.logger.Info("http server starting", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.logger.Info("http server shutting down")
	return s.server.Shutdown(ctx)
}

func traceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		l := applogger.FromContext(ctx)

		// Extract trace ID from OTel span
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			traceID := span.SpanContext().TraceID().String()
			// Re-bind logger with trace_id
			l = l.With("trace_id", traceID)
			c.Request = c.Request.WithContext(applogger.ToContext(ctx, l))
		}

		c.Next()
	}
}

func requestContextMiddleware(l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now().UTC()
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}

		ctx := requestctx.WithMetadata(c.Request.Context(), requestctx.Metadata{
			RequestID:    requestID,
			RequestStart: start,
			TraceParent:  strings.TrimSpace(c.GetHeader("traceparent")),
			TraceState:   strings.TrimSpace(c.GetHeader("tracestate")),
		})

		// Bind IDs to logger so they appear even without InfoContext
		lWithID := l.With("request_id", requestID)

		// Inject logger into context
		ctx = applogger.ToContext(ctx, lWithID)

		c.Request = c.Request.WithContext(ctx)
		c.Request.Header.Set("X-Request-ID", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

func newRequestID() string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}

	return hex.EncodeToString(data)
}

func accessLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := requestctx.RequestStart(c.Request.Context())
		if start.IsZero() {
			start = time.Now().UTC()
		}

		l := applogger.FromContext(c.Request.Context())

		c.Next()

		status := c.Writer.Status()
		latency := time.Since(start)

		fields := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"latency_ms", float64(latency.Microseconds()) / 1000.0,
			"created_at", start.Format(time.RFC3339),
		}

		if len(c.Errors) > 0 {
			fields = append(fields, "errors", c.Errors.String())
			l.Error("http request failed", fields...)
		} else if status >= http.StatusInternalServerError {
			l.Error("http request error", fields...)
		} else if status >= http.StatusBadRequest {
			l.Warn("http request warning", fields...)
		} else {
			l.Info("http request completed", fields...)
		}
	}
}

func summarizeUserAgent(userAgent string) (browser string, platform string, deviceType string) {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return "unknown", "unknown", "unknown"
	}

	browser = "unknown"
	switch {
	case strings.Contains(ua, "edg/"):
		browser = "edge"
	case strings.Contains(ua, "opr/"), strings.Contains(ua, "opera"):
		browser = "opera"
	case strings.Contains(ua, "chrome/") && !strings.Contains(ua, "edg/") && !strings.Contains(ua, "opr/"):
		browser = "chrome"
	case strings.Contains(ua, "firefox/"):
		browser = "firefox"
	case strings.Contains(ua, "safari/") && !strings.Contains(ua, "chrome/"):
		browser = "safari"
	case strings.Contains(ua, "postmanruntime/"):
		browser = "postman"
	case strings.Contains(ua, "curl/"):
		browser = "curl"
	}

	platform = "unknown"
	switch {
	case strings.Contains(ua, "windows"):
		platform = "windows"
	case strings.Contains(ua, "android"):
		platform = "android"
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"), strings.Contains(ua, "ios"):
		platform = "ios"
	case strings.Contains(ua, "mac os x"), strings.Contains(ua, "macintosh"):
		platform = "macos"
	case strings.Contains(ua, "linux"):
		platform = "linux"
	}

	deviceType = "desktop"
	switch {
	case strings.Contains(ua, "bot"), strings.Contains(ua, "spider"), strings.Contains(ua, "crawler"):
		deviceType = "bot"
	case strings.Contains(ua, "ipad"), strings.Contains(ua, "tablet"):
		deviceType = "tablet"
	case strings.Contains(ua, "mobile"), strings.Contains(ua, "iphone"), strings.Contains(ua, "android"):
		deviceType = "mobile"
	case browser == "curl" || browser == "postman":
		deviceType = "script"
	}

	return browser, platform, deviceType
}
