# Contributing to Flagstone

Thanks for taking the time to contribute.

## Before you start

- For bug fixes and small improvements, open a PR directly.
- For new features or significant changes, open an issue first so we can align on direction before you invest time writing code.

## Development setup

See the [README](README.md#development-setup) for the full setup guide. The short version:

```bash
# Backend
cp .env.example .env
docker compose up -d
go run ./cmd/flagstone

# Frontend
cd web && npm install && npm run dev
```

## Quality checks

Run these before pushing. All of them must pass for a PR to be merged.

### Backend

```bash
gofmt -l ./...          # should print nothing
go vet ./...            # should print nothing
go test ./...           # all tests pass
```

### Frontend

```bash
cd web
npm run typecheck       # tsc --noEmit, zero errors
npm run lint            # ESLint, zero warnings policy
npm run format:check    # Prettier check, no diffs
npm run test:run        # Vitest unit + integration tests
npm run test:e2e        # Playwright E2E (requires the app running)
```

If you're fixing a bug, add a test that would have caught it. If you're adding a feature, cover the happy path and any obvious edge cases.

## Commits

- One logical change per commit.
- Write the commit message in imperative mood: `add rate limiting to auth endpoint`, not `added` or `adds`.
- Reference the issue number if one exists: `fix flag toggle race condition (#42)`.

## Pull request checklist

- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes
- [ ] `npm run typecheck` passes (zero errors)
- [ ] `npm run lint` passes (zero warnings)
- [ ] `npm run format:check` passes
- [ ] `npm run test:run` passes
- [ ] `npm run test:e2e` passes (if the change touches UI or API behavior)
- [ ] API changes are reflected in the relevant docs
- [ ] PR description explains what changed and why

## License

By contributing, you agree that your contributions will be licensed under the [GNU Affero General Public License v3.0](LICENSE).
