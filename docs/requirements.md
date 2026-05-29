# Requirements

## Product Goal

`auto-openai-account` is a standard open-source, modular account automation console. The backend stays in Go, the UI uses React, shadcn-style components, and Tailwind CSS. A production build should start with one Go service and serve both API and UI.

The project is a complete standalone product. Product behavior, API contracts, and UI decisions should be documented inside this repository.

## First Version Scope

- Go HTTP API server.
- React Modern SaaS Console UI.
- UI served by the Go server from embedded/static build assets.
- SQLite local storage, starting from a new database.
- Mailbox import and management.
- Settings management.
- Multiple proxy configuration with random proxy selection.
- Register job creation, stop, progress, details, and logs.
- Per-mailbox live step logs, so the UI does not wait blindly.
- Internal plugin-style module boundaries for providers.

## Out Of Scope For First Version

- Runtime plugin installation marketplace.
- Old database compatibility.
- Full open-source compliance text and disclaimer, to be added after the project is complete.
- Multi-user authentication.

## UI Requirements

The product UI should follow `docs/design.md`.

- Light theme first.
- Blue/purple gradient accents.
- Card-based dashboard.
- Clean navigation and generous spacing.
- Table and log pages may use slightly higher density for efficiency.
- Realtime task logs should be visually prominent on job detail pages.

## Proxy Requirements

Settings must support multiple proxies.

- `proxy_groups`: grouped proxy pools with unique group names.
- Each group has `mode`: `random` or `round_robin`.
- Each group has `proxies`: list of proxy URLs.
- Supported schemes: `http`, `https`, `socks5`, `socks5h`.
- Task creation must allow choosing direct local network or one proxy group.
- Local network starts directly without proxy pre-check.
- Proxy group execution must pre-test candidate proxies before starting mailbox work.
- `random` group mode picks one working proxy; mailbox fails immediately if that proxy later fails.
- `round_robin` group mode should move to the next proxy only when the current request fails because of the proxy, and retry that same step instead of restarting the whole mailbox flow.
- Once a mailbox task has passed its first proxy-validated step, the rest of that mailbox flow should stay on the selected proxy unless the proxy itself fails.

## Runtime Log Requirements

Every important account operation should emit structured logs.

- `job_id`
- `mailbox_id`
- `email`
- `level`
- `step`
- `step_index`
- `step_total`
- `message`
- `created_at`

The UI must be able to display:

- Current job progress.
- Current running mailbox step.
- Historical job logs.
- Live job logs through Server-Sent Events.

## First API Contract

- `GET /api/health`
- `GET /api/settings`
- `PUT /api/settings`
- `POST /api/mailboxes/import`
- `GET /api/mailboxes`
- `GET /api/mailboxes/{id}`
- `PUT /api/mailboxes/{id}`
- `DELETE /api/mailboxes/{id}`
- `GET /api/mailboxes/{id}/token`
- `POST /api/mailboxes/{id}/login`
- `GET /api/register-jobs`
- `POST /api/register-jobs`
- `GET /api/register-jobs/{id}`
- `POST /api/register-jobs/{id}/stop`
- `GET /api/register-jobs/{id}/logs`
- `GET /api/register-jobs/{id}/events`
- `GET /api/register-jobs/{id}/tokens`
- `POST /api/login-jobs`
- `POST /api/proxy/test`
- `GET /api/stats`
