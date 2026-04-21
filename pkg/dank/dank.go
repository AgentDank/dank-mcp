// Copyright (c) 2025 Neomantra Corp
//
// Provides structs used by Dank MCP server

package dank

import (
	"database/sql"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

///////////////////////////////////////////////////////////////////////////////

// Registrar is an interface for getting Tools and Resources  descriptions
type Registrar interface {
	// GetMigrationUp returns the SQL to query to bring up the dataset in a DuckDB.
	GetMigrationUp() string
	// GetMigrationDown returns the SQL to query to tear down the dataset in a DuckDB.
	GetMigrationDown() string
	// GetResources returns all the mcp.Resource descriptions of this dataset
	GetResources() ([]mcp.Resource, error)
	// GetTools returns all the mcp.Tool descriptions of this dataset.
	GetTools() ([]mcp.Tool, error)
	// Fetch downloads the dataset from its source.
	// Returns error if any
	Fetch(duckdbConn *sql.DB, appToken string, maxCacheAge time.Duration) error
}

// Binding is collection of Resource/Prompts/Tools that are described together along with Data
type Binding struct {
	Name      string          `json:"name"`        // Unique, human-readable name of the Binding
	Title     string          `json:"title"`       // The title of the Binding
	Desc      string          `json:"description"` // The description of the Binding
	Resources []ResourceQuery `json:"resources"`   // The list of tools to include in this binding
	Tools     []ToolQuery     `json:"tools"`       // The list of tools to include in this binding
}

///////////////////////////////////////////////////////////////////////////////
// Resources

// ResourceQuery specifies an MCP Resource that maps to a SQL query.
type ResourceQuery struct {
	Name     string `json:"name"`                  // Unique, human-readable name of the Resource
	Uri      string `json:"uri"`                   // The URI of the MCP Resource to expose
	Desc     string `json:"description,omitempty"` // The description of the Resource
	MimeType string `json:"mimeType"`              // The MIME type of the Resource
	Query    string `json:"query,omitempty"`       // The SQL query to run to get the data
	RawData  string `json:"rawData,omitempty"`     // The raw data to respond with.  It is an error to set both query and rawData (empty or null is OK).
}

///////////////////////////////////////////////////////////////////////////////
// Prompts

// TODO

///////////////////////////////////////////////////////////////////////////////
// Tools

// ToolInputSchema describes the input schema for a tool.
type ToolInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

type ToolQuery struct {
	Name        string          `json:"name"`                  // Unique, human-readable namefor the Tool
	Desc        string          `json:"description,omitempty"` // The description of the Tool
	InputSchema ToolInputSchema `json:"schema"`                // The JSON schema of the intput
	Query       string          `json:"query"`                 // The SQL query to run to get the data
}
