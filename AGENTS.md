# AGENTS.md

Guidance for AI agents (and humans) working on `dank-mcp`. User-facing documentation lives in [`README.md`](./README.md); this file is for contributors modifying the code.

## What this repo is

`dank-mcp` is an MCP server that exposes cannabis datasets to LLMs via SQL over DuckDB. It is published under the [AgentDank](https://github.com/AgentDank) organization.

## Current state

- **No datasets are bundled.** The prior hand-rolled Connecticut (CT) brands handlers were removed in the transition to a generic binding architecture.
- The server registers one MCP tool, `query`, which executes arbitrary SQL against DuckDB and returns CSV.
- DuckDB is opened with `access_mode=read_only` **and** has `SET enable_external_access=false` applied via `db.RunSafeMode`. Both layers matter; don't remove one assuming the other covers it.

## Architectural direction

The project is moving from per-dataset Go code toward **declarative dataset bindings**:

- `pkg/dank.Registrar` is the contract new datasets should implement (`GetMigrationUp/Down`, `GetResources`, `GetTools`, `Fetch`).
- `Binding`, `ResourceQuery`, `ToolQuery` structs in `pkg/dank/dank.go` carry JSON tags; intent is config-driven wiring rather than bespoke Go per dataset.
- `internal/mcp.ToolMap` is a `map[string]ToolRegistrationFunc` that lets multiple datasets plug into the server side-by-side.

## Layout

- `main.go` — CLI entry; opens DuckDB, migrates, reopens read-only, applies safe mode, registers tools, runs the MCP router.
- `data/` — cache paths and `.dank` root directory management (`EnsureDankPath`, `GetDankDir`, etc.).
- `internal/db/` — migrations (`duckdb_up.sql`, `duckdb_safe.sql`), `RunMigration`, `RunSafeMode`, and the `RowsToCSV` helper.
- `internal/mcp/` — `server.go` (router + `ToolMap` pattern) and `tool.go` (the generic `query` tool).
- `pkg/dank/` — the binding contracts (currently types only; no implementations yet).

## Do / Don't

**Do:**
- Add new datasets as implementations of `dank.Registrar`, not as hand-rolled per-dataset Go in `data/us/…` style.
- Register new MCP tools by writing a function matching `mcp.ToolRegistrationFunc` (`func(*mcp_server.MCPServer, *sql.DB) error`) and adding it to the `ToolMap` in `main.go`.
- Use `data.EnsureDankPath(...)` for any path under the dank root; don't reimplement path logic.
- Pass connections explicitly — the `ToolRegistrationFunc` already takes a `*sql.DB`. Prefer closures over package globals.

**Don't:**
- Don't reintroduce the old CT-style per-dataset Go (`data/us/ct/*.go`). The `dank-mcp-bak/` dir is a historical snapshot, not a target to restore.
- Don't open the MCP-facing DuckDB connection read-write. The `query` tool only exists safely because the connection is hardened first.
- Don't bypass `db.RunSafeMode` or `access_mode=read_only` on the connection used by MCP tools.
- Don't add fetch/prime logic directly into `main.go`. If a dataset needs to populate the DB, put that behind `Registrar.Fetch` and call it from a well-scoped path.

## Build and run

```sh
task            # builds bin/dank-mcp
go build ./...  # equivalent direct invocation
go vet ./...
go run . --help
```

No test suite exists yet. If you add one, wire it into `Taskfile.yml`.

## Known drift to clean up

- `Taskfile.yml: stdio-schema` invokes `--no-fetch`, a flag that no longer exists. The task will fail as-is.
- `dank-mcp-bak/` is a pre-refactor working-tree snapshot; not currently gitignored.
- `defaultLogDest` in `main.go` is declared but unused.
