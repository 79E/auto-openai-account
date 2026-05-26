# Architecture

## Shape

The project is a Go server with an embedded React UI.

```text
Go HTTP server
  /api/*       JSON API and SSE events
  /            React app
  /assets/*    React static assets
```

Production usage should only require starting the Go service.

## Project Layout

```text
apps/
  server/       Go entrypoint
  web/          React + Tailwind UI
internal/
  api/          HTTP routing and handlers
  domain/       Shared business types
  runner/       Job execution and cancellation
  storage/      SQLite persistence
  proxypool/    Proxy testing
  webui/        Embedded UI filesystem
docs/           Requirements, API, architecture, and design notes
```

## Module Boundaries

The first version uses internal provider-style boundaries instead of runtime plugin installation.

```text
Provider boundaries should stay isolated from API, UI, and storage code.
Default implementations are compiled into the binary.
Future providers can be added without rewriting API/UI/storage.
```

Recommended provider categories:

- Account provider: OpenAI register/login/token exchange.
- OTP provider: IMAP, Graph API, Gmail, manual input, temporary mail.
- Mail token provider: Outlook refresh token to IMAP access token.
- Proxy provider: proxy selection and health checks.
- Storage provider: SQLite first, optional Postgres later.

## Realtime Logs

Task logs are persisted to SQLite and broadcast in memory to active SSE clients.

The UI should load historical logs first and then subscribe to:

```text
GET /api/register-jobs/{id}/events
```

This avoids a blank waiting screen while a mailbox is registering.

## Static UI Delivery

React is built into:

```text
dist
```

The Go server serves that directory from the project root.
