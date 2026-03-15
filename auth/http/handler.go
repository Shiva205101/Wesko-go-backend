package http

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	stdhttp "net/http"
	"strings"
	"time"

	"vesko/auth"
	authservice "vesko/auth/service"
	"vesko/requestctx"
	"vesko/validation"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *authservice.Service
	logger  *slog.Logger
	cookies CookieConfig
	limiter auth.RateLimiter
	limits  auth.OTPRateLimitConfig
}

type CookieConfig struct {
	RefreshTokenName   string
	Domain             string
	Path               string
	Secure             bool
	HTTPOnly           bool
	SameSite           stdhttp.SameSite
	CSRFCookieName     string
	CSRFHeaderName     string
	CSRFCookiePath     string
	CSRFCookieSecure   bool
	CSRFCookieSameSite stdhttp.SameSite
}

type signupRequestBody struct {
	Username   string `json:"username" validate:"required,min=3,max=50"`
	Password   string `json:"password" validate:"required,min=8,max=72"`
	Email      string `json:"email" validate:"required,email"`
	Mobile     string `json:"mobile" validate:"required"`
	ClientType string `json:"client_type" validate:"required,oneof=web android ios"`
}

type passwordLoginRequestBody struct {
	Username   string `json:"username" validate:"required,min=3,max=50"`
	Password   string `json:"password" validate:"required,min=8,max=72"`
	ClientType string `json:"client_type" validate:"required,oneof=web android ios"`
}

type otpRequestBody struct {
	Mobile     string `json:"mobile" validate:"required"`
	ClientType string `json:"client_type" validate:"required,oneof=web android ios"`
}

type otpVerifyRequestBody struct {
	Mobile     string `json:"mobile" validate:"required"`
	Code       string `json:"code" validate:"required,len=4|len=6"`
	ClientType string `json:"client_type" validate:"required,oneof=web android ios"`
}

type signupStatusRequestBody struct {
	Mobile string `json:"mobile" validate:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	ClientType   string `json:"client_type" validate:"required,oneof=web android ios"`
}

type userResponse struct {
	ID             uint   `json:"id"`
	Username       string `json:"username"`
	Email          string `json:"email,omitempty"`
	Mobile         string `json:"mobile,omitempty"`
	MobileVerified bool   `json:"mobile_verified"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

type errorResponse struct {
	Error     string            `json:"error"`
	RequestId string            `json:"request_id"`
	Details   map[string]string `json:"details,omitempty"`
}

type authResponse struct {
	User   userResponse   `json:"user"`
	Tokens auth.TokenPair `json:"tokens"`
}

type messageResponse struct {
	Message   string `json:"message"`
	RequestId string `json:"request_id"`
}

type signupStatusResponse struct {
	Status    string `json:"status"`
	RequestId string `json:"request_id"`
}

func New(service *authservice.Service, logger *slog.Logger, cookies CookieConfig) *Handler {
	return NewWithLimiter(service, logger, cookies, nil, auth.OTPRateLimitConfig{})
}

func NewWithLimiter(service *authservice.Service, logger *slog.Logger, cookies CookieConfig, limiter auth.RateLimiter, limits auth.OTPRateLimitConfig) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if cookies.RefreshTokenName == "" {
		cookies.RefreshTokenName = "refresh_token"
	}
	if cookies.Path == "" {
		cookies.Path = "/auth"
	}
	if cookies.SameSite == 0 {
		cookies.SameSite = stdhttp.SameSiteLaxMode
	}
	if cookies.CSRFCookieName == "" {
		cookies.CSRFCookieName = "csrf_token"
	}
	if cookies.CSRFCookiePath == "" {
		cookies.CSRFCookiePath = "/auth"
	}
	if cookies.CSRFHeaderName == "" {
		cookies.CSRFHeaderName = "X-CSRF-Token"
	}
	if cookies.CSRFCookieSameSite == 0 {
		cookies.CSRFCookieSameSite = stdhttp.SameSiteLaxMode
	}

	return &Handler{
		service: service,
		logger:  logger.With("component", "auth_http"),
		cookies: cookies,
		limiter: limiter,
		limits:  limits,
	}
}

func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.POST("/auth/register", h.handleRequestSignupOTP)
	router.POST("/auth/signup/request-otp", h.handleRequestSignupOTP)
	router.POST("/auth/signup/verify-otp", h.handleVerifySignupOTP)
	router.POST("/auth/signup/resend-otp", h.handleResendSignupOTP)
	router.POST("/auth/signup/status", h.handleSignupStatus)
	router.POST("/auth/login", h.handlePasswordLogin)
	router.POST("/auth/login/request-otp", h.handleRequestLoginOTP)
	router.POST("/auth/login/verify-otp", h.handleVerifyLoginOTP)
	router.POST("/auth/login/resend-otp", h.handleResendLoginOTP)
	router.POST("/auth/refresh", h.handleRefresh)
	router.POST("/auth/logout", h.handleLogout)
	router.GET("/auth/me", h.requireAccessToken(), h.handleMe)
}

func (h *Handler) handleRequestSignupOTP(c *gin.Context) {
	var req signupRequestBody
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}
	if err := h.enforceOTPRequestLimit(c, req.Mobile); err != nil {
		h.writeServiceError(c, err)
		return
	}
	err := h.service.RequestSignupOTP(c.Request.Context(), auth.RegisterRequest{
		Username:   req.Username,
		Password:   req.Password,
		Email:      req.Email,
		Mobile:     req.Mobile,
		ClientType: req.ClientType,
	})
	if err != nil {
		h.auditEvent(c, slog.LevelWarn, "signup_request_otp", "rejected",
			"client_type", req.ClientType,
			"username", strings.TrimSpace(req.Username),
			"mobile_masked", maskMobile(req.Mobile),
			"reason", err.Error(),
		)
		h.writeServiceError(c, err)
		return
	}
	h.auditEvent(c, slog.LevelInfo, "signup_request_otp", "accepted",
		"client_type", req.ClientType,
		"username", strings.TrimSpace(req.Username),
		"mobile_masked", maskMobile(req.Mobile),
	)
	c.JSON(stdhttp.StatusAccepted, messageResponse{Message: "otp sent to mobile number", RequestId: requestIDFromContext(c)})
	return
}

func (h *Handler) handleVerifySignupOTP(c *gin.Context) {
	var req otpVerifyRequestBody
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}
	if err := h.enforceOTPVerifyLimit(c, req.Mobile); err != nil {
		h.writeServiceError(c, err)
		return
	}
	user, tokens, err := h.service.VerifySignupOTP(c.Request.Context(), auth.OTPVerifyRequest{
		Mobile:     req.Mobile,
		Code:       req.Code,
		ClientType: req.ClientType,
	})
	if err != nil {
		h.auditEvent(c, slog.LevelWarn, "signup_verify_otp", "rejected",
			"client_type", req.ClientType,
			"mobile_masked", maskMobile(req.Mobile),
			"reason", err.Error(),
		)
		h.writeServiceError(c, err)
		return
	}
	h.applyRefreshTokenTransport(c, req.ClientType, &tokens)
	h.auditEvent(c, slog.LevelInfo, "signup_verify_otp", "succeeded",
		"client_type", req.ClientType,
		"username", user.Username,
		"mobile_masked", maskMobile(user.Mobile),
	)
	c.JSON(stdhttp.StatusCreated, authResponse{User: toUserResponse(user), Tokens: tokens})
	return
}

func (h *Handler) handleResendSignupOTP(c *gin.Context) {
	var req otpRequestBody
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}
	if err := h.enforceOTPRequestLimit(c, req.Mobile); err != nil {
		h.writeServiceError(c, err)
		return
	}
	if err := h.service.ResendSignupOTP(c.Request.Context(), auth.OTPRequest{Mobile: req.Mobile, ClientType: req.ClientType}); err != nil {
		h.auditEvent(c, slog.LevelWarn, "signup_resend_otp", "rejected",
			"client_type", req.ClientType,
			"mobile_masked", maskMobile(req.Mobile),
			"reason", err.Error(),
		)
		h.writeServiceError(c, err)
		return
	}
	h.auditEvent(c, slog.LevelInfo, "signup_resend_otp", "accepted",
		"client_type", req.ClientType,
		"mobile_masked", maskMobile(req.Mobile),
	)
	c.JSON(stdhttp.StatusAccepted, messageResponse{Message: "otp resent to mobile number", RequestId: requestIDFromContext(c)})
	return
}

func (h *Handler) handleSignupStatus(c *gin.Context) {
	var req signupStatusRequestBody
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}
	status, err := h.service.SignupStatus(c.Request.Context(), req.Mobile)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, signupStatusResponse{Status: status, RequestId: requestIDFromContext(c)})
	return
}

func (h *Handler) handlePasswordLogin(c *gin.Context) {
	var req passwordLoginRequestBody
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}
	user, tokens, err := h.service.Login(c.Request.Context(), auth.PasswordLoginRequest{
		Username:   req.Username,
		Password:   req.Password,
		ClientType: req.ClientType,
	})
	if err != nil {
		h.auditEvent(c, slog.LevelWarn, "password_login", "rejected",
			"client_type", req.ClientType,
			"username", strings.TrimSpace(req.Username),
			"reason", err.Error(),
		)
		h.writeServiceError(c, err)
		return
	}
	h.applyRefreshTokenTransport(c, req.ClientType, &tokens)
	h.auditEvent(c, slog.LevelInfo, "password_login", "succeeded",
		"client_type", req.ClientType,
		"username", user.Username,
	)
	c.JSON(stdhttp.StatusOK, authResponse{User: toUserResponse(user), Tokens: tokens})
	return
}

func (h *Handler) handleRequestLoginOTP(c *gin.Context) {
	var req otpRequestBody
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}
	if err := h.enforceOTPRequestLimit(c, req.Mobile); err != nil {
		h.writeServiceError(c, err)
		return
	}
	if err := h.service.RequestLoginOTP(c.Request.Context(), auth.OTPRequest{Mobile: req.Mobile, ClientType: req.ClientType}); err != nil {
		h.auditEvent(c, slog.LevelWarn, "login_request_otp", "rejected",
			"client_type", req.ClientType,
			"mobile_masked", maskMobile(req.Mobile),
			"reason", err.Error(),
		)
		h.writeServiceError(c, err)
		return
	}
	h.auditEvent(c, slog.LevelInfo, "login_request_otp", "accepted",
		"client_type", req.ClientType,
		"mobile_masked", maskMobile(req.Mobile),
	)
	c.JSON(stdhttp.StatusAccepted, messageResponse{Message: "if the account is eligible, an otp has been sent", RequestId: requestIDFromContext(c)})
	return
}

func (h *Handler) handleResendLoginOTP(c *gin.Context) {
	var req otpRequestBody
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}
	if err := h.enforceOTPRequestLimit(c, req.Mobile); err != nil {
		h.writeServiceError(c, err)
		return
	}
	if err := h.service.ResendLoginOTP(c.Request.Context(), auth.OTPRequest{Mobile: req.Mobile, ClientType: req.ClientType}); err != nil {
		h.auditEvent(c, slog.LevelWarn, "login_resend_otp", "rejected",
			"client_type", req.ClientType,
			"mobile_masked", maskMobile(req.Mobile),
			"reason", err.Error(),
		)
		h.writeServiceError(c, err)
		return
	}
	h.auditEvent(c, slog.LevelInfo, "login_resend_otp", "accepted",
		"client_type", req.ClientType,
		"mobile_masked", maskMobile(req.Mobile),
	)
	c.JSON(stdhttp.StatusAccepted, messageResponse{Message: "if the account is eligible, an otp has been sent", RequestId: requestIDFromContext(c)})
	return
}

func (h *Handler) handleVerifyLoginOTP(c *gin.Context) {
	var req otpVerifyRequestBody
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}
	if err := h.enforceOTPVerifyLimit(c, req.Mobile); err != nil {
		h.writeServiceError(c, err)
		return
	}
	user, tokens, err := h.service.VerifyLoginOTP(c.Request.Context(), auth.OTPVerifyRequest{
		Mobile:     req.Mobile,
		Code:       req.Code,
		ClientType: req.ClientType,
	})
	if err != nil {
		h.auditEvent(c, slog.LevelWarn, "login_verify_otp", "rejected",
			"client_type", req.ClientType,
			"mobile_masked", maskMobile(req.Mobile),
			"reason", err.Error(),
		)
		h.writeServiceError(c, err)
		return
	}
	h.applyRefreshTokenTransport(c, req.ClientType, &tokens)
	h.auditEvent(c, slog.LevelInfo, "login_verify_otp", "succeeded",
		"client_type", req.ClientType,
		"username", user.Username,
		"mobile_masked", maskMobile(user.Mobile),
	)
	c.JSON(stdhttp.StatusOK, authResponse{User: toUserResponse(user), Tokens: tokens})
	return
}

func (h *Handler) handleRefresh(c *gin.Context) {
	var req refreshRequest
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}
	refreshToken, err := h.refreshTokenForClient(c, req.ClientType, req.RefreshToken)
	if err != nil {
		h.auditEvent(c, slog.LevelWarn, "token_refresh", "rejected",
			"client_type", req.ClientType,
			"reason", err.Error(),
		)
		h.writeServiceError(c, err)
		return
	}
	if err := h.requireCSRFFromWeb(c, req.ClientType); err != nil {
		h.auditEvent(c, slog.LevelWarn, "token_refresh", "rejected",
			"client_type", req.ClientType,
			"reason", "csrf_check_failed",
		)
		h.writeServiceError(c, err)
		return
	}
	user, tokens, err := h.service.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		h.auditEvent(c, slog.LevelWarn, "token_refresh", "rejected",
			"client_type", req.ClientType,
			"reason", err.Error(),
		)
		h.writeServiceError(c, err)
		return
	}
	h.applyRefreshTokenTransport(c, req.ClientType, &tokens)
	h.auditEvent(c, slog.LevelInfo, "token_refresh", "succeeded",
		"client_type", req.ClientType,
		"username", user.Username,
	)
	c.JSON(stdhttp.StatusOK, authResponse{User: toUserResponse(user), Tokens: tokens})
	return
}

func (h *Handler) handleLogout(c *gin.Context) {
	var req refreshRequest
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}
	refreshToken, err := h.refreshTokenForClient(c, req.ClientType, req.RefreshToken)
	if err != nil {
		h.auditEvent(c, slog.LevelWarn, "logout", "rejected",
			"client_type", req.ClientType,
			"reason", err.Error(),
		)
		h.writeServiceError(c, err)
		return
	}
	if err := h.requireCSRFFromWeb(c, req.ClientType); err != nil {
		h.auditEvent(c, slog.LevelWarn, "logout", "rejected",
			"client_type", req.ClientType,
			"reason", "csrf_check_failed",
		)
		h.writeServiceError(c, err)
		return
	}
	if err := h.service.Logout(c.Request.Context(), refreshToken); err != nil {
		h.auditEvent(c, slog.LevelWarn, "logout", "rejected",
			"client_type", req.ClientType,
			"reason", err.Error(),
		)
		h.writeServiceError(c, err)
		return
	}
	if req.ClientType == "web" {
		h.clearRefreshTokenCookie(c)
		h.clearCSRFCookie(c)
	}
	h.auditEvent(c, slog.LevelInfo, "logout", "succeeded",
		"client_type", req.ClientType,
	)
	c.Status(stdhttp.StatusNoContent)
	return
}

func (h *Handler) handleMe(c *gin.Context) {
	username := c.GetString("auth.username")
	user, err := h.service.ValidateCurrentUser(c.Request.Context(), username)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, toUserResponse(user))
	return
}

func (h *Handler) requireAccessToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := h.service.AuthenticateAccessToken(bearerToken(c.GetHeader("Authorization")))
		if err != nil {
			h.writeServiceError(c, err)
			c.Abort()
			return
		}
		c.Set("auth.username", claims.Subject)
		c.Next()
	}
}

func (h *Handler) writeServiceError(c *gin.Context, err error) {
	status := stdhttp.StatusInternalServerError
	switch {
	case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrInvalidPassword), errors.Is(err, auth.ErrInvalidEmail), errors.Is(err, auth.ErrInvalidMobile), errors.Is(err, auth.ErrInvalidOTPCode), errors.Is(err, auth.ErrInvalidMobileFormat):
		status = stdhttp.StatusBadRequest
	case errors.Is(err, auth.ErrInvalidRefreshToken), errors.Is(err, auth.ErrInvalidToken), errors.Is(err, auth.ErrUnauthorized), errors.Is(err, auth.ErrTokenExpired), errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrMobileNotVerified):
		status = stdhttp.StatusUnauthorized
	case errors.Is(err, auth.ErrUsernameAlreadyExists), errors.Is(err, auth.ErrEmailAlreadyExists), errors.Is(err, auth.ErrMobileAlreadyExists), errors.Is(err, auth.ErrUserAlreadyExists), errors.Is(err, auth.ErrSignupVerificationPending):
		status = stdhttp.StatusConflict
	case errors.Is(err, auth.ErrOTPResendTooSoon), errors.Is(err, auth.ErrOTPResendLimitReached):
		status = stdhttp.StatusTooManyRequests
	case errors.Is(err, auth.ErrOTPTooManyRequests):
		status = stdhttp.StatusTooManyRequests
	case errors.Is(err, auth.ErrPendingSignupExpired):
		status = stdhttp.StatusGone
	case errors.Is(err, auth.ErrOTPProviderUnavailable):
		status = stdhttp.StatusBadGateway
	}
	if status >= stdhttp.StatusInternalServerError {
		h.logger.Error("request failed", "request_id", requestIDFromContext(c), "method", c.Request.Method, "path", c.FullPath(), "status", status, "error", err.Error())
		c.JSON(status, errorResponse{Error: "internal server error", RequestId: requestIDFromContext(c)})
		return
	}
	h.logger.Warn("request rejected", "request_id", requestIDFromContext(c), "method", c.Request.Method, "path", c.FullPath(), "status", status, "error", err.Error())
	c.JSON(status, errorResponse{RequestId: requestIDFromContext(c), Error: err.Error()})
	return
}

func (h *Handler) decodeAndValidateJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		h.logger.Warn("invalid request body", "request_id", requestIDFromContext(c), "path", c.FullPath(), "error", err.Error())
		c.JSON(stdhttp.StatusBadRequest, errorResponse{Error: "invalid request body", RequestId: requestIDFromContext(c)})
		return err
	}
	if err := validation.Validate(dst); err != nil {
		h.logger.Warn("request validation failed", "request_id", requestIDFromContext(c), "path", c.FullPath(), "error", err.Error())
		var validationErrs validation.Errors
		if errors.As(err, &validationErrs) {
			c.JSON(stdhttp.StatusBadRequest, errorResponse{Error: validation.ErrValidationFailed.Error(), RequestId: requestIDFromContext(c), Details: validationErrs.Messages()})
			return err
		}
		c.JSON(stdhttp.StatusBadRequest, errorResponse{Error: validation.ErrValidationFailed.Error(), RequestId: requestIDFromContext(c)})
		return err
	}
	return nil
}

func (h *Handler) applyRefreshTokenTransport(c *gin.Context, clientType string, tokens *auth.TokenPair) {
	if tokens == nil || clientType != "web" {
		return
	}
	csrfToken, err := newCSRFToken()
	if err != nil {
		h.logger.Error("csrf token generation failed", "request_id", requestIDFromContext(c), "error", err.Error())
		return
	}
	h.setRefreshTokenCookie(c, tokens.RefreshToken, tokens.RefreshTokenExpiresIn)
	h.setCSRFCookie(c, csrfToken, tokens.RefreshTokenExpiresIn)
	tokens.RefreshToken = ""
}

func (h *Handler) refreshTokenForClient(c *gin.Context, clientType string, requestToken string) (string, error) {
	if clientType == "web" {
		cookie, err := c.Cookie(h.cookies.RefreshTokenName)
		if err != nil || strings.TrimSpace(cookie) == "" {
			return "", auth.ErrInvalidRefreshToken
		}
		return cookie, nil
	}
	if strings.TrimSpace(requestToken) == "" {
		return "", auth.ErrInvalidRefreshToken
	}
	return requestToken, nil
}

func (h *Handler) setRefreshTokenCookie(c *gin.Context, refreshToken string, maxAgeSeconds int64) {
	stdhttp.SetCookie(c.Writer, &stdhttp.Cookie{
		Name:     h.cookies.RefreshTokenName,
		Value:    refreshToken,
		Path:     h.cookies.Path,
		Domain:   h.cookies.Domain,
		MaxAge:   int(maxAgeSeconds),
		HttpOnly: h.cookies.HTTPOnly,
		Secure:   h.cookies.Secure,
		SameSite: h.cookies.SameSite,
	})
}

func (h *Handler) clearRefreshTokenCookie(c *gin.Context) {
	stdhttp.SetCookie(c.Writer, &stdhttp.Cookie{
		Name:     h.cookies.RefreshTokenName,
		Value:    "",
		Path:     h.cookies.Path,
		Domain:   h.cookies.Domain,
		MaxAge:   -1,
		HttpOnly: h.cookies.HTTPOnly,
		Secure:   h.cookies.Secure,
		SameSite: h.cookies.SameSite,
	})
}

func (h *Handler) setCSRFCookie(c *gin.Context, csrfToken string, maxAgeSeconds int64) {
	stdhttp.SetCookie(c.Writer, &stdhttp.Cookie{
		Name:     h.cookies.CSRFCookieName,
		Value:    csrfToken,
		Path:     h.cookies.CSRFCookiePath,
		Domain:   h.cookies.Domain,
		MaxAge:   int(maxAgeSeconds),
		HttpOnly: false,
		Secure:   h.cookies.CSRFCookieSecure,
		SameSite: h.cookies.CSRFCookieSameSite,
	})
}

func (h *Handler) clearCSRFCookie(c *gin.Context) {
	stdhttp.SetCookie(c.Writer, &stdhttp.Cookie{
		Name:     h.cookies.CSRFCookieName,
		Value:    "",
		Path:     h.cookies.CSRFCookiePath,
		Domain:   h.cookies.Domain,
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   h.cookies.CSRFCookieSecure,
		SameSite: h.cookies.CSRFCookieSameSite,
	})
}

func (h *Handler) requireCSRFFromWeb(c *gin.Context, clientType string) error {
	if clientType != "web" {
		return nil
	}

	cookieToken, err := c.Cookie(h.cookies.CSRFCookieName)
	if err != nil || strings.TrimSpace(cookieToken) == "" {
		return auth.ErrUnauthorized
	}

	headerToken := strings.TrimSpace(c.GetHeader(h.cookies.CSRFHeaderName))
	if headerToken == "" || headerToken != cookieToken {
		return auth.ErrUnauthorized
	}

	return nil
}

func toUserResponse(user auth.User) userResponse {
	resp := userResponse{
		ID:             user.ID,
		Username:       user.Username,
		Email:          user.Email,
		Mobile:         user.Mobile,
		MobileVerified: user.MobileVerified,
	}
	if !user.CreatedAt.IsZero() {
		resp.CreatedAt = user.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !user.UpdatedAt.IsZero() {
		resp.UpdatedAt = user.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return resp
}

func requestIDFromContext(c *gin.Context) string {
	requestID := requestctx.RequestID(c.Request.Context())
	if requestID != "" {
		return requestID
	}
	return c.GetHeader("X-Request-ID")
}

func (h *Handler) enforceOTPRequestLimit(c *gin.Context, mobile string) error {
	return h.enforceRateLimit(c, "request", mobile, h.limits.RequestIP, h.limits.RequestMobile)
}

func (h *Handler) enforceOTPVerifyLimit(c *gin.Context, mobile string) error {
	return h.enforceRateLimit(c, "verify", mobile, h.limits.VerifyIP, h.limits.VerifyMobile)
}

func (h *Handler) enforceRateLimit(c *gin.Context, action string, mobile string, ipRule auth.RateLimitRule, mobileRule auth.RateLimitRule) error {
	if h.limiter == nil {
		return nil
	}

	ipKey := "otp:" + action + ":ip:" + c.ClientIP()
	allowed, err := h.limiter.Allow(c.Request.Context(), ipKey, ipRule)
	if err != nil {
		return err
	}
	if !allowed {
		h.logger.Warn("otp rate limit exceeded",
			"request_id", requestIDFromContext(c),
			"type", "auth_audit",
			"action", action,
			"scope", "ip",
			"client_ip", c.ClientIP(),
		)
		return auth.ErrOTPTooManyRequests
	}

	normalizedMobile, err := auth.NormalizeIndianMobile(mobile)
	if err != nil {
		return nil
	}

	mobileKey := "otp:" + action + ":mobile:" + normalizedMobile
	allowed, err = h.limiter.Allow(c.Request.Context(), mobileKey, mobileRule)
	if err != nil {
		return err
	}
	if !allowed {
		h.logger.Warn("otp rate limit exceeded",
			"request_id", requestIDFromContext(c),
			"type", "auth_audit",
			"action", action,
			"scope", "mobile",
		)
		return auth.ErrOTPTooManyRequests
	}

	return nil
}

func (h *Handler) auditEvent(c *gin.Context, level slog.Level, action string, outcome string, args ...any) {
	fields := []any{
		"type", "auth_audit",
		"request_id", requestIDFromContext(c),
		"action", action,
		"outcome", outcome,
		"client_ip", c.ClientIP(),
		"origin", strings.TrimSpace(c.GetHeader("Origin")),
	}
	fields = append(fields, args...)

	record := slog.NewRecord(time.Now(), level, "auth event", 0)
	record.Add(fields...)
	_ = h.logger.Handler().Handle(c.Request.Context(), record)
}

func maskMobile(mobile string) string {
	mobile = strings.TrimSpace(mobile)
	if mobile == "" {
		return ""
	}
	if len(mobile) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(mobile)-4) + mobile[len(mobile)-4:]
}

func bearerToken(header string) string {
	token := strings.TrimSpace(header)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return token
}

func newCSRFToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}

	return hex.EncodeToString(data), nil
}
