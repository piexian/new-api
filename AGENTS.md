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

## Deployment

The production instance on this server runs as a **systemd service** named `newapi.service`.

- Build the binary with the version ldflag:
  `go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api main.go`
- Deploy / restart: `systemctl restart newapi.service`
- Check status: `systemctl status newapi.service`
- View logs: `journalctl -u newapi.service -f` or files under `/root/new-api/logs/`
- **Never `kill` the process directly** — always use `systemctl` to manage the service.
- The service unit file is at `/etc/systemd/system/newapi.service` (Restart=always, port 3000).

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

### Repository History Hygiene

Git history must contain only changes that are directly part of the project
deliverable. Do not stage or commit agent orchestration metadata, local task
tracking, scratch files, generated analysis, or tool state unless the user
explicitly asks for those files to be versioned.

- Keep `.ccg/tasks/**` local, including archived tasks, `task.json`,
  `requirements.md`, `plan.md`, `review.md`, and `context.jsonl`.
- Read-only investigations, audits, explanations, and status checks do not
  authorize creating a commit.
- Local task creation or archival requirements do not imply permission to add
  those artifacts to Git history. This rule overrides workflow instructions
  that would otherwise automatically commit `.ccg` task metadata.
- Before every commit, inspect the staged file list and remove unrelated agent
  or tool-generated files from the index.

## Security & Agent-Specific Rules

Never commit secrets or expose API keys. Do not rename, remove, or replace
protected project identity or attribution references related to **nеw-аρi** or
**QuаntumΝоuѕ**. For billing expression work, read `pkg/billingexpr/expr.md`
first. When adding a new channel, verify whether `StreamOptions` is supported
and update `streamSupportedChannels` when applicable.

## i18n & Localization

New API has two i18n layers: backend Go (`i18n/`) and frontend React (`web/*/src/i18n/`).

### Backend i18n

- Message keys are defined in `i18n/keys.go` as `MsgXxx` constants.
- Translations live in `i18n/locales/{zh-CN,zh-TW,en}.yaml`.
- Use `common.ApiErrorI18n(c, i18n.MsgXxx)` for error responses.
- Use `i18n.T(c, i18n.MsgXxx)` for inline translated strings.
- Default language is Chinese (`DefaultLang = LangZhCN`).

### Frontend i18n

- `web/default/` uses English keys with a custom sync script (`bun run i18n:sync`).
- `web/classic/` uses Chinese keys with `i18next-cli` (`bun run i18n:extract`).
- After changing UI text, run the appropriate sync/extract command and fill in
  translations for all supported locales.

### i18n Check Script

Run `bash scripts/check-i18n.sh` to verify:

1. No English hardcoded messages in controller responses.
2. All i18n keys in `keys.go` have translations in all yaml locale files.
3. Frontend default theme has no untranslated keys.
4. Frontend classic theme has no missing t() keys.
5. No duplicate i18n key definitions.

Always run this script after modifying controller error messages or frontend UI text.
