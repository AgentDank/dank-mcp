// Copyright (c) 2026 Neomantra Corp

// Package depsrefs exists only to pin build-time dependencies that are
// not yet imported by production code. It will be deleted when the
// catalog-download feature lands.
package depsrefs

import (
	_ "github.com/charmbracelet/bubbles/progress"
	_ "github.com/charmbracelet/bubbletea"
	_ "github.com/klauspost/compress/zstd"
	_ "golang.org/x/term"
)
