// Copyright (c) 2026 Neomantra Corp

// Package main is the dank-mcp MCP server.
package main

import (
	// Pin build-time dependencies that are not yet imported by production code.
	// These will be deleted when the catalog-download feature lands.
	_ "charm.land/bubbles/v2/progress"
	_ "charm.land/bubbletea/v2"
	_ "github.com/klauspost/compress/zstd"
	_ "golang.org/x/term"
)
