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
	"time"

	"github.com/AgentDank/dank-mcp/internal/catalog"
)

const cacheTTL = 7 * 24 * time.Hour

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

	// Force bypasses the TTL check and always re-downloads. The TTL itself
	// is added in a later task; default false.
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

	// TTL check before any network I/O
	if !opts.Force {
		if info, err := os.Stat(opts.CachePath); err == nil {
			if time.Since(info.ModTime()) < cacheTTL {
				opts.Logger.Info("cache fresh; skipping download", "id", id, "path", opts.CachePath)
				return opts.CachePath, nil
			}
		}
	}

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
		if info, statErr := os.Stat(opts.CachePath); statErr == nil {
			opts.Logger.Warn("download failed; using stale cache",
				"err", err, "path", opts.CachePath, "age", time.Since(info.ModTime()).String())
			return opts.CachePath, nil
		}
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

	reporter := newProgressReporter(resp.ContentLength)
	defer reporter.finish()

	if _, err := copyAndVerify(f, reporter.wrap(resp.Body), sha256Hex); err != nil {
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
