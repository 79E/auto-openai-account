# Requirements

## Product Goal

`auto-openai-account` is a standard open-source, modular account automation console. The backend stays in Go, the UI uses React, shadcn/ui-style components, and Tailwind CSS. A production build should start with one Go service and serve both API and UI.

The business behavior should stay aligned with the existing desktop `goGPTRegisterAccount` project while allowing module extraction, small bug fixes, and clearer interfaces.

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

The chosen style is `02-modern-saas.html`: Modern SaaS Console.

- Light theme first.
- Blue/purple gradient accents.
- Card-based dashboard.
- Clean navigation and generous spacing.
- Table and log pages may use slightly higher density for efficiency.
- Realtime task logs should be visually prominent on job detail pages.

## Proxy Requirements

Settings must support multiple proxies.

- `proxy_mode`: `random`, `round_robin`, or `single`.
- `proxies`: list of proxy URLs.
- Supported schemes: `http`, `https`, `socks5`, `socks5h`.
- Default strategy: `random`.
- Each mailbox execution should select a proxy according to the strategy.

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
- `GET /api/stats`
