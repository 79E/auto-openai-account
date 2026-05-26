# Mapping From goGPTRegisterAccount

Reference project path:

```text
/Users/qy/Desktop/goGPTRegisterAccount
```

## File Mapping

| Old File                 | Responsibility                     | New Module                                                          |
| ------------------------ | ---------------------------------- | ------------------------------------------------------------------- |
| `api.go`                 | HTTP handlers                      | `internal/api`                                                      |
| `models.go`              | Settings, mailbox, job types       | `internal/domain`                                                   |
| `store.go`               | SQLite persistence                 | `internal/storage`                                                  |
| `runner.go`              | Batch job runner                   | `internal/runner`                                                   |
| `flow.go`                | Register/login orchestration       | future `internal/account`                                           |
| `main.go` worker methods | OpenAI/Auth/Sentinel requests      | future `internal/providers/openai`                                  |
| `imap.go`                | IMAP OTP and Outlook token refresh | future `internal/plugins/otp_imap`, `internal/plugins/mail_outlook` |
| `proxy.go`               | HTTP/SOCKS proxy client            | future `internal/network`                                           |
| `util.go`                | Helpers, logging, errors           | split into domain/storage/api/provider helpers                      |

## API Mapping

The new project keeps the same first API surface where possible and adds log/event endpoints.

| Old API                              | New API                              | Status   |
| ------------------------------------ | ------------------------------------ | -------- |
| `GET /api/health`                    | same                                 | kept     |
| `GET/PUT /api/settings`              | same, extended with proxy pool       | extended |
| `POST /api/mailboxes/import`         | same                                 | kept     |
| `GET /api/mailboxes`                 | same                                 | kept     |
| `GET/PUT/DELETE /api/mailboxes/{id}` | same                                 | kept     |
| `GET /api/mailboxes/{id}/token`      | same                                 | kept     |
| `POST /api/mailboxes/{id}/login`     | same                                 | kept     |
| `GET/POST /api/register-jobs`        | same                                 | kept     |
| `GET /api/register-jobs/{id}`        | same                                 | kept     |
| `POST /api/register-jobs/{id}/stop`  | same                                 | kept     |
| none                                 | `GET /api/register-jobs/{id}/logs`   | added    |
| none                                 | `GET /api/register-jobs/{id}/events` | added    |
| `GET /api/stats`                     | same                                 | kept     |

## Register Flow Mapping

Old `RegisterOne` steps:

| Step | Old Function             | New Log Step               |
| ---- | ------------------------ | -------------------------- |
| 1    | `platformAuthorize`      | `platform_authorize`       |
| 2    | `registerUser`           | `submit_register_password` |
| 3    | `sendOTP`                | `send_otp`                 |
| 4    | `OTPFetcher`             | `wait_otp`                 |
| 5    | `validateOTP`            | `validate_otp`             |
| 6    | `createAccount`          | `create_account_profile`   |
| 7    | `loginAndExchangeTokens` | `token_exchange`           |
| 8    | `MarkMailboxRegistered`  | `complete`                 |

The current scaffold emits these same step names so the UI and storage model are ready for the real provider migration.

## Important Compatibility Notes

- The new database starts empty.
- The old single proxy setting becomes a proxy pool.
- Runtime logs are structured and persisted instead of only printing to stdout.
- The OpenAI/Auth/Sentinel implementation should be migrated into an `openai` provider without changing UI/API contracts.
