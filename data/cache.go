// Copyright (c) 2025 Neomantra Corp

package data

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	DankDir  = ".dank" // DankDir is the directory where dank-mcp stores its data.
	CacheDir = "cache" // CacheDir is the directory under DankDir where dank-mcp stores its cache files.
)

var dankRoot string = "." // The root directory for dank-mcp data, default is '.'

//////////////////////////////////////////////////////////////////////////////

// SetDankRoot sets the root directory for dank-mcp data.
func SetDankRoot(root string) {
	dankRoot = root
}

// EnsureDankRoot ensures the current DankRoot exists, creating it if needed.
// Returns an error, if any.
func EnsureDankRoot() error {
	dankDir := GetDankDir()
	return os.MkdirAll(dankDir, os.ModePerm)
}

// GetDankDir returns the path to the DankDir directory.
func GetDankDir() string {
	return filepath.Join(dankRoot, DankDir)
}

// GetDankCacheDir returns the path to the CacheDir directory.
func GetDankCacheDir() string {
	return filepath.Join(dankRoot, DankDir, CacheDir)
}

// GetDankCachePathname returns the path to the given filename within the CacheDir.
func GetDankCachePathname(filename string) string {
	return filepath.Join(dankRoot, DankDir, CacheDir, filename)
}

// ChaekCacheFile checks DankDir/cache for a file.  Returns its bytes and error, if any.
// If the file is not found, it returns an error.
// If the file is older than maxAge, it returns an error.
func CheckCacheFile(filename string, maxAge time.Duration) ([]byte, error) {
	cacheFilename := filepath.Join(dankRoot, DankDir, "cache", filename)

	if stat, err := os.Stat(cacheFilename); err != nil {
		return nil, fmt.Errorf("cache file not found")
	} else if maxAge != 0 && time.Now().After(stat.ModTime().Add(maxAge)) {
		// now is past the max age
		return nil, fmt.Errorf("cache file is too old")
	}

	cacheBytes, err := os.ReadFile(cacheFilename)
	if err != nil {
		return nil, fmt.Errorf("cache file read error: %w", err)
	}
	return cacheBytes, nil
}

// MakeCacheFile creates a cache file in the DankDir/cache directory and returns its handles.
// Returns nil with any error.
func MakeCacheFile(filename string) (*os.File, error) {
	cachedFilename := filepath.Join(dankRoot, DankDir, "cache", filename)
	if err := os.MkdirAll(filepath.Dir(cachedFilename), 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cacheFile, err := os.Create(cachedFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache file: %w", err)
	}

	return cacheFile, nil
}
