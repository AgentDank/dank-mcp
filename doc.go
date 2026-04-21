// Copyright (c) 2026 Neomantra Corp

// Package main is the dank-mcp MCP server.
package main

import (
	// Pin build-time dependencies that are not yet imported by production code.
	// These will be deleted when the catalog-download feature lands.
	_ "github.com/charmbracelet/bubbles/progress"
	_ "github.com/charmbracelet/bubbletea"
	_ "github.com/klauspost/compress/zstd"
	_ "golang.org/x/term"
)
