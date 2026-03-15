# Vesko Go Backend

Go backend for Vesko authentication and supporting infrastructure.

## Current Scope

This repo currently focuses on:

- HTTP API server using Gin
- PostgreSQL-backed user persistence via GORM
- Redis-backed caching and auth state
- access/refresh token authentication
- browser and mobile client support
- mobile OTP-based signup verification and OTP login using Twilio Verify

## Tech Stack

- Go
- Gin
- GORM
- PostgreSQL
- Redis
- Twilio Verify

## Project Layout

```text
auth/                  core auth domain types, interfaces, errors, token logic
auth/http/             auth HTTP transport layer
auth/service/          auth orchestration and business flow logic
auth/provider/twilio/  Twilio Verify OTP provider
cache/redis/           Redis-backed stores and caches
configs/               environment loading and validation
dbase/                 database connection and repositories
docs/                  auth docs and design decisions
logger/                app logger setup
requestctx/            request metadata helpers
server/                HTTP server bootstrap and middleware
validation/            shared go-playground/validator wrapper
```

## Authentication Model

### Password Login

- login with `username + password`
- returns access token for all clients
- returns refresh token in JSON for mobile clients
- writes refresh token to `HttpOnly` cookie for browser clients

### OTP Signup

- signup starts with OTP request
- user is not inserted into the database immediately
- pending signup data is stored temporarily in Redis
- user is created only after OTP verification succeeds

### OTP Login

- OTP login uses mobile number
- only allowed for users with `mobile_verified=true`
- login OTP request uses a generic response to avoid account enumeration

## Client Types

Every auth flow requires `client_type`.

- `web`
  - access token in JSON
  - refresh token in `HttpOnly` cookie
  - frontend must use `credentials: "include"` where cookies are required
- `android`
  - access token in JSON
  - refresh token in JSON
- `ios`
  - access token in JSON
  - refresh token in JSON

## Main Auth Endpoints

- `POST /auth/register`
- `POST /auth/signup/request-otp`
- `POST /auth/signup/verify-otp`
- `POST /auth/signup/resend-otp`
- `POST /auth/signup/status`
- `POST /auth/login`
- `POST /auth/login/request-otp`
- `POST /auth/login/verify-otp`
- `POST /auth/login/resend-otp`
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /auth/me`
- `GET /healthz`

Detailed frontend contract:

- [docs/frontend-auth-integration.md](/home/shivateja-bodige/projects/vesko-go-backend/docs/frontend-auth-integration.md)

OTP implementation decisions:

- [docs/otp-auth-decisions.md](/home/shivateja-bodige/projects/vesko-go-backend/docs/otp-auth-decisions.md)

## Environment Variables

### Database

- `DB_HOST`
- `DB_PORT`
- `DB_USER`
- `DB_PASS`
- `DB_NAME`
- `DB_DEBUG`
- `DB_SSL_MODE`

### Redis

- `REDIS_HOST`
- `REDIS_PORT`
- `REDIS_USER`
- `REDIS_PASS`
- `REDIS_DB`
- `REDIS_CACHE_TTL_SECONDS`
- `REDIS_DIAL_TIMEOUT_SECONDS`
- `REDIS_READ_TIMEOUT_SECONDS`
- `REDIS_WRITE_TIMEOUT_SECONDS`
- `REDIS_POOL_TIMEOUT_MILLISECONDS`

### HTTP

- `HTTP_PORT`
- `HTTP_ALLOWED_ORIGINS`

### Auth Tokens

- `AUTH_ISSUER`
- `AUTH_JWE_KEY`
- `AUTH_ACCESS_TOKEN_TTL_SECONDS`
- `AUTH_REFRESH_TOKEN_TTL_SECONDS`

### Auth Cookies

- `AUTH_COOKIE_NAME`
- `AUTH_COOKIE_DOMAIN`
- `AUTH_COOKIE_PATH`
- `AUTH_COOKIE_SECURE`
- `AUTH_COOKIE_HTTP_ONLY`
- `AUTH_COOKIE_SAME_SITE`

### OTP / Twilio

- `TWILIO_ACCOUNT_SID`
- `TWILIO_AUTH_TOKEN`
- `TWILIO_VERIFY_SERVICE_SID`
- `AUTH_PENDING_SIGNUP_TTL_SECONDS`
- `AUTH_OTP_RESEND_COOLDOWN_SECONDS`
- `AUTH_OTP_MAX_RESENDS`
- `AUTH_OTP_REQUEST_LIMIT_IP`
- `AUTH_OTP_REQUEST_LIMIT_WINDOW_SECONDS`
- `AUTH_OTP_REQUEST_LIMIT_MOBILE`
- `AUTH_OTP_VERIFY_LIMIT_IP`
- `AUTH_OTP_VERIFY_LIMIT_WINDOW_SECONDS`
- `AUTH_OTP_VERIFY_LIMIT_MOBILE`
- `AUTH_CSRF_COOKIE_NAME`
- `AUTH_CSRF_HEADER_NAME`
- `AUTH_CSRF_COOKIE_PATH`
- `AUTH_CSRF_COOKIE_SECURE`
- `AUTH_CSRF_COOKIE_SAME_SITE`

### Local Env File

In `dev`, the app loads:

```text
./configs/.env.config
```

## Running Locally

1. Start PostgreSQL and Redis.
2. Configure the required environment variables.
3. Run the app:

```bash
go run .
```

If `ENV=dev`, the app loads values from `configs/.env.config`.

## Validation

Shared request validation is implemented through the repo-level `validation` package using `go-playground/validator`.

Validation is applied at the HTTP boundary before service execution.

## Testing

Run:

```bash
go test ./...
```

## Notes

- mobile numbers are normalized to Indian E.164 format before storage and OTP usage
- email and mobile are unique per account
- `mobile_verified` is enforced for OTP login
- browser cookie flows now require credentialed CORS using `HTTP_ALLOWED_ORIGINS`
- browser refresh/logout require double-submit CSRF using the CSRF cookie and configured CSRF header
- OTP request and verify endpoints now have rate-limit hooks for IP and mobile based throttling
- auth audit logs now capture key auth events such as OTP request/verify outcomes, login results, refresh/logout outcomes, CSRF failures, and OTP rate-limit hits
