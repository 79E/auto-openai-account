# Design

## Product Experience

`auto-openai-account` should feel like a focused operations console for running and monitoring account automation tasks.

The UI should optimize for:

- Fast status recognition
- Clear task progress
- Readable runtime logs
- Safe bulk operations
- Simple local configuration

The product is a tool, not a marketing site. Visual polish should support operational clarity.

## Visual Direction

- Light theme first.
- Use blue and purple accents for primary actions, highlights, and progress states.
- Prefer rounded cards, soft borders, subtle shadows, and calm spacing.
- Keep dense data areas efficient, especially tables and logs.
- Avoid heavy enterprise-admin styling, dark chrome, and decorative noise.

## Layout Rules

- Use a stable top-level navigation structure.
- Keep dashboard summaries visible without forcing the user to scroll on common desktop sizes.
- Put primary actions near the data they affect.
- For long-running tasks, keep progress and logs close together.
- On mobile, stack controls before data tables and avoid horizontal overflow where possible.

## Components

Use existing React + TypeScript components and CSS module patterns unless there is a clear reason to add new primitives.

Preferred component behaviors:

- Cards for summary, configuration, and grouped workflows.
- Badges for statuses and job types.
- Modals for create/edit flows that require confirmation or multiple fields.
- Tables for mailbox and job lists.
- Toasts for short success or failure feedback.
- Log panels for chronological runtime output.

## Pages

### Overview

- Show key counts and recent job status clearly.
- Prefer summary cards over large raw tables.
- Surface abnormal mailboxes and running work prominently.

### Mailboxes

- Optimize for scanning, filtering, editing, deleting, resetting, and starting login tasks.
- Keep sensitive values readable only when the user intentionally opens details.
- Show current status, current step, last job result, and last error when available.

### Jobs

- Show job type, status, counts, success rate, and duration.
- Job detail should show both item-level progress and runtime logs.
- Runtime logs should be readable and visually distinct from static metadata.

### Proxy Pool

- Make proxy format validation immediate and understandable.
- Show test status, latency, detected IP, and error text.
- Bulk testing should not hide individual proxy results.

### Plugins Or Providers

- Treat provider/plugin UI as configuration and capability discovery.
- Avoid presenting unsupported runtime installation as available behavior.

## Status Language

Use consistent labels for state.

- `new`: newly imported or unused mailbox
- `registering`: registration in progress
- `registered`: usable registered mailbox
- `logining`: login/token refresh in progress
- `abnormal`: failed or needs attention
- `running`: job is active
- `finished`: job completed
- `stopped`: job was manually stopped
- `failed`: job failed

## Interaction Rules

- Destructive actions require either a confirmation or a clearly reversible workflow.
- Long-running actions should set visible busy state.
- Errors should explain what failed and preserve useful backend messages.
- Avoid silently swallowing API errors.
- After mutations, refresh affected lists and detail views.

## Frontend Implementation Notes

- Use React + TypeScript.
- Keep API calls routed through `apps/web/src/lib/api.ts` where practical.
- Keep shared API types in `apps/web/src/types.ts`.
- Prefer existing components before adding new ones.
- Do not add a new UI framework unless explicitly requested.
- Maintain desktop and mobile usability for every user-facing page.
