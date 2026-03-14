# Frontend Auth Integration

This document describes the current frontend-facing auth contract.

## Client Types

Every auth flow requires `client_type`.

- `web`
  - access token is returned in JSON
  - refresh token is set in an `HttpOnly` cookie
  - frontend must use `credentials: "include"` for cookie-based routes
- `android` or `ios`
  - access token is returned in JSON
  - refresh token is returned in JSON

## Browser Notes

For `web` clients:

- store only the access token in frontend state
- send protected requests with `Authorization: Bearer <access_token>`
- use `credentials: "include"` on:
  - signup verify OTP
  - login
  - login verify OTP
  - refresh
  - logout

## Password Login

`POST /auth/login`

Request:

```json
{
  "username": "johndoe",
  "password": "strongPassword123",
  "client_type": "web"
}
```

Success: `200 OK`

Web response:

```json
{
  "user": {
    "id": 1,
    "username": "johndoe",
    "email": "john@example.com",
    "mobile": "+919642560235",
    "mobile_verified": true
  },
  "tokens": {
    "access_token": "<access-token>",
    "token_type": "Bearer",
    "access_token_expires_in": 900,
    "refresh_token_expires_in": 86400
  }
}
```

Mobile response additionally includes `refresh_token`.

## OTP Signup Flow

### 1. Request signup OTP

`POST /auth/signup/request-otp`

`POST /auth/register` is also supported as an alias and now starts OTP signup instead of creating a user immediately.

Request:

```json
{
  "username": "johndoe",
  "password": "strongPassword123",
  "email": "john@example.com",
  "mobile": "9642560235",
  "client_type": "web"
}
```

Notes:

- local Indian mobile input is accepted
- backend normalizes and stores mobile as E.164, for example `+919642560235`
- user is not created yet
- signup data is stored temporarily until OTP verification succeeds

Success: `202 Accepted`

```json
{
  "message": "otp sent to mobile number",
  "request_id": "<request-id>"
}
```

Conflict examples: `409 Conflict`

```json
{
  "error": "username already exists",
  "request_id": "<request-id>"
}
```

```json
{
  "error": "signup verification pending",
  "request_id": "<request-id>"
}
```

### 2. Verify signup OTP

`POST /auth/signup/verify-otp`

Request:

```json
{
  "mobile": "9642560235",
  "code": "123456",
  "client_type": "web"
}
```

Success: `201 Created`

- user is created
- `mobile_verified` becomes `true`
- tokens are issued immediately

For `web`, the refresh token is written to the cookie and omitted from JSON.

### 3. Resend signup OTP

`POST /auth/signup/resend-otp`

Request:

```json
{
  "mobile": "9642560235",
  "client_type": "web"
}
```

Success: `202 Accepted`

```json
{
  "message": "otp resent to mobile number",
  "request_id": "<request-id>"
}
```

Errors:

- `410 Gone` when pending signup expired
- `429 Too Many Requests` when resend cooldown or resend limit is hit

### 4. Signup status

`POST /auth/signup/status`

Request:

```json
{
  "mobile": "9642560235"
}
```

Response:

```json
{
  "status": "pending",
  "request_id": "<request-id>"
}
```

Possible status values:

- `pending`
- `expired`
- `completed`

## OTP Login Flow

OTP login is allowed only for users whose mobile number is already verified.

### 1. Request login OTP

`POST /auth/login/request-otp`

Request:

```json
{
  "mobile": "9642560235",
  "client_type": "web"
}
```

Success: always `202 Accepted`

```json
{
  "message": "if the account is eligible, an otp has been sent",
  "request_id": "<request-id>"
}
```

This response is intentionally generic to avoid account enumeration.

### 2. Resend login OTP

`POST /auth/login/resend-otp`

Request body is the same as `login/request-otp`.

Response is the same generic `202 Accepted` message.

### 3. Verify login OTP

`POST /auth/login/verify-otp`

Request:

```json
{
  "mobile": "9642560235",
  "code": "123456",
  "client_type": "web"
}
```

Success: `200 OK`

Response shape matches password login.

## Refresh

`POST /auth/refresh`

Web request:

```json
{
  "client_type": "web"
}
```

Mobile request:

```json
{
  "client_type": "android",
  "refresh_token": "<refresh-token>"
}
```

## Logout

`POST /auth/logout`

Web request:

```json
{
  "client_type": "web"
}
```

Mobile request:

```json
{
  "client_type": "ios",
  "refresh_token": "<refresh-token>"
}
```

## Current User

`GET /auth/me`

Header:

```http
Authorization: Bearer <access-token>
```

## Standard Error Shape

```json
{
  "error": "message",
  "request_id": "<request-id>",
  "details": {
    "field_name": "validation message"
  }
}
```

`details` is only present for validation errors.
