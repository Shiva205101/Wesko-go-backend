package server

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeRegistrar struct{}

func (r fakeRegistrar) RegisterRoutes(router gin.IRouter) {
	router.POST("/auth/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/auth/fail", func(c *gin.Context) {
		c.Status(http.StatusUnauthorized)
	})
}

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	server := New(Config{
		Addr:        ":8080",
		ServiceName: "wesko-api",
		Environment: "staging",
	}, fakeRegistrar{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "\"status\":\"ok\"") {
		t.Fatalf("expected health response body, got %q", rr.Body.String())
	}
}

func TestReadyEndpointReturnsServiceUnavailableWhenDependencyFails(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	server := New(Config{
		Addr: ":8080",
		ReadinessCheck: func(context.Context) error {
			return context.DeadlineExceeded
		},
	}, fakeRegistrar{})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestCORSPreflightAllowsConfiguredOrigin(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	server := New(Config{
		Addr:           ":8080",
		AllowedOrigins: []string{"http://localhost:3000"},
	}, fakeRegistrar{})

	req := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")

	rr := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("expected allowed origin header, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header, got %q", got)
	}
}

func TestCORSPreflightRejectsDisallowedOrigin(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	server := New(Config{
		Addr:           ":8080",
		AllowedOrigins: []string{"http://localhost:3000"},
	}, fakeRegistrar{})

	req := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	rr := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestAccessLogMiddlewareLogsRequestStartAndFinish(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{}))
	server := New(Config{
		Addr:        ":8080",
		Logger:      logger,
		ServiceName: "wesko-api",
		BaseContext: context.Background(),
	}, fakeRegistrar{})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"demo"}`))
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "203.0.113.9:1234"

	rr := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	logs := logOutput.String()
	if !strings.Contains(logs, "msg=\"http request started\"") {
		t.Fatalf("expected request start log, got %q", logs)
	}
	if !strings.Contains(logs, "phase=start") {
		t.Fatalf("expected request start phase, got %q", logs)
	}
	if !strings.Contains(logs, "msg=\"http request completed\"") {
		t.Fatalf("expected request completion log, got %q", logs)
	}
	if !strings.Contains(logs, "phase=finish") {
		t.Fatalf("expected request completion phase, got %q", logs)
	}
	if !strings.Contains(logs, "path=/auth/login") {
		t.Fatalf("expected request path in logs, got %q", logs)
	}
	if !strings.Contains(logs, "route=/auth/login") {
		t.Fatalf("expected route path in logs, got %q", logs)
	}
	if strings.Contains(logs, "msg=\"http request client details\"") {
		t.Fatalf("did not expect client details log for successful request, got %q", logs)
	}
	if strings.Contains(logs, "user_agent=") {
		t.Fatalf("did not expect user agent in successful access logs, got %q", logs)
	}
}

func TestAccessLogMiddlewareLogsClientDetailsForWarnResponses(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{}))
	server := New(Config{
		Addr:        ":8080",
		Logger:      logger,
		ServiceName: "wesko-api",
		BaseContext: context.Background(),
	}, fakeRegistrar{})

	req := httptest.NewRequest(http.MethodGet, "/auth/fail", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	req.RemoteAddr = "203.0.113.9:1234"

	rr := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	logs := logOutput.String()
	if !strings.Contains(logs, "msg=\"http request completed\"") || !strings.Contains(logs, "level=WARN") {
		t.Fatalf("expected warn completion log, got %q", logs)
	}
	if !strings.Contains(logs, "msg=\"http request client details\"") {
		t.Fatalf("expected client details log, got %q", logs)
	}
	if !strings.Contains(logs, "phase=client") {
		t.Fatalf("expected client details phase, got %q", logs)
	}
	if !strings.Contains(logs, "browser=chrome") {
		t.Fatalf("expected browser summary, got %q", logs)
	}
	if !strings.Contains(logs, "platform=macos") {
		t.Fatalf("expected platform summary, got %q", logs)
	}
	if !strings.Contains(logs, "device_type=desktop") {
		t.Fatalf("expected device type summary, got %q", logs)
	}
	if !strings.Contains(logs, "user_agent=\"Mozilla/5.0") {
		t.Fatalf("expected raw user agent on warn logs, got %q", logs)
	}
}
