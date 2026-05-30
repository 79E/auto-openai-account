# API Reference

This document describes the current API implemented by `internal/api/server.go`.

Base path: `/api`

All JSON responses use `Content-Type: application/json; charset=utf-8` unless noted otherwise.

## Error Format

Errors are returned as JSON:

```json
{
  "error": "message"
}
```

Unsupported methods return `405 Method Not Allowed`. Missing resources generally return `404`.

## Common Types

### Settings

```json
{
  "proxy_groups": [
    {
      "id": "d93f40f760db4c90a0df873b59f90f34",
      "name": "us-residential",
      "mode": "round_robin",
      "proxies": ["http://127.0.0.1:8080"]
    }
  ],
  "password_mode": "random",
  "fixed_password": "Mima1234567890.",
  "register_concurrency": 1,
  "otp_timeout_seconds": 180,
  "otp_poll_interval_seconds": 5,
  "imap_host": "outlook.office365.com",
  "imap_port": 993,
  "imap_auth_mode": "auto",
  "listen": ":8080",
  "sms_configs": [
    {
      "id": "90b1607f4ff6473ea554787b03d7dc2d",
      "name": "smsbower-main",
      "type": "provider",
      "platform": "smsbower",
      "api_key": "secret",
      "service_id": "dr",
      "country_id": 38,
      "max_price": 0
    },
    {
      "id": "4f06fcbfd45c4be8af6fdf2cc7fd0c11",
      "name": "boss100-main",
      "type": "pool",
      "platform": "custom",
      "platform_label": "boss100",
      "max_usage_per_phone": 3,
      "disable_on_error": "permanent_only",
      "pool_summary": {
        "total_count": 100,
        "ready_count": 72,
        "reserved_count": 2,
        "used_up_count": 20,
        "disabled_count": 6,
        "error_count": 0,
        "remaining_uses": 144
      }
    }
  ]
}
```

Allowed values:

- `proxy_groups[].mode`: `random` or `round_robin`
- `password_mode`: `random`, `fixed`
- `imap_auth_mode`: `auto`, `password`, `xoauth2`
- `sms_configs[].platform`: `smsbower` or `hero-sms`
- `sms_configs[].type`: `provider` or `pool`
- `sms_configs[].disable_on_error`: `permanent_only` or `any_failure`
- `sms_configs[].max_price`: `0` means the API request does not send a max price limit

Identity note:

- `proxy_groups[].id` and `sms_configs[].id` are stable unique identifiers used by task creation.
- `name` remains user-facing display text and should still stay unique for easier management.

Compatibility note:

- Old databases may still contain `proxy_mode` and `proxies` internally.
- The current settings API no longer returns those legacy fields.
- When old data is loaded without `proxy_groups`, the server/frontend normalize it into one `默认分组` for migration.

### Mailbox

```json
{
  "id": 1,
  "email": "user@example.com",
  "password": "mailbox-password",
  "client_id": "client-id",
  "access_token": "access-token",
  "status": "new",
  "status_text": "新导入",
  "register_password": "account-password",
  "token_json": "{}",
  "remark": "",
  "last_error": "",
  "current_step": "",
  "current_step_index": 0,
  "current_step_total": 0,
  "proxy": "",
  "registered_at": "",
  "last_login_at": "",
  "phone_number": "",
  "last_job_id": 0,
  "last_job_type": "",
  "last_job_status": "",
  "last_job_error": "",
  "created_at": "2026-05-26T00:00:00Z",
  "updated_at": "2026-05-26T00:00:00Z"
}
```

Some empty fields may be omitted.

Mailbox statuses:

- `new`
- `registering`
- `registered`
- `logining`
- `abnormal`

### RegisterJob

```json
{
  "id": 1,
  "type": "register",
  "status": "running",
  "requested_count": 1,
  "total_count": 1,
  "success_count": 0,
  "failed_count": 0,
  "success_rate": 0,
  "avg_duration_ms": 0,
  "total_duration_ms": 0,
  "started_at": "2026-05-26T00:00:00Z",
  "finished_at": "",
  "created_at": "2026-05-26T00:00:00Z",
  "updated_at": "2026-05-26T00:00:00Z",
  "items": []
}
```

Job types:

- `register`
- `register_login`
- `register_codex`
- `login`
- `codex_login`

Job statuses:

- `running`
- `finished`
- `stopped`
- `failed`

### RegisterJobItem

```json
{
  "id": 1,
  "job_id": 1,
  "mailbox_id": 1,
  "email": "user@example.com",
  "status": "running",
  "error": "",
  "duration_ms": 0,
  "started_at": "2026-05-26T00:00:00Z",
  "finished_at": "",
  "created_at": "2026-05-26T00:00:00Z",
  "updated_at": "2026-05-26T00:00:00Z"
}
```

Item statuses:

- `pending`
- `running`
- `success`
- `failed`

### RuntimeLog

```json
{
  "id": 1,
  "job_id": 1,
  "mailbox_id": 1,
  "email": "user@example.com",
  "level": "info",
  "step": "send_otp",
  "step_index": 3,
  "step_total": 8,
  "message": "runtime message",
  "created_at": "2026-05-26T00:00:00Z"
}
```

### ProxyTestResult

```json
{
  "proxy": "http://127.0.0.1:8080",
  "ok": true,
  "ip": "203.0.113.10",
  "latency_ms": 320,
  "error": ""
}
```

`ip` and `error` may be omitted when empty.

### SMSCatalog

```json
{
  "services": [
    {
      "code": "dr",
      "name": "OpenAI"
    }
  ],
  "countries": [
    {
      "id": 38,
      "rus": "",
      "eng": "United States",
      "chn": "美国"
    }
  ]
}
```

SMS service and country lists are fetched from the selected SMS provider.

### PhonePoolItem

```json
{
  "id": 1,
  "sms_config_id": "4f06fcbfd45c4be8af6fdf2cc7fd0c11",
  "phone_number": "+18352622848",
  "code_fetch_url": "https://example.com/api/record?token=...",
  "status": "ready",
  "use_count": 1,
  "max_use_count": 3,
  "last_error": "",
  "last_job_id": 12,
  "last_mailbox_id": 8,
  "reserved_at": "",
  "last_used_at": "2026-05-26T00:00:00Z",
  "created_at": "2026-05-26T00:00:00Z",
  "updated_at": "2026-05-26T00:00:00Z"
}
```

Phone pool statuses:

- `ready`
- `reserved`
- `used_up`
- `disabled`
- `error`

## Endpoints

### GET /api/health

Returns service health.

Response:

```json
{
  "ok": true,
  "service": "auto-openai-account"
}
```

### GET /api/settings

Returns normalized settings.

Response: `Settings`

Pool configs include `pool_summary`, which is derived from phone pool records.

### PUT /api/settings

Updates settings. `POST` is also accepted by the current handler.

Request body: `Settings`

### POST /api/sms-configs/{id}/phone-pool/import

Imports phone numbers into a pool SMS config.

Request body:

```json
{
  "text": "+18352622848----https://example.com/api/record?token=..."
}
```

Supported line formats:

- `手机号----链接`
- `手机号|链接`
- `手机号:链接` where the right side starts with `http://` or `https://`

Phone numbers are normalized to a `+` prefixed format before storage.

Response:

```json
{
  "imported": 10,
  "skipped": 2,
  "failed": 1,
  "errors": ["line 3: invalid format"]
}
```

### GET /api/sms-configs/{id}/phone-pool

Lists phone pool items for one SMS config.

Query params:

- `page`
- `page_size`
- `status`

Response:

```json
{
  "total": 100,
  "items": [PhonePoolItem]
}
```

Response:

```json
{
  "ok": true,
  "settings": {}
}
```

`settings` is the normalized saved `Settings` object.

### POST /api/sms/catalog

Fetches SMS service and country lists from the selected provider.

Request body:

```json
{
  "platform": "smsbower",
  "api_key": "secret"
}
```

Response: `SMSCatalog`

Supported platforms:

- `smsbower`
- `hero-sms`

### POST /api/mailboxes/import

Imports mailbox credentials from text.

Request body:

```json
{
  "text": "user@example.com----password----client-id----access-token"
}
```

Supported line formats:

- `email----password`
- `email----password----client_id_or_access_token`
- `email----password----client_id----access_token`

Response:

```json
{
  "imported": 1,
  "skipped": 0,
  "failed": 0,
  "errors": []
}
```

### GET /api/mailboxes

Lists mailboxes.

Query parameters:

- `status`: optional mailbox status filter
- `page`: optional page number, default `1`
- `page_size`: optional page size, default `50`, maximum `200`

Response:

```json
{
  "total": 1,
  "items": []
}
```

`items` is an array of `Mailbox`.

### GET /api/mailboxes/{id}

Returns one mailbox.

Response:

```json
{
  "item": {}
}
```

`item` is a `Mailbox`.

### PUT /api/mailboxes/{id}

Updates editable mailbox fields. `POST` is also accepted by the current handler.

Request body can include:

```json
{
  "email": "user@example.com",
  "password": "mailbox-password",
  "client_id": "client-id",
  "access_token": "access-token",
  "remark": "note",
  "register_password": "account-password",
  "status": "new"
}
```

Response:

```json
{
  "item": {}
}
```

`item` is the updated `Mailbox`.

### DELETE /api/mailboxes/{id}

Deletes one mailbox.

Response:

```json
{
  "ok": true
}
```

### GET /api/mailboxes/{id}/token

Returns parsed token JSON for one mailbox.

Response:

```json
{
  "email": "user@example.com",
  "token_json": {}
}
```

If `token_json` is empty or cannot be parsed, `token_json` may be `null`.

### POST /api/mailboxes/{id}/login

Starts a login job for a single mailbox.

Response status: `202 Accepted`

Response:

```json
{
  "ok": true,
  "queued": true,
  "job": {}
}
```

`job` is a `RegisterJob` with type `login`.

### GET /api/register-jobs

Lists jobs.

Query parameters:

- `page`: optional page number, default `1`
- `page_size`: optional page size, default `5`, maximum `200`

Response:

```json
{
  "total": 1,
  "items": []
}
```

`items` is an array of `RegisterJob` without `items` details.

### POST /api/register-jobs

Starts a registration job.

Request body:

```json
{
  "count": 1,
  "flow": "register_login",
  "sms_config_id": "90b1607f4ff6473ea554787b03d7dc2d",
  "proxy_group_id": "d93f40f760db4c90a0df873b59f90f34"
}
```

Allowed `flow` values:

- `register`: register only, no token login
- `register_login`: register, then normal login token exchange
- `register_codex`: register, then normal login token exchange, then Codex authorization login

If `flow` is omitted, the server uses `register_login` for compatibility.
`register_codex` requires `sms_config_id`; missing or unknown SMS config returns `400` before creating a job.
Omit `proxy_group_id` or pass an empty string to use direct local network.
For backward compatibility, the server still accepts `sms_config_name` and `proxy_group_name` if the corresponding `*_id` field is omitted.

Response: `RegisterJob`

### GET /api/register-jobs/{id}

Returns a job with item details.

Response: `RegisterJob` with `items`.

### POST /api/register-jobs/{id}/stop

Stops a running job.

Response:

```json
{
  "ok": true,
  "job": {}
}
```

`job` is the recalculated `RegisterJob`.

### GET /api/register-jobs/{id}/logs

Returns historical runtime logs for a job.

Response:

```json
{
  "items": []
}
```

`items` is an array of `RuntimeLog`. The current handler returns up to 300 logs.

### GET /api/register-jobs/{id}/events

Streams live runtime logs using Server-Sent Events.

Response content type: `text/event-stream`

Event format:

```text
event: log
data: {"id":1,"job_id":1,"message":"runtime message"}
```

Each `data` payload is a `RuntimeLog` JSON object.

### GET /api/register-jobs/{id}/tokens

Exports token payloads for successful items in a finished or stopped job.

Response:

```json
{
  "count": 1,
  "items": []
}
```

The endpoint returns `400` unless the job status is `finished` or `stopped`.

### POST /api/login-jobs

Starts a login job for one or more mailboxes.

Request body:

```json
{
  "mailbox_ids": [1, 2],
  "flow": "login",
  "sms_config_id": "90b1607f4ff6473ea554787b03d7dc2d",
  "proxy_group_id": "d93f40f760db4c90a0df873b59f90f34"
}
```

Allowed `flow` values:

- `login`: normal login token exchange
- `codex_login`: Codex authorization login

If `flow` is omitted, the server uses `login` for compatibility.
Both login flows require the mailbox to have an OpenAI login password.
`codex_login` requires `sms_config_id`; missing or unknown SMS config returns `400` before creating a job.
Omit `proxy_group_id` or pass an empty string to use direct local network.
For backward compatibility, the server still accepts `sms_config_name` and `proxy_group_name` if the corresponding `*_id` field is omitted.

Response status: `202 Accepted`

Response: `RegisterJob` with type matching `flow`.

### POST /api/proxy/test

Tests one proxy or multiple proxies by requesting `https://api.ipify.org?format=json` through each candidate.

Request body for one proxy:

```json
{
  "proxy": "http://127.0.0.1:8080",
  "timeout_seconds": 15
}
```

Request body for multiple proxies:

```json
{
  "proxies": ["http://127.0.0.1:8080"],
  "timeout_seconds": 15
}
```

Supported proxy schemes:

- `http`
- `https`
- `socks5`
- `socks5h`

Response:

```json
{
  "items": []
}
```

`items` is an array of `ProxyTestResult`.

### GET /api/stats

Returns dashboard summary counts.

Response:

```json
{
  "mailboxes": {
    "new": 0,
    "registering": 0,
    "registered": 0,
    "logining": 0,
    "abnormal": 0
  },
  "jobs": {
    "total": 0,
    "running": 0,
    "finished": 0,
    "stopped": 0,
    "failed": 0
  }
}
```
