// Copyright (c) 2025 Neomantra Corp

package mcp

import (
	"database/sql"
	"fmt"
	"log/slog"

	mcp_server "github.com/mark3labs/mcp-go/server"
)

const (
	regex10  = `^(0(\.\d+)?|1(\.0+)?)$`
	regexInt = `^[0-9]*$`
)

// Config is configuration for our MCP server
type Config struct {
	Name    string // Service Name
	Version string // Service Version

	UseSSE      bool   // Use SSE Transport instead of STDIO
	SSEHostPort string // HostPort to use for SSE
}

// we resort to module-global variable rather than setting up closures
var duckdbConn *sql.DB

// ToolRegistrationFunc is a function type for registering tools with the MCP server.
// It takes an MCPServer and a database, returning an error if any.
type ToolRegistrationFunc func(mcpServer *mcp_server.MCPServer, conn *sql.DB) error

// ToolMap is a map of names to ToolRegistrationFunc registration functions.
type ToolMap map[string]ToolRegistrationFunc

//////////////////////////////////////////////////////////////////////////////

// SetDatabase sets the DuckDB connection for the MCP server.
func SetDatabase(conn *sql.DB) error {
	if conn == nil {
		return fmt.Errorf("DuckDB connection is nil")
	}
	duckdbConn = conn
	return nil
}

// RunRouter runs the MCP server with the given configuration and logger.
func RunRouter(config Config, logger *slog.Logger, regs map[string]ToolRegistrationFunc) error {
	if duckdbConn == nil {
		return fmt.Errorf("DuckDB connection is nil")
	}

	// Create the MCP Server and register Tools on it
	mcpServer := mcp_server.NewMCPServer(config.Name, config.Version)
	toolCount := 0
	for name, registrator := range regs {
		if err := registrator(mcpServer, duckdbConn); err != nil {
			logger.Error("failed to register tool", "name", name, "error", err)
		} else {
			toolCount++ // track number of registered tools
		}
	}
	if toolCount == 0 {
		return fmt.Errorf("no tools registered")
	}

	// Run the appropriate server
	if config.UseSSE {
		sseServer := mcp_server.NewSSEServer(mcpServer)
		logger.Info("MCP SSE server started", "hostPort", config.SSEHostPort)
		if err := sseServer.Start(config.SSEHostPort); err != nil {
			return fmt.Errorf("MCP SSE server error: %w", err)
		}
	} else {
		logger.Info("MCP STDIO server started")
		if err := mcp_server.ServeStdio(mcpServer); err != nil {
			return fmt.Errorf("MCP STDIO server error: %w", err)
		}
	}

	return nil
}
