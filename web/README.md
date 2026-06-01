# Flagstone Web

Frontend for Flagstone — a feature flag management platform built with Next.js 16.

## Prerequisites

- Node.js 22+
- A running Flagstone API backend (default: `http://localhost:8080`)

## Quick Start

```bash
cp .env.local .env.local  # already configured for local dev
npm install
npm run dev               # starts on http://localhost:3002
```

Open http://localhost:3002. If the backend has demo data:

- **Email:** `admin@acme.com`
- **Password:** `password123`

## Environment Variables

| Variable              | Default                    | Description                    |
|-----------------------|----------------------------|--------------------------------|
| `BACKEND_URL`         | `http://localhost:8080`    | Flagstone API backend          |
| `NEXT_PUBLIC_APP_URL` | `http://localhost:3000`    | Public frontend URL (for APIs) |

## Available Scripts

| Command               | Description                            |
|-----------------------|----------------------------------------|
| `npm run dev`         | Start dev server (port 3002)           |
| `npm run build`       | Production build                       |
| `npm run start`       | Start production server (port 3002)    |
| `npm run lint`        | ESLint (0 warnings policy)             |
| `npm run typecheck`   | `tsc --noEmit`                        |
| `npm run format`      | Prettier format                        |
| `npm run test`        | Vitest (watch mode)                    |
| `npm run test:run`    | Vitest (single run, 103 tests)         |
| `npm run test:e2e`    | Playwright E2E tests                   |

## Project Structure

```
web/
├── app/                      # Next.js App Router pages
│   ├── (app)/                # Authenticated layout
│   │   └── projects/[slug]/
│   │       ├── flags/        # Flags list + rule editor
│   │       ├── api-keys/     # API key management
│   │       ├── segments/     # Segment targeting
│   │       ├── audit/        # Audit log
│   │       └── settings/     # Project settings
│   ├── login/                # Login page
│   ├── setup/                # First-time setup page
│   └── api/auth/             # Auth proxy routes (login, logout, setup)
├── components/
│   ├── layout/               # Sidebar, Topbar
│   ├── login/                # Login form
│   ├── projects/             # Project card, create dialog
│   ├── flags/                # Flags table, toggle, create dialog
│   ├── rules/                # Rule editor (ConditionRow, RuleCard, RuleEditor)
│   └── ui/                   # Base UI components (shadcn-style)
├── lib/
│   ├── api.ts                # API client (serverFetch, apiFetch, transformKeys)
│   ├── types.ts              # TypeScript types
│   ├── schemas.ts            # Zod validation schemas
│   └── utils.ts              # Utilities
├── __tests__/
│   ├── unit/                 # Unit tests
│   ├── integration/          # Integration tests
│   ├── e2e/                  # Playwright E2E tests
│   └── mocks/                # MSW handlers
├── proxy.ts                  # Auth middleware (Next.js 16 proxy)
└── playwright.config.ts      # E2E test configuration
```

## Architecture

### Auth & Proxying

`proxy.ts` (Next.js 16 `proxy` export) runs before every request:
- Unauthenticated users → redirected to `/login`
- Authenticated users on `/login` or `/setup` → redirected to `/projects`
- Auth API routes (`/api/auth/*`) proxy to the backend, setting/clearing the `access_token` cookie

### API Client

`lib/api.ts` provides two fetch wrappers:

- **`serverFetch<T>`** — server-side data fetching (uses `cookies()`)
- **`apiFetch<T>`** — client-side data fetching (reads `access_token` from document cookie)

Both apply `transformKeys` to convert backend `snake_case` responses to frontend `camelCase`. The backend returns bare arrays (not wrapped objects), so callers receive `T[]` directly.

### Snake→Camel Transform

The backend uses `snake_case` JSON keys. `transformKeys` recursively converts them to `camelCase` at the fetch boundary. No per-type transformation required.

### Rule Editor

The rule editor (`components/rules/`) implements the backend engine's tree-based condition model:

- `RolloutInput` — percentage slider (0–100)
- `ConditionRow` — attribute/operator/value triplet
- `RuleCard` — grouped conditions (AND logic) + rollout + return value
- `RuleEditor` — full page with env selector, dirty tracking, OCC save

## Testing

### Unit & Integration (Vitest + happy-dom)

```bash
npm run test:run            # 103 tests, 17 files
npm run test                # watch mode
npm run test:coverage       # with coverage
```

Tests use MSW to mock backend API calls. The handler at `__tests__/mocks/handlers.ts` simulates backend responses including conflict (409) scenarios.

### E2E (Playwright)

```bash
npm run test:e2e            # headless
npm run test:e2e:ui         # interactive UI mode
```

Playwright expects the backend to be running with demo data. The test server starts on port 3000 by default.

## Known Issues

- **DEP0205 warning**: `module.register()` is deprecated in Node.js 22 — harmless, affects Next.js compilation worker
- **LocalStorage warning**: Experimental API warning on server startup — harmless
- **W7 (Account page)**: Backend lacks `GET /api/v1/auth/me`, `PATCH /me/password`, `GET /auth/sessions`, `DELETE /sessions/:id` — blocked until backend implements these
- **W8 (Setup status)**: Backend has `POST /api/v1/setup` but no `GET /api/v1/setup/status` — setup page cannot auto-detect if initialization is needed
- **Audit endpoint**: `GET /api/v1/audit` returns 500 `INTERNAL_ERROR` — backend bug
- **Segments page**: Renders but backend segment API is minimal
- **Select in tests**: base-ui Select interactions are unreliable in happy-dom; some Select-dependent assertions are skipped in unit tests
