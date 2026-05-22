# AGENTS.md

## Build & Generate

```bash
# Required before first build or after model changes — generated code is not in git
go generate ./...

# Build server binary
go build

# Build debug TUI client (separate module under tools/client)
go build ./tools/client

# Docker build (static Linux binary output to out/)
docker build --output=out .
```

The Docker build runs `go generate ./...` then `go test ./...` before compiling. If `go generate` fails locally, check
`clients/model/gen.go` and `clients/model/meta.go` for the generate directives.

## Test

```bash
go test ./...                # server + service packages
go test ./tools/client/...   # TUI client only
```

## Run

```bash
# First time: create SQLite DB (file named "db" in cwd)
./aiagent --mode=migrate

# Start server (default port 8640, must provide a real DeepSeek API key)
./aiagent --DeepSeekAPIKey=sk-... --mode=server
```

The migrate command refuses to run if `db` already exists. Delete it first to re-migrate.

## Architecture

```
main.go                 — entrypoint, CLI flags, wires dependencies, starts HTTP server
clients/openai/         — hand-written DeepSeek/OpenAI HTTP client (no third-party SDK)
clients/model/          — GORM models + codegen triggers (gen.go, meta.go)
clients/generated/      — generated from `go tool gorm gen` (in meta.go)
clients/query/          — generated from `go run gen.go` (gorm/gen query builder)
clients/chat/           — Chat repository (GORM queries)
clients/session/        — Session repository (GORM queries + digest logic)
service/                — HTTP handlers using github.com/hyisen/wf router
service/chat/           — Chat orchestration: history loading, upstream calls, result persisting
console/                — REPL mode (interactive terminal chat)
tools/client/           — Standalone TUI client binary (separate main package)
```

## Code Generation (Two-Stage)

1. `clients/model/gen.go` → `clients/query/` (gorm/gen query builder via `go run`)
2. `clients/model/meta.go` → `clients/generated/` (custom raw SQL query via `gorm gen` tool)

Both are triggered by `go generate ./...`. Generated output is gitignored.

## Framework & Dependencies

- **HTTP router**: `github.com/hyisen/wf` (internal framework — path matching, SSE helpers, JSON handlers)
- **ORM**: `gorm.io/gorm` + `gorm.io/gen` (code-gen query builder) + SQLite
- **DB**: SQLite file named `db`, DDL embedded from `docs/ddl.sql` via `//go:embed`
- **No OpenAI SDK**: Custom client in `clients/openai/` — do not add `github.com/openai/openai-go`
- **No SSE library**: Custom SSE codec in the openai client — do not add third-party SSE packages

## API Conventions

- JSON serialization hides DB primary keys via `json:"-"` tag on model structs
- `WithID()` method variants expose ID fields when needed in API responses
- `?stream=true` query parameter switches between one-shot and SSE streaming on chat endpoints
- Chat stream timeout is 120s (vs 2s default for other routes)

## Streaming Chat

- The stream handler uses `context.WithoutCancel` (detached context) so client disconnect doesn't abort the upstream LLM
  request — the result is still saved to DB
- Stream translation happens in `service/chat/chat.go:translateAggregateSave` which maps upstream SSE chunks to
  `wf.MessageEvent` types: `head`, `role`, `cotEnd`, `finish`, `usage`, `error`

## CI

Two workflows (both ignore `master` branch):

- `ci.yml`: Docker build + deploy via amah (for `**/*.go`, excludes `tools/client/**`)
- `client.yml`: Go build cross-platform binaries for `tools/client/**` only

## Auth

Authentication is handled by an upstream gateway (amah). This server trusts the gateway — no auth middleware, no token
verification. User IDs come from the URL path (`/v2/users/{userID}/sessions`).

## Convention

- Read `docs/go-conventions.md` before reviewing Go code — it covers idioms like consumer-defined interfaces and `panic`
  for invariants.
