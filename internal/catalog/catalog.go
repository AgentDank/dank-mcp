// Copyright (c) 2026 Neomantra Corp

// Package catalog fetches and parses the dank-data catalog.json.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
)

const (
	currentVersion = 1

	// DefaultURL is the well-known location of the dank-data catalog.
	DefaultURL = "https://raw.githubusercontent.com/AgentDank/dank-data/main/snapshots/catalog.json"
)

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
