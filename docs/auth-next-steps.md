# Auth Next Steps

This document captures the remaining auth work after the current mobile OTP, browser cookie auth, CORS, and CSRF changes.

## Current State

Implemented:

- password login with `username + password`
- OTP signup with pending signup state
- OTP login with verified mobile only
- browser/mobile split for refresh token transport
- credentialed CORS support
- CSRF protection for browser refresh/logout
- auth package split into:
  - `auth`
  - `auth/service`
  - `auth/http`
  - `auth/provider/twilio`

Not implemented yet:

- email verification
- HTTP handler/integration coverage
- OTP abuse/rate limiting
- stronger DB migration strategy

## Recommended Next Order

1. Implement email verification
2. Introduce proper DB migrations

## Completed In This Session

- auth HTTP handler/integration tests for browser/mobile login transport, CSRF enforcement, logout cookie clearing, and CORS preflight
- OTP abuse protection with rate-limit hooks for request/resend/verify routes
- browser auth doc refinement for CORS and CSRF requirements
- auth audit logging for OTP request/verify flows, password login, refresh/logout outcomes, CSRF failures, and OTP rate-limit hits

## 1. Email Verification

Why:

- current service logic has test coverage
- transport behavior is now more complex than service behavior
- cookies, CSRF, and CORS are easiest to regress silently

Add tests for:

- `POST /auth/login` for `web`
  - refresh cookie set
  - CSRF cookie set
  - refresh token omitted from JSON
- `POST /auth/login` for `android` or `ios`
  - refresh token returned in JSON
  - no cookie dependency
- `POST /auth/refresh`
  - browser request succeeds only when cookie + CSRF header both match
  - browser request fails without CSRF header
  - mobile request succeeds with refresh token in body
- `POST /auth/logout`
  - browser request clears refresh cookie
  - browser request requires CSRF
- CORS
  - allowed origin gets credentialed CORS headers
  - disallowed origin gets blocked on preflight

When implemented, likely changes:

- add `email_verified` to the user model
- decide whether email verification is:
  - required at signup completion
  - or post-signup
- add email verification request/verify endpoints
- add resend logic and expiry rules
- choose email provider abstraction similar to OTP provider abstraction
- update frontend auth docs

Important policy decision for later:

- decide whether legacy users need an `email_verified` backfill strategy when email verification is introduced

## 2. DB Migration Strategy

Current state:

- schema changes rely on GORM `AutoMigrate`

Why improve:

- auth schema is getting more sensitive
- explicit migrations are safer for rollout and rollback
- future fields like `email_verified`, audit columns, or verification metadata are better introduced with versioned migrations

Recommended direction:

- adopt a migration tool
- keep `AutoMigrate` out of production schema evolution once migrations are in place

## Frontend Follow-Ups

Frontend must already support:

- `credentials: "include"` for browser auth cookie flows
- `X-CSRF-Token` for browser refresh/logout
- `client_type` on auth requests

Still worth confirming in frontend code:

- helper for reading CSRF cookie
- shared refresh flow wrapper
- browser vs mobile auth client separation

## Security Follow-Ups

- review `SameSite` and `Secure` cookie settings per environment
- confirm production CORS origins are explicit and minimal
- consider masking mobile values in logs if you want lower PII exposure
- consider exporting metrics in addition to logs for:
  - OTP request/verify outcomes
  - rate-limit hits
  - refresh failures
  - logout failures

## Nice-to-Have Cleanup

- add OpenAPI or API reference for auth routes
- add a small auth architecture doc linking service, provider, Redis state, and HTTP transport
- add more typed/domain-specific error response mapping if the API surface grows
