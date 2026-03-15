# TODO

## Email Verification

- add `email_verified` to the user model
- decide whether email verification should be mandatory during signup or after signup
- add email verification request and verify endpoints
- add resend and expiry policy for email verification
- decide whether legacy users need an `email_verified` backfill strategy when email verification is introduced
- update frontend integration docs for the email verification flow
