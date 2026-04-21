# dank-data catalog download — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--fetch <id>` / `--list` / `--force` / `--fetch-only` CLI flags to `dank-mcp` that download a zstd-compressed DuckDB snapshot from the `AgentDank/dank-data` repo via a remote catalog.json, verify its sha256, decompress it, and atomically install it at `.dank/cache/<id>/dank-data.duckdb`.

**Architecture:** Two new internal packages (`internal/catalog`, `internal/fetch`), a path helper on the existing `data` package, and new flag wiring in `main.go`. The download pipeline streams: HTTP body → sha256 hash-and-copy → `.partial` file → zstd decompress → `.new` file → atomic rename to final path. Progress rendered via `bubbles/progress` on stderr, only when stderr is a TTY; otherwise `slog.Info` fallback.

**Tech Stack:** Go 1.24, stdlib `net/http`, `github.com/klauspost/compress/zstd`, `charm.land/bubbletea/v2`, `charm.land/bubbles/v2/progress`, `golang.org/x/term`.

**Design spec:** `docs/superpowers/specs/2026-04-21-dank-data-catalog-download-design.md`

---

## Task 1: Add dependencies and `test` task

**Files:**
- Modify: `go.mod`, `go.sum`
- Modify: `Taskfile.yml`

- [ ] **Step 1: Add the four new dependencies**

```bash
go get github.com/klauspost/compress/zstd
go get charm.land/bubbletea/v2
go get charm.land/bubbles/v2/progress
go get golang.org/x/term
go mod tidy
```

- [ ] **Step 2: Add a `test` task to Taskfile.yml**

Append after the `build` task block:

```yaml
  test:
    desc: 'Run Go tests'
    cmds:
      - go test ./...
    sources:
      - "*.go"
      - "data/**/*.go"
      - "internal/**/*.go"
      - "pkg/**/*.go"
```

- [ ] **Step 3: Verify everything still builds**

```bash
go build ./...
task test
```

Expected: build succeeds; `task test` runs (may say "no test files" for now — that's fine).

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum Taskfile.yml
git commit -m "Add deps (zstd, bubbletea, bubbles, x/term) and test task"
```

---

## Task 2: Add `GetDatasetCachePath` to `data` package

**Files:**
- Modify: `data/cache.go`
- Create: `data/cache_test.go`

- [ ] **Step 1: Write the failing test**

Create `data/cache_test.go`:

```go
// Copyright (c) 2026 Neomantra Corp

package data

import (
	"path/filepath"
	"testing"
)

func TestGetDatasetCachePath(t *testing.T) {
	t.Cleanup(func() { SetDankRoot(".") })
	SetDankRoot("/tmp/dank-test")

	got := GetDatasetCachePath("us/ct")
	want := filepath.Join("/tmp/dank-test", ".dank", "cache", "us", "ct", "dank-data.duckdb")
	if got != want {
		t.Errorf("GetDatasetCachePath(us/ct) = %q; want %q", got, want)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

```bash
go test ./data/
```

Expected: FAIL — `undefined: GetDatasetCachePath`.

- [ ] **Step 3: Implement**

Append to `data/cache.go`:

```go
// GetDatasetCachePath returns the canonical on-disk path for a dataset's
// downloaded DuckDB under the dank root: .dank/cache/<id>/dank-data.duckdb
func GetDatasetCachePath(id string) string {
	return filepath.Join(GetDankCacheDir(), filepath.FromSlash(id), "dank-data.duckdb")
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./data/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add data/cache.go data/cache_test.go
git commit -m "Add data.GetDatasetCachePath helper"
```

---

## Task 3: Catalog types and `Parse()`

**Files:**
- Create: `internal/catalog/catalog.go`
- Create: `internal/catalog/catalog_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/catalog/catalog_test.go`:

```go
// Copyright (c) 2026 Neomantra Corp

package catalog

import (
	"strings"
	"testing"
)

const validCatalog = `{
  "version": 1,
  "datasets": {
    "us/ct": {
      "title": "United States — Connecticut",
      "description": "CT cannabis data.",
      "duckdb_url": "https://example.com/us/ct/dank-data.duckdb.zst",
      "sha256": "0000000000000000000000000000000000000000000000000000000000000000",
      "updated_at": "2026-04-19T00:00:00Z"
    }
  }
}`

func TestParse_Valid(t *testing.T) {
	cat, err := Parse([]byte(validCatalog))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cat.Version != 1 {
		t.Errorf("Version = %d; want 1", cat.Version)
	}
	entry, ok := cat.Datasets["us/ct"]
	if !ok {
		t.Fatalf("us/ct missing")
	}
	if entry.DuckDBURL != "https://example.com/us/ct/dank-data.duckdb.zst" {
		t.Errorf("DuckDBURL = %q", entry.DuckDBURL)
	}
	if entry.SHA256 != "0000000000000000000000000000000000000000000000000000000000000000" {
		t.Errorf("SHA256 = %q", entry.SHA256)
	}
}

func TestParse_RejectsUnknownVersion(t *testing.T) {
	body := strings.Replace(validCatalog, `"version": 1`, `"version": 99`, 1)
	_, err := Parse([]byte(body))
	if err == nil {
		t.Fatal("expected error for unknown version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("error should mention version: %v", err)
	}
}

func TestParse_RejectsMissingDuckDBURL(t *testing.T) {
	body := strings.Replace(validCatalog,
		`"duckdb_url": "https://example.com/us/ct/dank-data.duckdb.zst",`, "", 1)
	_, err := Parse([]byte(body))
	if err == nil {
		t.Fatal("expected error for missing duckdb_url")
	}
}

func TestParse_RejectsMissingSHA256(t *testing.T) {
	body := strings.Replace(validCatalog,
		`"sha256": "0000000000000000000000000000000000000000000000000000000000000000",`, "", 1)
	_, err := Parse([]byte(body))
	if err == nil {
		t.Fatal("expected error for missing sha256")
	}
}

func TestParse_IgnoresUnknownFields(t *testing.T) {
	body := strings.Replace(validCatalog,
		`"updated_at": "2026-04-19T00:00:00Z"`,
		`"updated_at": "2026-04-19T00:00:00Z", "future_field": 42`, 1)
	_, err := Parse([]byte(body))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/catalog/
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement**

Create `internal/catalog/catalog.go`:

```go
// Copyright (c) 2026 Neomantra Corp

// Package catalog fetches and parses the dank-data catalog.json.
package catalog

import (
	"encoding/json"
	"fmt"
)

const currentVersion = 1

// Catalog is the parsed catalog.json document.
type Catalog struct {
	Version  int                     `json:"version"`
	Datasets map[string]DatasetEntry `json:"datasets"`
}

// DatasetEntry describes one downloadable dataset.
type DatasetEntry struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	DuckDBURL   string `json:"duckdb_url"`
	SHA256      string `json:"sha256"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// Parse decodes a catalog.json body and validates the required fields.
func Parse(body []byte) (Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(body, &c); err != nil {
		return Catalog{}, fmt.Errorf("decode catalog: %w", err)
	}
	if c.Version != currentVersion {
		return Catalog{}, fmt.Errorf("unsupported catalog version %d (expected %d); please upgrade dank-mcp", c.Version, currentVersion)
	}
	for id, entry := range c.Datasets {
		if entry.DuckDBURL == "" {
			return Catalog{}, fmt.Errorf("dataset %q missing required field duckdb_url", id)
		}
		if entry.SHA256 == "" {
			return Catalog{}, fmt.Errorf("dataset %q missing required field sha256", id)
		}
	}
	return c, nil
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/catalog/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/
git commit -m "Add catalog.Parse with schema validation"
```

---

## Task 4: Catalog `Lookup()`

**Files:**
- Modify: `internal/catalog/catalog.go`
- Modify: `internal/catalog/catalog_test.go`

- [ ] **Step 1: Append failing tests**

Append to `internal/catalog/catalog_test.go`:

```go
func TestLookup_Hit(t *testing.T) {
	cat, _ := Parse([]byte(validCatalog))
	entry, err := cat.Lookup("us/ct")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if entry.Title != "United States — Connecticut" {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestLookup_MissIncludesKnownIDs(t *testing.T) {
	cat, _ := Parse([]byte(validCatalog))
	_, err := cat.Lookup("us/zz")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "us/ct") {
		t.Errorf("error should list known ids: %v", err)
	}
}
```

- [ ] **Step 2: Run tests, confirm fail**

```bash
go test ./internal/catalog/
```

Expected: FAIL — `cat.Lookup` undefined.

- [ ] **Step 3: Implement**

Append to `internal/catalog/catalog.go`:

```go
import "sort"

// (adjust existing imports to include "sort")

// Lookup returns the entry for the given dataset id, or an error listing
// the known ids if the id is not in the catalog.
func (c Catalog) Lookup(id string) (DatasetEntry, error) {
	if entry, ok := c.Datasets[id]; ok {
		return entry, nil
	}
	known := make([]string, 0, len(c.Datasets))
	for k := range c.Datasets {
		known = append(known, k)
	}
	sort.Strings(known)
	return DatasetEntry{}, fmt.Errorf("unknown dataset %q; known ids: %v", id, known)
}
```

(Edit the existing `import` block to include `"sort"`.)

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/catalog/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/
git commit -m "Add catalog.Lookup with helpful miss message"
```

---

## Task 5: Catalog `Fetch()` over HTTP

**Files:**
- Modify: `internal/catalog/catalog.go`
- Modify: `internal/catalog/catalog_test.go`

- [ ] **Step 1: Append failing test**

Append to `internal/catalog/catalog_test.go`:

```go
import (
	"context"
	"net/http"
	"net/http/httptest"
)

// NOTE: merge this import block with the existing one manually.

func TestFetch_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/catalog.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(validCatalog))
	}))
	defer srv.Close()

	cat, err := Fetch(context.Background(), srv.URL+"/catalog.json", srv.Client())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if cat.Version != 1 {
		t.Errorf("Version = %d", cat.Version)
	}
}

func TestFetch_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), srv.URL+"/catalog.json", srv.Client())
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run test, confirm fail**

```bash
go test ./internal/catalog/
```

Expected: FAIL — `Fetch` undefined.

- [ ] **Step 3: Implement**

Append to `internal/catalog/catalog.go`:

```go
import (
	"context"
	"io"
	"net/http"
)

// (merge with existing imports)

// DefaultURL is the well-known location of the dank-data catalog.
const DefaultURL = "https://raw.githubusercontent.com/AgentDank/dank-data/main/snapshots/catalog.json"

// Fetch retrieves and parses the catalog at url. Pass nil for client to use
// http.DefaultClient.
func Fetch(ctx context.Context, url string, client *http.Client) (Catalog, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Catalog{}, fmt.Errorf("build catalog request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Catalog{}, fmt.Errorf("fetch catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Catalog{}, fmt.Errorf("fetch catalog: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Catalog{}, fmt.Errorf("read catalog body: %w", err)
	}
	return Parse(body)
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/catalog/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/
git commit -m "Add catalog.Fetch for HTTP retrieval"
```

---

## Task 6: zstd decompress helper

**Files:**
- Create: `internal/fetch/decompress.go`
- Create: `internal/fetch/decompress_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/fetch/decompress_test.go`:

```go
// Copyright (c) 2026 Neomantra Corp

package fetch

import (
	"bytes"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestDecompressZstd(t *testing.T) {
	payload := []byte("hello dank-data, this is a test payload")

	var compressed bytes.Buffer
	w, err := zstd.NewWriter(&compressed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	w.Close()

	var out bytes.Buffer
	if err := decompressZstd(&compressed, &out); err != nil {
		t.Fatalf("decompressZstd: %v", err)
	}
	if !bytes.Equal(out.Bytes(), payload) {
		t.Errorf("mismatch: got %q want %q", out.String(), payload)
	}
}

func TestDecompressZstd_BadInput(t *testing.T) {
	var out bytes.Buffer
	err := decompressZstd(bytes.NewReader([]byte("not zstd")), &out)
	if err == nil {
		t.Fatal("expected error for garbage input")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/fetch/
```

Expected: FAIL — package / function undefined.

- [ ] **Step 3: Implement**

Create `internal/fetch/decompress.go`:

```go
// Copyright (c) 2026 Neomantra Corp

package fetch

import (
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// decompressZstd streams a zstd-compressed reader into dst.
func decompressZstd(src io.Reader, dst io.Writer) error {
	dec, err := zstd.NewReader(src)
	if err != nil {
		return fmt.Errorf("zstd reader: %w", err)
	}
	defer dec.Close()
	if _, err := io.Copy(dst, dec); err != nil {
		return fmt.Errorf("zstd decode: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/fetch/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/fetch/
git commit -m "Add zstd streaming decompress helper"
```

---

## Task 7: sha256 verify-while-copy helper

**Files:**
- Create: `internal/fetch/sha256.go`
- Create: `internal/fetch/sha256_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/fetch/sha256_test.go`:

```go
// Copyright (c) 2026 Neomantra Corp

package fetch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestCopyAndVerify_Match(t *testing.T) {
	payload := []byte("exactly these bytes")
	sum := sha256.Sum256(payload)
	wantHex := hex.EncodeToString(sum[:])

	var dst bytes.Buffer
	n, err := copyAndVerify(&dst, bytes.NewReader(payload), wantHex)
	if err != nil {
		t.Fatalf("copyAndVerify: %v", err)
	}
	if n != int64(len(payload)) {
		t.Errorf("n = %d; want %d", n, len(payload))
	}
	if !bytes.Equal(dst.Bytes(), payload) {
		t.Errorf("dst bytes mismatch")
	}
}

func TestCopyAndVerify_Mismatch(t *testing.T) {
	payload := []byte("some bytes")
	wrongHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	var dst bytes.Buffer
	_, err := copyAndVerify(&dst, bytes.NewReader(payload), wrongHex)
	if err == nil {
		t.Fatal("expected sha256 mismatch error")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/fetch/
```

Expected: FAIL — `copyAndVerify` undefined.

- [ ] **Step 3: Implement**

Create `internal/fetch/sha256.go`:

```go
// Copyright (c) 2026 Neomantra Corp

package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// copyAndVerify streams src into dst while computing a sha256. After EOF,
// the computed digest is compared against wantHex. Returns the number of
// bytes copied, or an error if the digest does not match.
func copyAndVerify(dst io.Writer, src io.Reader, wantHex string) (int64, error) {
	h := sha256.New()
	tee := io.TeeReader(src, h)
	n, err := io.Copy(dst, tee)
	if err != nil {
		return n, fmt.Errorf("copy: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != wantHex {
		return n, fmt.Errorf("sha256 mismatch: expected %s, got %s", wantHex, got)
	}
	return n, nil
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/fetch/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/fetch/
git commit -m "Add copyAndVerify for streaming sha256 validation"
```

---

## Task 8: `Download` orchestrator — happy path

**Files:**
- Create: `internal/fetch/fetch.go`
- Create: `internal/fetch/fetch_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/fetch/fetch_test.go`:

```go
// Copyright (c) 2026 Neomantra Corp

package fetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// buildSnapshot returns (compressedBytes, sha256Hex, originalPayload).
func buildSnapshot(t *testing.T) ([]byte, string, []byte) {
	t.Helper()
	payload := []byte("fake duckdb bytes for test")
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	w.Close()
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:]), payload
}

func startServer(t *testing.T, compressed []byte, sha256Hex string) *httptest.Server {
	t.Helper()
	catalog := fmt.Sprintf(`{
  "version": 1,
  "datasets": {
    "us/ct": {
      "title": "Test",
      "description": "Test",
      "duckdb_url": "%%s/snapshot.zst",
      "sha256": "%s",
      "updated_at": "2026-04-19T00:00:00Z"
    }
  }
}`, sha256Hex)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/catalog.json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, catalog, "http://"+r.Host)
		case "/snapshot.zst":
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(compressed)))
			w.Write(compressed)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func TestDownload_HappyPath(t *testing.T) {
	compressed, shaHex, payload := buildSnapshot(t)
	srv := startServer(t, compressed, shaHex)
	defer srv.Close()

	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "dank-data.duckdb")

	opts := Options{
		CatalogURL: srv.URL + "/catalog.json",
		CachePath:  cachePath,
		Client:     srv.Client(),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	gotPath, err := Download(context.Background(), "us/ct", opts)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if gotPath != cachePath {
		t.Errorf("path = %q; want %q", gotPath, cachePath)
	}
	got, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("cached bytes mismatch: got %q want %q", got, payload)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/fetch/
```

Expected: FAIL — `Download` / `Options` undefined.

- [ ] **Step 3: Implement**

Create `internal/fetch/fetch.go`:

```go
// Copyright (c) 2026 Neomantra Corp

// Package fetch downloads dank-data snapshot DuckDBs and installs them
// into the dank cache with integrity verification and atomic replacement.
package fetch

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/AgentDank/dank-mcp/internal/catalog"
)

// Options configures a Download call.
type Options struct {
	// CatalogURL overrides the default catalog location. If empty, the
	// default (catalog.DefaultURL) is used.
	CatalogURL string

	// CachePath is the final on-disk location for the installed DuckDB.
	// Must be an absolute or otherwise already-resolved path.
	CachePath string

	// Client is the HTTP client used for all requests. If nil, http.DefaultClient.
	Client *http.Client

	// Logger receives progress and warning messages. Must not be nil.
	Logger *slog.Logger

	// Force bypasses the TTL check and always re-downloads. Added in a
	// later task; default false.
	Force bool
}

// Download fetches the catalog, resolves id, downloads and verifies the
// snapshot, decompresses it, and atomically installs it at CachePath.
// Returns CachePath on success.
func Download(ctx context.Context, id string, opts Options) (string, error) {
	if opts.Logger == nil {
		return "", fmt.Errorf("fetch.Download: Logger is required")
	}
	if opts.CachePath == "" {
		return "", fmt.Errorf("fetch.Download: CachePath is required")
	}
	catURL := opts.CatalogURL
	if catURL == "" {
		catURL = catalog.DefaultURL
	}

	opts.Logger.Info("fetching catalog", "url", catURL)
	cat, err := catalog.Fetch(ctx, catURL, opts.Client)
	if err != nil {
		return "", fmt.Errorf("catalog: %w", err)
	}
	entry, err := cat.Lookup(id)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(opts.CachePath), 0o755); err != nil {
		return "", fmt.Errorf("mkdir cache dir: %w", err)
	}
	partialPath := opts.CachePath + ".zst.partial"
	newPath := opts.CachePath + ".new"

	// Always clean up partial / new on exit (unless rename succeeded).
	var renamed bool
	defer func() {
		os.Remove(partialPath)
		if !renamed {
			os.Remove(newPath)
		}
	}()

	opts.Logger.Info("downloading", "id", id, "url", entry.DuckDBURL)
	if err := downloadVerified(ctx, opts.Client, entry.DuckDBURL, partialPath, entry.SHA256); err != nil {
		return "", err
	}

	if err := decompressFile(partialPath, newPath); err != nil {
		return "", err
	}

	if err := os.Rename(newPath, opts.CachePath); err != nil {
		return "", fmt.Errorf("install cache file: %w", err)
	}
	renamed = true

	info, _ := os.Stat(opts.CachePath)
	if info != nil {
		opts.Logger.Info("downloaded", "id", id, "bytes", info.Size())
	}
	return opts.CachePath, nil
}

func downloadVerified(ctx context.Context, client *http.Client, url, partialPath, sha256Hex string) error {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}

	f, err := os.Create(partialPath)
	if err != nil {
		return fmt.Errorf("create partial: %w", err)
	}
	defer f.Close()

	if _, err := copyAndVerify(f, resp.Body, sha256Hex); err != nil {
		return err
	}
	return nil
}

func decompressFile(srcPath, dstPath string) error {
	in, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open compressed: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create decompressed: %w", err)
	}
	defer out.Close()

	if err := decompressZstd(in, out); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/fetch/
```

Expected: PASS (all fetch tests, including the new happy-path one).

- [ ] **Step 5: Commit**

```bash
git add internal/fetch/
git commit -m "Add fetch.Download happy-path orchestration"
```

---

## Task 9: `Download` error cases

**Files:**
- Modify: `internal/fetch/fetch_test.go`

- [ ] **Step 1: Append error-case tests**

Append to `internal/fetch/fetch_test.go`:

```go
func TestDownload_SHA256Mismatch(t *testing.T) {
	compressed, _, _ := buildSnapshot(t)
	badHex := "0000000000000000000000000000000000000000000000000000000000000000"
	srv := startServer(t, compressed, badHex)
	defer srv.Close()

	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "dank-data.duckdb")

	_, err := Download(context.Background(), "us/ct", Options{
		CatalogURL: srv.URL + "/catalog.json",
		CachePath:  cachePath,
		Client:     srv.Client(),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err == nil {
		t.Fatal("expected sha256 mismatch error")
	}
	if _, statErr := os.Stat(cachePath); statErr == nil {
		t.Error("cached file should not exist after mismatch")
	}
}

func TestDownload_Unknown(t *testing.T) {
	compressed, shaHex, _ := buildSnapshot(t)
	srv := startServer(t, compressed, shaHex)
	defer srv.Close()

	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "dank-data.duckdb")

	_, err := Download(context.Background(), "us/zz", Options{
		CatalogURL: srv.URL + "/catalog.json",
		CachePath:  cachePath,
		Client:     srv.Client(),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err == nil {
		t.Fatal("expected unknown-id error")
	}
}

func TestDownload_Catalog404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	_, err := Download(context.Background(), "us/ct", Options{
		CatalogURL: srv.URL + "/catalog.json",
		CachePath:  filepath.Join(tmp, "dank-data.duckdb"),
		Client:     srv.Client(),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run, confirm all pass (existing impl already handles these)**

```bash
go test ./internal/fetch/
```

Expected: PASS — errors already flow through the Download pipeline from Task 8.

- [ ] **Step 3: Commit**

```bash
git add internal/fetch/
git commit -m "Cover Download error cases (sha256, unknown id, catalog 404)"
```

---

## Task 10: TTL skip and `--force` override

**Files:**
- Modify: `internal/fetch/fetch.go`
- Modify: `internal/fetch/fetch_test.go`

- [ ] **Step 1: Append failing tests**

Append to `internal/fetch/fetch_test.go`:

```go
import "time" // merge with existing imports

func TestDownload_TTLSkip(t *testing.T) {
	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "dank-data.duckdb")
	// Pre-populate cache with recent mtime
	if err := os.WriteFile(cachePath, []byte("prior-good"), 0o644); err != nil {
		t.Fatal(err)
	}

	// httptest server that panics on any request (TTL skip should hit nothing)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s", r.URL.Path)
	}))
	defer srv.Close()

	path, err := Download(context.Background(), "us/ct", Options{
		CatalogURL: srv.URL + "/catalog.json",
		CachePath:  cachePath,
		Client:     srv.Client(),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if path != cachePath {
		t.Errorf("path = %q", path)
	}
	got, _ := os.ReadFile(cachePath)
	if string(got) != "prior-good" {
		t.Errorf("cache mutated: %q", got)
	}
}

func TestDownload_TTLSkipBypassedByForce(t *testing.T) {
	compressed, shaHex, payload := buildSnapshot(t)
	srv := startServer(t, compressed, shaHex)
	defer srv.Close()

	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "dank-data.duckdb")
	os.WriteFile(cachePath, []byte("prior-good"), 0o644)

	_, err := Download(context.Background(), "us/ct", Options{
		CatalogURL: srv.URL + "/catalog.json",
		CachePath:  cachePath,
		Client:     srv.Client(),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Force:      true,
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, _ := os.ReadFile(cachePath)
	if !bytes.Equal(got, payload) {
		t.Errorf("cache not refreshed; got %q", got)
	}
}

func TestDownload_StalePast7Days(t *testing.T) {
	compressed, shaHex, payload := buildSnapshot(t)
	srv := startServer(t, compressed, shaHex)
	defer srv.Close()

	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "dank-data.duckdb")
	os.WriteFile(cachePath, []byte("old"), 0o644)
	old := time.Now().Add(-8 * 24 * time.Hour)
	os.Chtimes(cachePath, old, old)

	_, err := Download(context.Background(), "us/ct", Options{
		CatalogURL: srv.URL + "/catalog.json",
		CachePath:  cachePath,
		Client:     srv.Client(),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, _ := os.ReadFile(cachePath)
	if !bytes.Equal(got, payload) {
		t.Errorf("cache not refreshed; got %q", got)
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/fetch/
```

Expected: FAIL — TTL skip not implemented; fresh cache still triggers download.

- [ ] **Step 3: Implement TTL check**

In `internal/fetch/fetch.go`, replace the beginning of `Download` (right after the preamble error checks and catURL resolution) with:

```go
	// TTL check before any network I/O
	if !opts.Force {
		if info, err := os.Stat(opts.CachePath); err == nil {
			if time.Since(info.ModTime()) < cacheTTL {
				opts.Logger.Info("cache fresh; skipping download", "id", id, "path", opts.CachePath)
				return opts.CachePath, nil
			}
		}
	}
```

Add at top of file:

```go
import "time" // merge with existing

const cacheTTL = 7 * 24 * time.Hour
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/fetch/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/fetch/
git commit -m "Add 7-day TTL skip with --force override"
```

---

## Task 11: Offline warn-and-fallback behavior

**Files:**
- Modify: `internal/fetch/fetch.go`
- Modify: `internal/fetch/fetch_test.go`

- [ ] **Step 1: Append failing test**

Append to `internal/fetch/fetch_test.go`:

```go
func TestDownload_CatalogFailsWithStaleCache(t *testing.T) {
	// Server always 500s on catalog
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "dank-data.duckdb")
	os.WriteFile(cachePath, []byte("stale-but-usable"), 0o644)
	old := time.Now().Add(-30 * 24 * time.Hour)
	os.Chtimes(cachePath, old, old)

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	path, err := Download(context.Background(), "us/ct", Options{
		CatalogURL: srv.URL + "/catalog.json",
		CachePath:  cachePath,
		Client:     srv.Client(),
		Logger:     logger,
	})
	if err != nil {
		t.Fatalf("expected fallback, got error: %v", err)
	}
	if path != cachePath {
		t.Errorf("path = %q", path)
	}
	// A warn-level record should have been emitted about the fallback.
	if !bytes.Contains(buf.Bytes(), []byte("level=WARN")) {
		t.Errorf("expected WARN log; got:\n%s", buf.String())
	}
}

func TestDownload_CatalogFailsNoCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	_, err := Download(context.Background(), "us/ct", Options{
		CatalogURL: srv.URL + "/catalog.json",
		CachePath:  filepath.Join(tmp, "dank-data.duckdb"),
		Client:     srv.Client(),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err == nil {
		t.Fatal("expected error with no cache")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/fetch/
```

Expected: FAIL — current Download returns the catalog error directly.

- [ ] **Step 3: Implement fallback**

In `internal/fetch/fetch.go`, rewrite the catalog-fetch portion of `Download`:

```go
	opts.Logger.Info("fetching catalog", "url", catURL)
	cat, err := catalog.Fetch(ctx, catURL, opts.Client)
	if err != nil {
		// If there's a usable cache, degrade gracefully.
		if info, statErr := os.Stat(opts.CachePath); statErr == nil {
			opts.Logger.Warn("catalog fetch failed; using stale cache",
				"err", err, "path", opts.CachePath, "age", time.Since(info.ModTime()).String())
			return opts.CachePath, nil
		}
		return "", fmt.Errorf("catalog: %w", err)
	}
```

Do the same treatment for the `downloadVerified` call:

```go
	opts.Logger.Info("downloading", "id", id, "url", entry.DuckDBURL)
	if err := downloadVerified(ctx, opts.Client, entry.DuckDBURL, partialPath, entry.SHA256); err != nil {
		if info, statErr := os.Stat(opts.CachePath); statErr == nil {
			opts.Logger.Warn("download failed; using stale cache",
				"err", err, "path", opts.CachePath, "age", time.Since(info.ModTime()).String())
			return opts.CachePath, nil
		}
		return "", err
	}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/fetch/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/fetch/
git commit -m "Fall back to stale cache on catalog/download failure"
```

---

## Task 12: Progress UI (TTY-gated, stderr-only)

**Files:**
- Create: `internal/fetch/progress.go`
- Modify: `internal/fetch/fetch.go`

Note: Bubble Tea is awkward to unit-test cleanly. We verify the progress code compiles and plugs in correctly; behavioral verification is via manual smoke (step 4).

- [ ] **Step 1: Implement the progress model and wrapper**

Create `internal/fetch/progress.go`:

```go
// Copyright (c) 2026 Neomantra Corp

package fetch

import (
	"io"
	"os"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

// NOTE: bubbletea v2 and bubbles v2 have API shifts from v1. Before writing
// the model, run `go doc charm.land/bubbletea/v2` and
// `go doc charm.land/bubbles/v2/progress` to confirm: NewProgram options,
// Program.Send/Wait/Quit, progress.Model construction, and FrameMsg handling.
// If any method signature below doesn't compile against the v2 API, adjust
// the code to match v2 and keep the overall structure (TTY gate, stderr
// output, counting reader pattern) intact.

// progressReporter reports download progress to stderr when stderr is a TTY,
// and is a no-op otherwise.
type progressReporter struct {
	prog  *tea.Program
	model *progressModel
	total int64
}

func newProgressReporter(totalBytes int64) *progressReporter {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return &progressReporter{}
	}
	m := &progressModel{bar: progress.New(progress.WithDefaultGradient()), total: totalBytes}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	go func() { _, _ = p.Run() }()
	return &progressReporter{prog: p, model: m, total: totalBytes}
}

func (r *progressReporter) wrap(body io.Reader) io.Reader {
	if r.prog == nil {
		return body
	}
	return &countingReader{src: body, reporter: r}
}

func (r *progressReporter) update(n int64) {
	if r.prog == nil {
		return
	}
	r.prog.Send(progressMsg{bytesRead: n})
}

func (r *progressReporter) finish() {
	if r.prog == nil {
		return
	}
	r.prog.Send(tea.Quit())
	r.prog.Wait()
}

type countingReader struct {
	src      io.Reader
	reporter *progressReporter
	read     int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.src.Read(p)
	c.read += int64(n)
	c.reporter.update(c.read)
	return n, err
}

type progressMsg struct{ bytesRead int64 }

type progressModel struct {
	bar   progress.Model
	total int64
	read  int64
	done  bool
}

func (m *progressModel) Init() tea.Cmd { return nil }

func (m *progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressMsg:
		m.read = msg.bytesRead
		var pct float64
		if m.total > 0 {
			pct = float64(m.read) / float64(m.total)
			if pct > 1 {
				pct = 1
			}
		}
		return m, m.bar.SetPercent(pct)
	case progress.FrameMsg:
		var cmd tea.Cmd
		var updated tea.Model
		updated, cmd = m.bar.Update(msg)
		m.bar = updated.(progress.Model)
		return m, cmd
	case tea.QuitMsg:
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *progressModel) View() string {
	return m.bar.View() + "\n"
}
```

- [ ] **Step 2: Plug the reporter into `downloadVerified`**

In `internal/fetch/fetch.go`, replace `downloadVerified` with:

```go
func downloadVerified(ctx context.Context, client *http.Client, url, partialPath, sha256Hex string) error {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}

	f, err := os.Create(partialPath)
	if err != nil {
		return fmt.Errorf("create partial: %w", err)
	}
	defer f.Close()

	reporter := newProgressReporter(resp.ContentLength)
	defer reporter.finish()

	if _, err := copyAndVerify(f, reporter.wrap(resp.Body), sha256Hex); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 3: Run all tests**

```bash
go test ./...
```

Expected: PASS — existing tests run with stderr piped (non-TTY), so the TUI is bypassed and behavior is identical.

- [ ] **Step 4: Manual smoke test (optional but recommended)**

```bash
go build -o bin/dank-mcp .
./bin/dank-mcp --help 2>&1 | grep fetch  # confirms flag present (added in Task 13; skip if not yet)
```

- [ ] **Step 5: Commit**

```bash
git add internal/fetch/
git commit -m "Add TTY-gated bubbles progress reporter for downloads"
```

---

## Task 13: Wire flags into `main.go` — `--fetch`, `--fetch-only`, `--force`

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Add flag declarations**

In `main.go`, inside `main()`, find the `pflag.BoolVarP(&showHelp, ...)` line. Add these flag declarations immediately before it:

```go
	var fetchID string
	var fetchOnly, forceFetch bool
	pflag.StringVarP(&fetchID, "fetch", "", "", "Dataset id to download from dank-data (e.g., us/ct)")
	pflag.BoolVarP(&fetchOnly, "fetch-only", "", false, "Download only; do not start the MCP server")
	pflag.BoolVarP(&forceFetch, "force", "", false, "Force re-download even if cache is fresh (requires --fetch)")
```

- [ ] **Step 2: Validate flag combinations after `pflag.Parse()`**

Immediately after the `pflag.Parse()` call:

```go
	if forceFetch && fetchID == "" {
		fmt.Fprintln(os.Stderr, "--force requires --fetch <id>")
		os.Exit(2)
	}
	if fetchOnly && fetchID == "" {
		fmt.Fprintln(os.Stderr, "--fetch-only requires --fetch <id>")
		os.Exit(2)
	}
```

- [ ] **Step 3: Add import for `internal/fetch`**

At the top of `main.go`, add to the import block:

```go
	"github.com/AgentDank/dank-mcp/internal/fetch"
```

- [ ] **Step 4: Insert the fetch block**

After the logger is set up (the block ending `logger.Info("dank-mcp")`) and before the DuckDB-open section, add:

```go
	// Optional data fetch from dank-data catalog.
	if fetchID != "" {
		cachePath := data.GetDatasetCachePath(fetchID)
		resolved, err := fetch.Download(context.Background(), fetchID, fetch.Options{
			CachePath: cachePath,
			Logger:    logger,
			Force:     forceFetch,
		})
		if err != nil {
			logger.Error("fetch failed", "id", fetchID, "error", err.Error())
			os.Exit(1)
		}
		if fetchOnly {
			logger.Info("fetch-only complete", "id", fetchID, "path", resolved)
			return
		}
		// If the user didn't explicitly pass --db, serve from the fetched file.
		if config.DuckDBFile == "" {
			config.DuckDBFile = resolved
		}
	}
```

Also add `"context"` to the import block.

- [ ] **Step 5: Build**

```bash
go build ./...
```

Expected: succeeds.

- [ ] **Step 6: Manual smoke — verify flag surface**

```bash
./bin/dank-mcp --help 2>&1 | grep -E '(fetch|force)'
```

Expected: shows `--fetch`, `--fetch-only`, `--force` lines.

- [ ] **Step 7: Commit**

```bash
git add main.go
git commit -m "Wire --fetch/--fetch-only/--force flags into main"
```

---

## Task 14: `--list` flag

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Add `--list` flag declaration**

Alongside the other new flag declarations in `main()`:

```go
	var listCatalog bool
	pflag.BoolVarP(&listCatalog, "list", "", false, "List datasets from the dank-data catalog and exit")
```

- [ ] **Step 2: Add list-handler block immediately after the `--help` handler**

After the `if showHelp { ... os.Exit(0) }` block:

```go
	if listCatalog {
		cat, err := catalog.Fetch(context.Background(), catalog.DefaultURL, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to fetch catalog: %s\n", err.Error())
			os.Exit(1)
		}
		ids := make([]string, 0, len(cat.Datasets))
		for id := range cat.Datasets {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		fmt.Fprintln(os.Stdout, "ID\tTITLE\tUPDATED\tDESCRIPTION")
		for _, id := range ids {
			e := cat.Datasets[id]
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\n", id, e.Title, e.UpdatedAt, e.Description)
		}
		os.Exit(0)
	}
```

- [ ] **Step 3: Add the new imports**

In `main.go`'s import block:

```go
	"sort"

	"github.com/AgentDank/dank-mcp/internal/catalog"
```

- [ ] **Step 4: Build**

```bash
go build ./...
```

Expected: succeeds.

- [ ] **Step 5: Manual smoke (once dank-data catalog.json is live)**

```bash
./bin/dank-mcp --list
```

Expected: prints at minimum a `us/ct` row. If dank-data hasn't shipped catalog.json yet, expect a "failed to fetch catalog" error — that's acceptable for now.

- [ ] **Step 6: Commit**

```bash
git add main.go
git commit -m "Add --list flag to print catalog entries"
```

---

## Task 15: Update README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Loosen the Status banner**

Replace the existing `> [!NOTE]` block with:

```markdown
> [!NOTE]
> **Status: under refactor.** `dank-mcp` is being reworked toward a generic, declarative dataset-binding architecture (see [Notes on Design](#notes-on-design)). You can bootstrap a dataset via `--fetch <id>`, which downloads a prebuilt snapshot from [`dank-data`](https://github.com/AgentDank/dank-data). Today the only dataset is `us/ct`.
```

- [ ] **Step 2: Add a "Loading data" section**

Insert immediately before the `## Command Line Usage` section:

```markdown
## Loading Data

`dank-mcp` can download a prebuilt DuckDB snapshot from the [AgentDank `dank-data`](https://github.com/AgentDank/dank-data) repo:

```sh
$ dank-mcp --list                     # show available datasets
$ dank-mcp --fetch us/ct              # download and serve
$ dank-mcp --fetch us/ct --fetch-only # download and exit
$ dank-mcp --fetch us/ct --force      # force re-download
```

Downloads are cached at `.dank/cache/<id>/dank-data.duckdb` under `--root` (or the current directory). The cache is re-used for 7 days before a new download happens; use `--force` to override.

The snapshot's SHA-256 is verified against the catalog before install, and the local file is atomically replaced via rename — there's no window where a torn file is visible.
```

- [ ] **Step 3: Update the Command Line help block**

Replace the flag listing under `## Command Line Usage` with the current output:

```
usage: ./bin/dank-mcp [opts]

      --db string         DuckDB data file to use, use ':memory:' for in-memory. Default is '.dank/dank-mcp.duckdb' under --root
      --fetch string      Dataset id to download from dank-data (e.g., us/ct)
      --fetch-only        Download only; do not start the MCP server
      --force             Force re-download even if cache is fresh (requires --fetch)
  -h, --help              Show help
      --list              List datasets from the dank-data catalog and exit
  -l, --log-file string   Log file destination (or MCP_LOG_FILE envvar). Default is stderr
  -j, --log-json          Log in JSON (default is plaintext)
      --root string       Set root location of '.dank' dir (Default: current dir)
      --sse               Use SSE Transport (default is STDIO transport)
      --sse-host string   host:port to listen to SSE connections
  -v, --verbose           Verbose logging
```

- [ ] **Step 4: Add a TOC entry for "Loading Data"**

In the TOC list near the top, insert `* [Loading Data](#loading-data)` between `Command Line Usage` and the preceding line so order becomes: `Using with LLMs` → `Loading Data` → `Command Line Usage` → `Building`.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "Document --fetch/--list usage in README"
```

---

## Task 16: Update AGENTS.md

**Files:**
- Modify: `AGENTS.md`

- [ ] **Step 1: Add a "Dataset catalog" section**

Insert after the "Layout" section and before "Do / Don't":

```markdown
## Dataset catalog

Datasets are resolved via a remote `catalog.json` hosted at:

```
https://raw.githubusercontent.com/AgentDank/dank-data/main/snapshots/catalog.json
```

- Schema: see `docs/superpowers/specs/2026-04-21-dank-data-catalog-download-design.md`.
- Client code lives in `internal/catalog` (fetch + parse + lookup) and `internal/fetch` (download pipeline: sha256-verify → zstd-decompress → atomic rename).
- Cache layout: `.dank/cache/<id>/dank-data.duckdb`.
- TTL: 7 days. Override with `--force`.
- Offline degrade: a catalog or download failure with an existing cache logs `slog.Warn` and proceeds using the stale cache; no cache → hard error.
```

- [ ] **Step 2: Update "Known drift to clean up"**

Remove the obsolete `stdio-schema` bullet only if Taskfile.yml's `stdio-schema` task no longer uses `--no-fetch` (not part of this feature — leave as-is). No change needed for this task.

- [ ] **Step 3: Commit**

```bash
git add AGENTS.md
git commit -m "Document dataset catalog + fetch pipeline in AGENTS.md"
```

---

## Task 17: Final verification

**Files:** none

- [ ] **Step 1: Run the full test suite**

```bash
go test ./...
```

Expected: all packages PASS.

- [ ] **Step 2: Run `go vet`**

```bash
go vet ./...
```

Expected: no diagnostics.

- [ ] **Step 3: Build the binary**

```bash
task build
```

Expected: `bin/dank-mcp` produced.

- [ ] **Step 4: Verify help output**

```bash
./bin/dank-mcp --help
```

Expected: `--fetch`, `--fetch-only`, `--force`, `--list` flags all present in the output.

- [ ] **Step 5: Smoke test the MCP tool loop (no fetch)**

```bash
echo '{"method":"tools/list","params":{},"jsonrpc":"2.0","id":1}' | ./bin/dank-mcp --db ./dank-mcp.duckdb 2>/dev/null
```

Expected: single-line JSON-RPC response listing the `query` tool.

- [ ] **Step 6: Manual dank-data smoke (do this AFTER dank-data's catalog.json ships)**

```bash
./bin/dank-mcp --fetch us/ct --fetch-only --root /tmp/dank-smoke
ls -la /tmp/dank-smoke/.dank/cache/us/ct/dank-data.duckdb
```

Expected: cached file exists; size > 0.

- [ ] **Step 7: No commit — this task is verification only**

---

## Notes for the implementer

- **Imports**: Go errors loudly on unused imports, and each task adds imports to files it also edits. After each "import" instruction, run `go build ./...` or let `goimports` / your editor fix them — the plan lists the new imports but won't restate the full import block.
- **Test isolation**: tests rely on `t.TempDir()` for cache paths; never use a fixed path that could collide across parallel test runs.
- **Commit discipline**: each task ends with a commit. Squash-merging at PR time is fine; the per-task commits exist so review can follow the TDD flow.
- **Progress UI**: if you're running the test suite locally in an interactive terminal, the bubbletea TTY detection might pick up your terminal and try to render. That's harmless — the tests don't assert on progress output — but if you see stray progress characters in test output, check that `newProgressReporter` is correctly gated on `term.IsTerminal`.
- **dank-data readiness**: Task 17's Step 6 depends on the `dank-data` repo shipping `snapshots/catalog.json` with a valid `us/ct` entry. Until that lands, the manual smoke test will fail at the catalog fetch step — that's expected and not a code defect.
