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

	"vesko/requestctx"

	"github.com/gin-gonic/gin"
)

type RouteRegistrar interface {
	RegisterRoutes(router gin.IRouter)
}

type Config struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	BaseContext     context.Context
	Logger          *slog.Logger
}

type HTTPServer struct {
	server *http.Server
	logger *slog.Logger
}

func New(cfg Config, routeRegistrar RouteRegistrar) *HTTPServer {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	serverLogger := logger.With("component", "http_server")
	baseContext := cfg.BaseContext
	if baseContext == nil {
		baseContext = context.Background()
	}

	router := gin.New()
	router.Use(requestContextMiddleware())
	router.Use(accessLogMiddleware(serverLogger))
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		serverLogger.Error("panic recovered",
			"request_id", requestctx.RequestID(c.Request.Context()),
			"method", c.Request.Method,
			"path", c.FullPath(),
			"error", fmt.Sprint(recovered),
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}))
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
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

func (s *HTTPServer) Start() error {
	s.logger.Info("http server starting", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.logger.Info("http server shutting down")
	return s.server.Shutdown(ctx)
}

func requestContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now().UTC()
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}

		c.Request = c.Request.WithContext(requestctx.WithMetadata(c.Request.Context(), requestctx.Metadata{
			RequestID:    requestID,
			RequestStart: start,
			TraceParent:  strings.TrimSpace(c.GetHeader("traceparent")),
			TraceState:   strings.TrimSpace(c.GetHeader("tracestate")),
		}))
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

func accessLogMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := requestctx.RequestStart(c.Request.Context())
		if start.IsZero() {
			start = time.Now().UTC()
		}
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		logFn := logger.Info
		if len(c.Errors) > 0 || c.Writer.Status() >= http.StatusInternalServerError {
			logFn = logger.Error
		} else if c.Writer.Status() >= http.StatusBadRequest {
			logFn = logger.Warn
		}

		logFn("http request completed",
			"type", "http_access",
			"request_id", requestctx.RequestID(c.Request.Context()),
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency_ms", float64(time.Since(start).Microseconds())/1000.0,
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"bytes_in", c.Request.ContentLength,
			"bytes_out", c.Writer.Size(),
			"errors", c.Errors.String(),
		)
	}
}
