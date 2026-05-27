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

## User Object

Most success responses return a `user` object with the following shape:

```json
{
  "id": 1,
  "username": "johndoe",
  "email": "john@example.com",
  "mobile": "+919642560235",
  "mobile_verified": true,
  "role": "customer",
  "is_profile_complete": true
}
```

- `role`: Can be `customer` or `admin`. Default is `customer`.
- `is_profile_complete`: `true` for standard accounts. `false` for new SSO users who haven't set a username/mobile yet.

## Browser Notes

For `web` clients:

- store only the access token in frontend state
- send protected requests with `Authorization: Bearer <access_token>`
- use `credentials: "include"` on:
  - signup verify OTP
  - login
  - login verify OTP
  - Google callback
  - profile completion
  - refresh
  - logout
- read the CSRF token from the `csrf_token` cookie and send it in the `X-CSRF-Token` header for:
  - refresh
  - logout

Example browser refresh request:

```ts
const csrfToken = readCookie("csrf_token");

await fetch("/auth/refresh", {
  method: "POST",
  credentials: "include",
  headers: {
    "Content-Type": "application/json",
    "X-CSRF-Token": csrfToken
  },
  body: JSON.stringify({ client_type: "web" })
});
```

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
    "mobile_verified": true,
    "role": "customer",
    "is_profile_complete": true
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

## Google SSO Flow

### 1. Initiate Login
Frontend redirects the user to the backend login endpoint.

`GET /auth/google/login`

The backend will respond with a `302 Redirect` to Google's OAuth2 consent screen.

### 2. Handle Callback
After Google login, the user is redirected back to the authorized Redirect URI with a `code` parameter.

`GET /auth/google/callback?code=<code>&client_type=web`

Success: `200 OK`

Response shape matches Password Login.

**Note:** If `is_profile_complete` is `false`, the issued `access_token` is a **partial token**. It only allows access to the Profile Completion endpoint. The frontend must redirect the user to a profile completion form.

### 3. Complete Profile (Mandatory for new SSO users)
Used to set a `username` and `mobile` number after the first Google login.

`POST /auth/profile/complete`

Header: `Authorization: Bearer <partial-access-token>`

Request:
```json
{
  "username": "chosen_username",
  "mobile": "9642560235",
  "client_type": "web"
}
```

Success: `200 OK`
Returns a new `user` object with `is_profile_complete: true` and a fresh full `access_token`.

## OTP Signup Flow

### 1. Request signup OTP

`POST /auth/signup/request-otp`

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

Success: `202 Accepted`

```json
{
  "message": "otp sent to mobile number",
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
Response shape matches Password Login.

## OTP Login Flow

### 1. Request login OTP

`POST /auth/login/request-otp`

Request:

```json
{
  "mobile": "9642560235",
  "client_type": "web"
}
```

Success: `202 Accepted` (Generic response)

### 2. Verify login OTP

`POST /auth/login/verify-otp`

Success: `200 OK`
Response shape matches Password Login.

## Session Management

### Refresh
`POST /auth/refresh`

### Logout
`POST /auth/logout`

### Current User
`GET /auth/me`

Header: `Authorization: Bearer <access-token>`

Returns the [User Object](#user-object).

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
