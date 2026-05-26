# AGENTS.md

## Project Summary

`auto-openai-account` is a Go backend with an embedded React UI for local account automation workflows.

Production should run as one Go service that serves:

- `/api/*` JSON API and SSE events
- `/` React app
- `/assets/*` React static assets

The backend stays in Go. The frontend stays in React, TypeScript, Vite, and Tailwind CSS. SQLite is the first storage backend.

## Read First

Before making non-trivial changes, read these files:

- `docs/requirements.md` for product scope and first-version requirements
- `docs/architecture.md` for module layout and boundaries
- `docs/design.md` for UI and interaction direction
- `docs/api.md` for current API contracts

## Project Layout

- `apps/server/main.go`: Go server entrypoint
- `apps/web`: React + Vite frontend
- `internal/api`: HTTP routing, request parsing, response formatting, SSE endpoint handlers
- `internal/domain`: shared business types, JSON fields, status constants, default settings
- `internal/storage`: SQLite schema, migrations, persistence queries, data mapping
- `internal/runner`: job creation, concurrency, cancellation, runtime logs, SSE fanout
- `internal/legacy`: account automation implementation kept behind current app boundaries
- `internal/proxypool`: proxy connectivity tests
- `internal/webui`: embedded frontend filesystem and static UI handler
- `docs`: product, architecture, API, and design notes

## Development Commands

Backend:

```bash
go test ./...
go run ./apps/server
```

Frontend:

```bash
cd apps/web
npm run dev
npm run build
```

The default server listen address is `:8080`. It can be overridden with `AUTO_OPENAI_ACCOUNT_LISTEN`.

## Verification Rules

Run the smallest useful verification for the files changed.

- Backend changes: `go test ./...`
- Frontend changes: `cd apps/web && npm run build`
- API contract changes: update `docs/api.md` and verify frontend types in `apps/web/src/types.ts`
- UI behavior changes: verify the affected page builds and follows `docs/design.md`

If both backend and frontend are changed, run both backend and frontend verification commands.

## Backend Rules

- Keep existing API response shapes stable unless the task explicitly requires a contract change.
- Put HTTP handlers, route branching, request validation, and response writing in `internal/api`.
- Put shared JSON structs, status constants, and defaults in `internal/domain`.
- Put SQLite schema and persistence behavior in `internal/storage`.
- Put job lifecycle, worker concurrency, cancellation, log broadcasting, and SSE subscription behavior in `internal/runner`.
- Do not move account automation implementation into API handlers.
- Keep SQLite local-first unless explicitly asked to add another storage backend.
- If changing JSON fields used by the UI, update both Go domain types and `apps/web/src/types.ts`.

## Frontend Rules

- Use React + TypeScript.
- Keep API helper behavior centralized in `apps/web/src/lib/api.ts` where practical.
- Keep shared frontend response types in `apps/web/src/types.ts`.
- Follow the product UI direction in `docs/design.md`.
- Preserve the existing visual language: light theme, blue/purple accents, rounded cards, clean navigation, readable logs.
- Do not introduce a new UI framework or component library unless explicitly requested.

## Data And Generated Files

- Runtime SQLite files live under `data/` and are ignored by git.
- Do not delete or reset `data/` unless explicitly asked.
- Do not commit `node_modules/`, `apps/web/dist/`, database files, or local cache/build artifacts.
- Keep build artifacts out of source changes unless the task is specifically about release packaging.

## API Notes

- API documentation lives in `docs/api.md`.
- Errors are JSON objects shaped as `{ "error": "message" }`.
- Job logs are persisted in SQLite and can also stream through SSE.
- Registration and login jobs currently share the `register_jobs` storage model with a `type` field.

## Do Not Do Unless Asked

- Do not rewrite the whole architecture for a small feature.
- Do not casually change API response shapes or status values.
- Do not introduce multi-user authentication; it is out of first-version scope.
- Do not add old database compatibility unless explicitly requested.
- Do not remove existing user data, runtime database files, or unrelated working tree changes.

## Good Task Workflow

For any feature or bug fix:

1. Identify the affected module boundaries.
2. Read the relevant existing code before editing.
3. Make the smallest correct change.
4. Keep backend and frontend types/contracts aligned.
5. Run the relevant verification commands.
6. Update docs when behavior, design, or API contracts change.
