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
