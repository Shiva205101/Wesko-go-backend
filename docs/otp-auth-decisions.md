# OTP Auth Decisions

This document records the questions discussed before implementing mobile OTP signup and login, along with the chosen answers.

## 1. Should mobile numbers be unique?

Yes.

- each user must have a unique mobile number
- each user must have a unique email
- username also remains unique

## 2. Should mobile numbers be stored as local formats or normalized?

Backend accepts local Indian input but normalizes it before storage.

- input like `9642560235` is accepted
- backend normalizes it to E.164
- stored format is canonical, for example `+919642560235`
- default country is India

## 3. How should signup OTP work?

Use pending signup.

Flow:

1. request signup OTP with `username`, `password`, `email`, `mobile`, `client_type`
2. store pending signup temporarily
3. send OTP
4. verify OTP
5. create user only after successful verification
6. issue tokens immediately

## 4. How should pending signup status be exposed?

Support both:

- `signup/request-otp`
- `signup/verify-otp`
- `signup/resend-otp`
- `signup/status`

Status values:

- `pending`
- `expired`
- `completed`

## 5. How should OTP login work?

Use a two-step flow.

1. `login/request-otp`
2. `login/verify-otp`

`login/resend-otp` is also supported.

## 6. How should password login work?

Keep it simple for now:

- password login uses `username + password`
- OTP login uses `mobile + otp`

Do not implement the broader `identifier + auth_method` matrix yet.

## 7. Should OTP login require verified mobile?

Yes.

Only users with `mobile_verified=true` can log in via OTP.

## 8. Should OTP TTL, cooldown, and limits be configurable?

Yes.

They are env-configurable.

Recommended defaults discussed:

- pending signup TTL: `10m`
- resend cooldown: `60s`
- max resends: `5`

## 9. Which OTP provider flow should be used?

Use Twilio Verify:

- send OTP via `Verifications`
- verify OTP via `VerificationCheck`

## 10. Should OTP provider integration be abstracted?

Yes.

Use an interface in the auth layer and a Twilio implementation behind it, so additional providers can be added later.

## 11. What should signup status return when no pending signup exists?

Use:

- `pending`
- `expired`
- `completed`

## 12. Should resend support only cooldown or also max resend count?

Use both:

- cooldown
- max resend count

Both are configurable.

## 13. Should pending signup block duplicate signup attempts?

Yes.

If a pending signup exists for the same username, email, or mobile, a new signup attempt is blocked until it expires or completes.

## 14. If the same signup is attempted again while pending, should OTP be resent automatically?

No.

Return a pending/conflict response and require explicit `resend-otp`.

## 15. Should invalid Indian mobile inputs be rejected after normalization?

Yes.

Do not store raw invalid mobile strings.

## 16. Should the user table include `mobile_verified` now?

Yes.

`email_verified` can be added later with the email verification implementation.

## 17. After signup OTP verification, should tokens be issued immediately?

Yes.

Successful signup verification creates the account and logs the user in.

## 18. Should login OTP request reveal whether the user exists?

No.

Use a generic anti-enumeration response:

- if eligible, OTP is sent
- otherwise return the same generic accepted message

## 19. Should signup return explicit conflict errors?

Yes.

Signup should explicitly tell the frontend about:

- username conflict
- email conflict
- mobile conflict
- pending verification state

## 20. Should tests be added now?

Yes.

Critical normalization and OTP flow behavior should be covered.
