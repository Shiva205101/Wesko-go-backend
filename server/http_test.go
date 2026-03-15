package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeRegistrar struct{}

func (r fakeRegistrar) RegisterRoutes(router gin.IRouter) {
	router.POST("/auth/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
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
