# Vesko Go Backend

Go backend for Vesko auth and session management.

## Current Scope

The server currently exposes auth-focused APIs with:

- Gin HTTP server with request ID propagation, access logs, and panic recovery
- PostgreSQL-backed user persistence via GORM
- Redis-backed caching, refresh sessions, pending signup state, login OTP state, and OTP rate limiting
- password login
- OTP-based signup and OTP login using Twilio Verify
- Google SSO login with first-time profile completion
- web and mobile client token flows

The repository also contains a `catalog/` scaffold and broader architecture notes, but those routes are not wired into `main.go` yet.

## Tech Stack

- Go 1.25
- Gin
- GORM
- PostgreSQL
- Redis
- Twilio Verify
- Google OAuth2 / Google UserInfo API

## Project Layout

```text
auth/                  auth domain types, interfaces, errors, token logic
auth/http/             auth HTTP handlers and middleware helpers
auth/service/          auth orchestration and business flows
auth/provider/twilio/  Twilio Verify OTP provider
cache/redis/           Redis-backed stores, caches, and rate limiter
catalog/               catalog scaffold (not registered in the live HTTP server yet)
configs/               environment loading and validation
dbase/                 database connection and Postgres repositories
docs/                  frontend contract, architecture notes, Postman collection
logger/                structured logger setup
requestctx/            request metadata helpers
server/                HTTP server bootstrap and middleware
validation/            shared request validation wrapper
```

## Authentication Model

### Password Login

- login with `username + password`
- returns an access token for all clients
- returns refresh token in JSON for `android` and `ios`
- stores refresh token in an `HttpOnly` cookie for `web`

### OTP Signup

- signup starts with OTP request
- pending signup data is stored in Redis until verification succeeds
- user record is created only after OTP verification
- duplicate username/email/mobile checks include pending signups

### OTP Login

- OTP login uses mobile number
- only allowed for users with `mobile_verified=true`
- OTP request endpoints return generic responses for ineligible accounts

### Google SSO

- `GET /auth/google/login` redirects the client to Google OAuth
- `GET /auth/google/callback` exchanges the code, fetches Google user info, and either:
  - signs in an already linked Google account
  - links to an existing Vesko account with the same email
  - creates a new SSO account with `is_profile_complete=false`
- frontend must branch on `user.is_profile_complete` from the callback response:
  - `false`: show profile completion and call `POST /auth/profile/complete`
  - `true`: skip profile completion and continue to the app
- first-time SSO users must call `POST /auth/profile/complete` with their access token to set `username` and `mobile`
- partial SSO access tokens are not accepted by `GET /auth/me`

## Client Types

Every auth flow requires `client_type`.

- `web`
  - access token in JSON
  - refresh token in `HttpOnly` cookie
  - CSRF cookie is also issued on successful web auth flows that mint refresh tokens
  - frontend must use `credentials: "include"` for cookie-based requests
- `android`
  - access token in JSON
  - refresh token in JSON
- `ios`
  - access token in JSON
  - refresh token in JSON

## Main Endpoints

- `POST /auth/register`
- `POST /auth/signup/request-otp`
- `POST /auth/signup/verify-otp`
- `POST /auth/signup/resend-otp`
- `POST /auth/signup/status`
- `POST /auth/login`
- `POST /auth/login/request-otp`
- `POST /auth/login/verify-otp`
- `POST /auth/login/resend-otp`
- `GET /auth/google/login`
- `GET /auth/google/callback`
- `POST /auth/profile/complete`
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /auth/me`
- `GET /healthz`

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

- `REDIS_ENABLED`
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

Set `REDIS_ENABLED=false` to run without Redis. In that mode the app uses Postgres directly for user lookups and process-local memory for refresh sessions, pending signup state, login OTP state, and OTP rate limits. This is intended for initial development only: deploys, restarts, cold replacements, scale-to-zero, or multiple Cloud Run instances clear or split that in-memory auth state. Set `REDIS_ENABLED=true` to use Redis-backed cache and auth state.

### HTTP

- `HTTP_PORT`
- `HTTP_ALLOWED_ORIGINS`

`HTTP_ALLOWED_ORIGINS` is a comma-separated list of allowed browser origins.

### Auth Tokens

- `AUTH_ISSUER`
- `AUTH_JWE_KEY`
- `AUTH_ACCESS_TOKEN_TTL_SECONDS`
- `AUTH_REFRESH_TOKEN_TTL_SECONDS`

`AUTH_JWE_KEY` must be exactly 32 bytes.

### Auth Cookies

- `AUTH_COOKIE_NAME`
- `AUTH_COOKIE_DOMAIN`
- `AUTH_COOKIE_PATH`
- `AUTH_COOKIE_SECURE`
- `AUTH_COOKIE_HTTP_ONLY`
- `AUTH_COOKIE_SAME_SITE`

`AUTH_COOKIE_SAME_SITE` must be one of `lax`, `strict`, or `none`.

### CSRF

- `AUTH_CSRF_COOKIE_NAME`
- `AUTH_CSRF_HEADER_NAME`
- `AUTH_CSRF_COOKIE_PATH`
- `AUTH_CSRF_COOKIE_SECURE`
- `AUTH_CSRF_COOKIE_SAME_SITE`

`AUTH_CSRF_COOKIE_SAME_SITE` must be one of `lax`, `strict`, or `none`.

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

### Google OAuth

- `GOOGLE_CLIENT_ID`
- `GOOGLE_CLIENT_SECRET`
- `GOOGLE_REDIRECT_URI`

## Local Development

In `dev`, the app loads:

```text
./configs/.env.config
```

To run locally:

1. Start PostgreSQL. Start Redis only if `REDIS_ENABLED=true`.
2. Set the required env vars for auth, Twilio, and Google OAuth.
3. Run:

```bash
go run .
```

On startup, the app auto-migrates the user, password, and SSO account tables.

## Validation And Testing

Request validation is handled through the repo-level `validation` package using `go-playground/validator`.

Run tests with:

```bash
go test ./...
```

## Docs

- Frontend contract: [docs/frontend-auth-integration.md](/home/shivateja-bodige/projects/vesko-go-backend/docs/frontend-auth-integration.md)
- Postman collection: [docs/auth_postman_collection.json](/home/shivateja-bodige/projects/vesko-go-backend/docs/auth_postman_collection.json)
- Architecture direction: [docs/architecture.md](/home/shivateja-bodige/projects/vesko-go-backend/docs/architecture.md)
- OTP design notes: [docs/otp-auth-decisions.md](/home/shivateja-bodige/projects/vesko-go-backend/docs/otp-auth-decisions.md)

## Notes

- mobile numbers are normalized to Indian E.164 format before storage and OTP usage
- browser refresh and logout require the configured CSRF cookie/header pair
- OTP request and verify endpoints are rate-limited by IP and normalized mobile number
- auth audit logs capture signup, login, refresh, logout, CSRF failures, Google SSO, and OTP rate-limit events
- the HTTP server generates or forwards `X-Request-ID` for every request
