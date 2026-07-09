# Repository Guidelines

## Project Structure & Module Organization

`new-api` is a Go API gateway with an embedded admin dashboard. The Go module is
`github.com/QuantumNous/new-api`. Backend code follows
`router -> controller -> service -> model`: routes live in `router/`, handlers in
`controller/`, business logic in `service/`, and persistence in `model/`.
Provider relay code is under `relay/`, with adapters in `relay/channel/`. Shared
utilities and contracts live in `common/`, `dto/`, `constant/`, `types/`,
`setting/`, `middleware/`, `oauth/`, `i18n/`, and `pkg/`.

Frontend themes are under `web/`: `web/default/` is React 19, TypeScript,
Rsbuild, Base UI, and Tailwind CSS; `web/classic/` is the legacy React 18 theme.
Frontend translations are flat JSON files in
`web/default/src/i18n/locales/{lang}.json`.

## Frontend Parity

User-facing frontend changes must usually be implemented in both
`web/default/` and `web/classic/`. Keep behavior, routes, permissions, API usage,
validation, and visible copy aligned across both themes. Only skip one frontend
when the change is explicitly theme-specific, and state that in the PR.

## Build, Test, and Development Commands

- `go run main.go` starts the backend locally.
- `go build ./...` compiles all Go packages.
- `go test ./...` runs backend unit tests.
- `make dev-api` starts the Docker-based API stack from `docker-compose.dev.yml`.
- `make dev-web` runs the default frontend dev server.
- `make build-all-frontends` builds both frontend themes.
- From `web/default/`, use `bun run build`, `bun run typecheck`,
  `bun run lint`, `bun run format:check`, and `bun run i18n:sync`.

Use Bun for frontend dependency and script work.

## Coding Style & Naming Conventions

Format Go with `gofmt`; keep package names short and lowercase. Do not call
`encoding/json` marshal/unmarshal functions directly in business code; use
`common.Marshal`, `common.Unmarshal`, `common.UnmarshalJsonStr`,
`common.DecodeJson`, or `common.GetJsonType`.

All database changes must support SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6.
Prefer GORM APIs; when raw SQL is unavoidable, branch on the database helpers in
`common/` and reuse quoted-column helpers from `model/main.go`.

For upstream relay request DTOs, optional scalar JSON fields must be pointer
types with `omitempty` so explicit `0`, `0.0`, or `false` values survive
round-trips. In `web/default/`, follow ESLint and Prettier: 2-space indent,
single quotes, no semicolons, sorted imports, and no `console` calls.

## Testing Guidelines

Place Go tests beside the package under test as `*_test.go`. Add focused tests
for relay conversion, billing, auth, database edge cases, and provider adapters.
When changing request DTOs, test both absent fields and explicit zero values.
For frontend changes, run typecheck, lint, build, and `bun run i18n:sync` when UI
text changes. Validate both frontends when the change affects shared behavior.

## Commit & Pull Request Guidelines

Recent history uses Conventional Commit style, for example `feat: ...`,
`fix: ...`, `feat(auth): ...`, and `fix(classic): ...`. Keep subjects short and
imperative. Pull requests should describe the behavior change, list validation
commands, link related issues, call out database compatibility work, and include
screenshots for visible UI changes.

## Security & Agent-Specific Rules

Never commit secrets or expose API keys. Do not rename, remove, or replace
protected project identity or attribution references related to **nеw-аρi** or
**QuаntumΝоuѕ**. For billing expression work, read `pkg/billingexpr/expr.md`
first. When adding a new channel, verify whether `StreamOptions` is supported
and update `streamSupportedChannels` when applicable.
