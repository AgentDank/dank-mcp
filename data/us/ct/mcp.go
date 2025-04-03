// Copyright 2025 Neomantra Corp
//
// CT Cannabis Data MCP Tools
//
// Socrata Documentation:
//   https://dev.socrata.com/foundry/data.ct.gov/egd5-wb6r
// Interactive Brand Dataset:
//   https://data.ct.gov/api/views/egd5-wb6r/rows.csv?accessType=DOWNLOAD&api_foundry=true

package ct

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/AgentDank/dank-mcp/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	mcp_server "github.com/mark3labs/mcp-go/server"
)

// Our MCP Tools' DuckDB connection, set during RegisterMCP
var duckdbConn *sql.DB

// RegisterMCP registers CT MCP tools with the MCPServer
func RegisterMCP(mcpServer *mcp_server.MCPServer, conn *sql.DB) error {
	// Set the DuckDB connection
	if conn == nil {
		return fmt.Errorf("DuckDB connection is nil")
	}
	duckdbConn = conn

	// us_ct_brand_query
	mcpServer.AddTool(mcp.NewTool("us_ct_brand_query_sql",
		mcp.WithDescription(`Queries database of US Connecticut CT Cannabis brands, returnings a CSV of the query results.  The database is DuckDB and this tool performs SQL queries based on the arguments.  It is a read-only database and this is a SELECT-only endpoint. 
It has the following applied tables: `+db.DuckdbUpMigration),
		mcp.WithString("sql",
			mcp.Title("SQL statement to query"),
			mcp.Required(),
			mcp.Description(`Queries DuckDB with the SQL statement.  The sole table is 'brands_us_ct'.`),
		),
	), queryToolHandler)

	return nil
}

///////////////////////////////////////////////////////////////////////////////

func queryToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract the parameter
	if duckdbConn == nil {
		return nil, fmt.Errorf("No database")
	}
	queryStr, ok := request.Params.Arguments["sql"].(string)
	if !ok {
		return nil, errors.New("sql must be set")
	}

	// Query the database
	rows, err := duckdbConn.QueryContext(context.Background(), queryStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query candle: %w", err)
	}
	defer rows.Close()

	// Marshal results to CSV
	csvData, err := db.RowsToCSV(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to convert rows to CSV: %w", err)
	}

	// Return CSV response
	return mcp.NewToolResultText(csvData), nil
}
