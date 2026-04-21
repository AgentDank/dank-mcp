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
