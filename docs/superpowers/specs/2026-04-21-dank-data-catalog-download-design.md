# dank-data catalog download — design

**Date:** 2026-04-21
**Status:** Approved (pending user sign-off on this doc)
**Owner:** @neomantra

## 1. Goal

Let `dank-mcp` bootstrap itself by downloading a prebuilt DuckDB snapshot from the AgentDank `dank-data` GitHub repository, so users don't have to hand-wrangle data files before running the server. Today only `us/ct` exists; the design must scale to many datasets without further dank-mcp releases.

## 2. Non-goals (v1)

- Auto-fetch on first run with no flags. Network I/O is opt-in via `--fetch`.
- Multi-dataset fetch in a single invocation (`--fetch us/ct,us/ma`).
- Incremental / delta updates. TTL + full re-download only.
- Baked-in fallback catalog. If `catalog.json` is unreachable and no cached copy of the dataset exists, the command hard-fails with a helpful message.
- Per-table CSV/JSON ingestion. A future feature, driven by `pkg/dank.Registrar`.
- Proxy / auth headers beyond what the stdlib `http.Client` picks up from `HTTP_PROXY`.

## 3. Catalog format

A single JSON file served at:

```
https://raw.githubusercontent.com/AgentDank/dank-data/main/snapshots/catalog.json
```

### Schema (`version: 1`)

```json
{
  "version": 1,
  "datasets": {
    "us/ct": {
      "title": "United States — Connecticut",
      "description": "Cannabis brand registry, applications, credentials, lottery, retail locations, tax, weekly sales, and zoning.",
      "duckdb_url": "https://raw.githubusercontent.com/AgentDank/dank-data/main/snapshots/us/ct/dank-data.duckdb.zst",
      "sha256": "<64-hex-chars>",
      "updated_at": "2026-04-19T00:00:00Z"
    }
  }
}
```

### Field semantics

| Field | Required | Purpose |
|---|---|---|
| `version` | yes | Catalog-format version. Client refuses unknown major versions. |
| `datasets` | yes | Map keyed by dataset id (path convention: `us/ct`, `us/ma`, `ca/on`, …). |
| `title` | yes | Human-readable name; shown in `--list`. |
| `description` | yes | One-liner; shown in `--list`. |
| `duckdb_url` | yes | Absolute URL to the zstd-compressed DuckDB snapshot. |
| `sha256` | yes | Hex SHA-256 of the compressed `.duckdb.zst` bytes. Verified after download. |
| `updated_at` | optional | ISO-8601; informational only. TTL uses local file mtime, not this field. |

Extra unknown fields are ignored (forward-compatible JSON decoding).

### dank-data publishing responsibility

dank-data's CI is responsible for:
1. Producing `snapshots/<id>/dank-data.duckdb.zst`.
2. Computing its sha256 and injecting it into `snapshots/catalog.json`.
3. Committing both in the same commit so catalog and files never drift.

## 4. CLI surface

New flags on the existing `dank-mcp` command (no subcommands):

```
dank-mcp --list                       # print catalog, exit 0
dank-mcp --fetch <id>                 # download-if-stale, then serve from cached DB
dank-mcp --fetch <id> --fetch-only    # download-if-stale, exit 0
dank-mcp --fetch <id> --force         # force re-download ignoring TTL
dank-mcp --db PATH                    # unchanged; explicit path wins over --fetch's cache
```

### Rules

1. **`--db` always wins.** If set, server opens `--db` regardless of whether `--fetch` ran. `--fetch` still downloads into the cache — useful if you're warming the cache for a later run.
2. **Cache auto-use.** If `--db` is not set and `--fetch <id>` succeeded (or skipped-fresh), the server opens the cached path for that id.
3. **`--fetch-only` and `--list` exit** after their work; they do not start the MCP server.
4. **`--force` requires `--fetch`.** Using it alone is a usage error (exit code 2).

### `--list` output

One dataset per line, tab-separated, with a header:

```
ID        TITLE                          UPDATED       DESCRIPTION
us/ct     United States — Connecticut    2026-04-19    Cannabis brand registry, applications…
```

## 5. Architecture

Packages (all internal):

- **`internal/catalog`** — `Fetch(ctx) (Catalog, error)` and `Lookup(catalog, id) (Entry, error)`. HTTP-agnostic beyond an injectable `http.Client` for testability.
- **`internal/fetch`** — orchestrates the pipeline for one dataset: TTL check → catalog lookup → HTTP GET → sha256 verify → zstd decompress → atomic rename. Exposes `Download(ctx, id, opts) (cachePath string, err error)`.
- **`data/cache.go`** (existing) — extended with `GetDatasetCachePath(id) string` helper that returns `.dank/cache/<id>/dank-data.duckdb`. No new package.
- **`main.go`** — adds the flag plumbing and, if `--fetch` succeeded and `--db` is unset, sets `config.DuckDBFile` to the cached path before opening DuckDB.

### Dependencies added

- `github.com/klauspost/compress` — pure-Go zstd.
- `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles` — TTY progress bar.
- `golang.org/x/term` — TTY detection.

No third-party HTTP client; stdlib `net/http` with a 5-minute timeout and `HTTP_PROXY` honored implicitly.

## 6. Fetch data flow

For a single `--fetch <id>` invocation:

1. Resolve cache path → `.dank/cache/<id>/dank-data.duckdb`; ensure parent dirs via `data.EnsureDankPath(...)`.
2. **TTL check**: if `cached` exists and `time.Since(stat.ModTime()) < 7*24h` and `--force` is absent → skip to step 8.
3. `GET https://raw.githubusercontent.com/AgentDank/dank-data/main/snapshots/catalog.json`. Parse; assert `version == 1`; look up `id`. Missing id → error listing known ids.
4. `GET datasets[id].duckdb_url`. Stream bytes through a `sha256.Writer` into `.dank/cache/<id>/dank-data.duckdb.zst.partial`.
5. After EOF, compare computed sha256 to catalog's `sha256`. Mismatch → delete `.partial`, hard error.
6. Stream-decompress `.partial` (klauspost zstd reader) into `.dank/cache/<id>/dank-data.duckdb.new`.
7. `os.Rename(".new", "dank-data.duckdb")` — atomic commit. Delete `.partial`.
8. Return the final cached path.

The live `dank-data.duckdb` is never in a torn state: the rename is the commit boundary. Crashes leave `.partial` / `.new` debris that the next run overwrites.

### Offline / failure matrix

"Proceed" below means: if `--fetch-only` was given, exit 0 after logging; otherwise open the cached DB and start the MCP server.

| Condition | Cache exists | Behavior | Exit |
|---|---|---|---|
| Catalog fetch fails | yes | `slog.Warn`, proceed with stale cache | 0 |
| Catalog fetch fails | no | hard error | 1 |
| Dataset download fails | yes | `slog.Warn`, proceed with stale cache | 0 |
| Dataset download fails | no | hard error | 1 |
| sha256 mismatch | — | hard error (never trust bad bytes) | 1 |
| Zstd decode fails | — | hard error | 1 |
| Unknown catalog version | — | hard error ("please upgrade dank-mcp") | 1 |
| Unknown id | — | hard error with list of known ids | 2 |
| `--force` without `--fetch` | — | usage error | 2 |

## 7. Progress UX

Download progress uses `bubbles/progress` driven by a `bubbletea` program.

- **Output: stderr only.** `tea.NewProgram(model, tea.WithOutput(os.Stderr))`. Stdout is reserved for MCP JSON-RPC; we must never contaminate it.
- **TTY gate.** `golang.org/x/term.IsTerminal(int(os.Stderr.Fd()))`. If false (piped, redirected, running under Claude Desktop), skip the TUI entirely.
- **Non-TTY runs.** Log `slog.Info("downloading", "id", id, "url", url)` before the GET and `slog.Info("downloaded", "bytes", n)` after. Visible in the log file the user configured.
- **Implementation sketch.** A `countingReader` wraps the HTTP response body; in TTY mode it sends `tea.Cmd` progress messages on each N-byte read. `Content-Length` drives the total; if missing, the model falls back to an indeterminate animation.

## 8. Testing

No test suite exists today. This feature seeds one.

- **`internal/catalog` unit tests**
  - Parse fixture `testdata/catalog.json` (one entry, multiple entries).
  - Reject missing required fields (`duckdb_url`, `sha256`).
  - Reject unknown `version` values.
  - Ignore unknown fields.

- **`internal/fetch` unit tests** using `httptest.Server`
  - Happy path: fixture tiny `.duckdb.zst` with known sha256 → verify cached file exists and matches expected bytes.
  - sha256 mismatch: return wrong bytes → expect error, no final cached file.
  - 404 on dataset URL: expect error.
  - TTL skip: pre-populate cache with recent mtime → verify no HTTP call issued.
  - `--force` with fresh cache: verify HTTP call is issued regardless of mtime.
  - Catalog fetch failure with pre-existing stale cache: expect warning + reuse.

- **No integration test against real dank-data.** Offline/deterministic suite. A manual smoke step in the PR description confirms real-world fetch.

Wire into `Taskfile.yml`:

```yaml
  test:
    desc: 'Run Go tests'
    cmds:
      - go test ./...
```

## 9. File changes

New:

- `internal/catalog/catalog.go`
- `internal/catalog/catalog_test.go`
- `internal/catalog/testdata/catalog.json`
- `internal/fetch/fetch.go`
- `internal/fetch/fetch_test.go`
- `internal/fetch/progress.go`
- `internal/fetch/testdata/tiny.duckdb.zst` (for httptest fixtures)

Modified:

- `main.go` — new flags (`--fetch`, `--fetch-only`, `--force`, `--list`), wiring.
- `data/cache.go` — add `GetDatasetCachePath(id string) string`.
- `go.mod` / `go.sum` — new deps (klauspost/compress, bubbletea, bubbles, x/term).
- `Taskfile.yml` — add `test` task.
- `README.md` — document `--fetch` and `--list`; add a "Loading CT data" section.
- `AGENTS.md` — note catalog URL and fetch flow for future contributors.

External (in `AgentDank/dank-data`):

- `snapshots/catalog.json` — new file per schema above.
- CI: compute sha256 at publish time and keep catalog in sync with snapshot files.

## 10. Open questions

None at design time. Implementation may surface judgment calls around the exact Bubble Tea message cadence, sha256 chunk size, and context-cancellation propagation — to be resolved in review.
