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

func TestValidateDatasetID(t *testing.T) {
	cases := []struct {
		id      string
		wantErr bool
	}{
		{"us/ct", false},
		{"us/ma", false},
		{"ca/on", false},
		{"us/weekly_sales", false},
		{"us/with-hyphens", false},
		{"", true},
		{"us", true},
		{"US/ct", true},
		{"us/CT", true},
		{"usa/ct", true},
		{"us/ct/extra", true},
		{"../../etc/passwd", true},
		{"..", true},
		{"/etc/passwd", true},
		{"us/ct/..", true},
		{"us/ct ", true},
		{" us/ct", true},
		{"us/ct\n", true},
	}
	for _, c := range cases {
		err := ValidateDatasetID(c.id)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateDatasetID(%q) error = %v; wantErr = %v", c.id, err, c.wantErr)
		}
	}
}
