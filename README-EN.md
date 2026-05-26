# auto-openai-account

[中文文档](README.md)

`auto-openai-account` is a local-first account automation console with a Go backend and a React web UI.

It is designed to run as one Go service that serves both:

- JSON API and Server-Sent Events under `/api/*`
- The React single-page app under `/`

The project is intended for local automation workflows, mailbox management, proxy configuration, registration/login jobs, runtime logs, and token export.

## Features

- Mailbox import and management
- Settings management
- Proxy pool configuration and connectivity testing
- Registration job creation, stop, progress, detail, and logs
- Login/token refresh jobs
- Runtime logs persisted in SQLite and streamed over SSE
- Token export for completed jobs
- Embedded React UI served by the Go service

## Tech Stack

- Backend: Go
- Storage: SQLite via `modernc.org/sqlite`
- Frontend: React, TypeScript, Vite, Tailwind CSS
- Runtime UI delivery: Go server serving Vite build output

## Project Layout

```text
apps/
  server/       Go server entrypoint
  web/          React + Vite frontend
internal/
  api/          HTTP routes, handlers, request/response formatting
  domain/       Shared business types, status constants, defaults
  storage/      SQLite schema and persistence
  runner/       Job lifecycle, concurrency, cancellation, logs, SSE fanout
  legacy/       Account automation implementation behind app boundaries
  proxypool/    Proxy connectivity tests
  webui/        Static UI handler
docs/
  api.md        Current API reference
  architecture.md
  design.md
  requirements.md
```

## Requirements

- Go 1.25 or newer, matching `go.mod`
- Node.js and npm for frontend development/builds

## Local Development

### Backend Only

Run the Go server:

```bash
go run ./apps/server
```

Default address:

```text
http://localhost:8080
```

Override the listen address with:

```bash
AUTO_OPENAI_ACCOUNT_LISTEN=:9090 go run ./apps/server
```

### Frontend Dev Server

In one terminal, run the backend:

```bash
go run ./apps/server
```

In another terminal, run Vite:

```bash
cd apps/web
npm run dev
```

The Vite dev server proxies `/api` requests to `http://localhost:8080`.

## Build

### Build Frontend Assets

```bash
cd apps/web
npm run build
```

The current Vite config outputs the built UI to the repository root `dist/` directory.

### Build Backend Binary

```bash
go build -o auto-openai-account ./apps/server
```

### Build Full Local Release

```bash
cd apps/web
npm run build
cd ../..
go build -o auto-openai-account ./apps/server
```

Run the built binary from the repository root so it can serve `dist/` and store runtime data under `data/`:

```bash
./auto-openai-account
```

Open:

```text
http://localhost:8080
```

## Deployment

For a simple single-machine deployment:

1. Build frontend assets with `cd apps/web && npm run build`.
2. Build the Go binary with `go build -o auto-openai-account ./apps/server`.
3. Deploy the binary together with the generated `dist/` directory.
4. Run the binary from the directory that contains `dist/`.
5. Persist the `data/` directory because it contains the SQLite runtime database.

Example runtime layout:

```text
release/
  auto-openai-account
  dist/
    index.html
    assets/
  data/
    register.db
```

Start with a custom port if needed:

```bash
AUTO_OPENAI_ACCOUNT_LISTEN=:8080 ./auto-openai-account
```

## Verification

Backend tests:

```bash
go test ./...
```

Frontend build check:

```bash
cd apps/web
npm run build
```

If a change touches both backend and frontend contracts, run both commands and verify `docs/api.md` and `apps/web/src/types.ts` stay aligned.

## Runtime Data

Runtime SQLite files are stored under `data/` by default:

```text
data/register.db
data/register.db-shm
data/register.db-wal
```

Do not delete `data/` unless you intentionally want to remove local runtime data.

## API Documentation

See `docs/api.md` for the current API contract.

Important endpoint groups:

- Health: `/api/health`
- Settings: `/api/settings`
- Mailboxes: `/api/mailboxes`
- Jobs: `/api/register-jobs`, `/api/login-jobs`
- Logs and events: `/api/register-jobs/{id}/logs`, `/api/register-jobs/{id}/events`
- Proxy testing: `/api/proxy/test`
- Stats: `/api/stats`

Errors use this JSON shape:

```json
{
  "error": "message"
}
```

## For Agents

Read `AGENTS.md` before making non-trivial changes.

Recommended context files:

- `AGENTS.md`
- `docs/requirements.md`
- `docs/architecture.md`
- `docs/design.md`
- `docs/api.md`

Development rules:

- Keep API handlers in `internal/api`.
- Keep shared JSON fields and status constants in `internal/domain`.
- Keep persistence in `internal/storage`.
- Keep job lifecycle, cancellation, runtime logs, and SSE in `internal/runner`.
- Keep frontend API types in `apps/web/src/types.ts` aligned with Go JSON types.
- Do not delete `data/` or unrelated working tree changes.

## Git Ignore Notes

Generated/runtime files should not be committed:

- `data/`
- `*.db`, `*.db-shm`, `*.db-wal`
- `node_modules/`
- frontend build output unless the task explicitly requires packaging artifacts

## Disclaimer

This project is provided only for programming learning and academic research, such as studying OAuth2 authorization mechanisms, TLS fingerprint security, and related defensive or adversarial techniques.

Do not use this project for any illegal purpose, large-scale malicious registration, or commercial activity that violates any platform's Terms of Service.

OpenAI's API risk-control strategies change frequently, and this project does not guarantee permanent availability. Any account bans, service restrictions, legal disputes, or other consequences caused by using this code are the user's sole responsibility and are unrelated to the developers.

## License

See `LICENSE`.
