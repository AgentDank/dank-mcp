// Copyright (c) 2026 Neomantra Corp

// Package depsrefs exists only to pin build-time dependencies that are
// not yet imported by production code. It will be deleted when the
// catalog-download feature lands.
package depsrefs

import (
	_ "charm.land/bubbles/v2/progress"
	_ "charm.land/bubbletea/v2"
	_ "github.com/klauspost/compress/zstd"
	_ "golang.org/x/term"
)
