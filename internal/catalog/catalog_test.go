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
