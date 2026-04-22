// Copyright (c) 2025 Neomantra Corp

package db

import (
	"database/sql"
	_ "embed"
	"fmt"

	// Import the DuckDB driver
	_ "github.com/duckdb/duckdb-go/v2"
)

//go:embed duckdb_safe.sql
var DuckdbSafeMigration string

///////////////////////////////////////////////////////////////////////////////

// RunSafeMode locks the database down with the DuckdbSafeMigration.
// Returns an error, if any
func RunSafeMode(conn *sql.DB) error {
	_, err := conn.Exec(DuckdbSafeMigration)
	if err != nil {
		return fmt.Errorf("failed to run safe mode migration: %w", err)
	}
	return nil
}
