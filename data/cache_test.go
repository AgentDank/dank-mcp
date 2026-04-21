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
