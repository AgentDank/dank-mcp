// Copyright (c) 2025 Neomantra Corp

package db

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"strings"
)

// RowsToCSV converts sql.Rows rows to a CSV string.
func RowsToCSV(rows *sql.Rows) (string, error) {
	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("error getting column names: %w", err)
	}

	// Create CSV writer and string.Builder
	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	// Write header row
	if err := writer.Write(columns); err != nil {
		return "", fmt.Errorf("error writing header row: %w", err)
	}

	// Create a slice of interface{} to hold each row's values
	values := make([]interface{}, len(columns))
	// Create a slice of pointers to the values
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	// Iterate through rows
	for rows.Next() {
		// Scan the row into the valuePtrs slice
		if err := rows.Scan(valuePtrs...); err != nil {
			return "", fmt.Errorf("error scanning row: %w", err)
		}

		// Convert each value to string
		stringValues := make([]string, len(columns))
		for i, val := range values {
			stringValues[i] = toString(val)
		}

		// Write the row to the CSV
		if err := writer.Write(stringValues); err != nil {
			return "", fmt.Errorf("error writing row: %w", err)
		}
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error iterating through rows: %w", err)
	}

	// Make sure to flush to write any buffered data to the string builder
	writer.Flush()

	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("error flushing CSV writer: %w", err)
	}

	return sb.String(), nil
}

// Helper function to convert various types to string
func toString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
