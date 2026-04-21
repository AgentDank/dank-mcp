// Copyright (c) 2025 Neomantra Corp

package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	// Import the DuckDB driver
	_ "github.com/marcboeker/go-duckdb/v2"
)

//go:embed duckdb_up.sql
var DuckdbUpMigration string

//go:embed duckdb_safe.sql
var DuckdbSafeMigration string

///////////////////////////////////////////////////////////////////////////////

// RunMigration executes the migration string on the DuckDB connection.
// Returns an error, if any.
func RunMigration(conn *sql.DB) error {
	_, err := conn.Exec(DuckdbUpMigration)
	if err != nil {
		return fmt.Errorf("failed to run migration: %w", err)
	}
	return nil
}

// RunSafeMode locks the database down with the DuckdbSafeMigration.
// Returns an error, if any
func RunSafeMode(conn *sql.DB) error {
	_, err := conn.Exec(DuckdbSafeMigration)
	if err != nil {
		return fmt.Errorf("failed to run safe mode migration: %w", err)
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////

// String internally quotes a string for use in a SQL query.
func String(str string) string {
	return strings.Replace(str, "'", "''", -1)
}
