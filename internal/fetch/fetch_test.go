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
	"time"

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
	catalogTpl := fmt.Sprintf(`{
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
			fmt.Fprintf(w, catalogTpl, "http://"+r.Host)
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

func TestDownload_RejectsUnsafeID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request for unsafe id to %s", r.URL.Path)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	_, err := Download(context.Background(), "../../etc/passwd", Options{
		CatalogURL: srv.URL + "/catalog.json",
		CachePath:  filepath.Join(tmp, "dank-data.duckdb"),
		Client:     srv.Client(),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err == nil {
		t.Fatal("expected validation error for unsafe id")
	}
}
