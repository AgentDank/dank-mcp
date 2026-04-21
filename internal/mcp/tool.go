// Copyright (c) 2025 Neomantra Corp

package mcp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/AgentDank/dank-mcp/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	mcp_server "github.com/mark3labs/mcp-go/server"
)

// RegisterQueryTool registers the generic "query" tool, which executes a
// read-only SQL query against the given DuckDB connection and returns CSV.
func RegisterQueryTool(mcpServer *mcp_server.MCPServer, conn *sql.DB) error {
	if conn == nil {
		return fmt.Errorf("DuckDB connection is nil")
	}
	mcpServer.AddTool(mcp.NewTool("query",
		mcp.WithDescription("Execute a read-only SQL query against the DuckDB database and return CSV results"),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("The SQL query to execute"),
		),
	), makeQueryHandler(conn))
	return nil
}

func makeQueryHandler(conn *sql.DB) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		queryStr, err := request.RequireString("sql")
		if err != nil {
			return nil, errors.New("sql must be set")
		}

		rows, err := conn.QueryContext(ctx, queryStr)
		if err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}
		defer rows.Close()

		csvData, err := db.RowsToCSV(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to convert rows to CSV: %w", err)
		}

		return mcp.NewToolResultText(csvData), nil
	}
}
