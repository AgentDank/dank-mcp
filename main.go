// Copyright 2025 Neomantra Corp
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/AgentDank/dank-mcp/data"
	"github.com/AgentDank/dank-mcp/internal/catalog"
	"github.com/AgentDank/dank-mcp/internal/db"
	"github.com/AgentDank/dank-mcp/internal/fetch"
	"github.com/AgentDank/dank-mcp/internal/mcp"
	"github.com/spf13/pflag"
)

///////////////////////////////////////////////////////////////////////////////

const (
	mcpServerName    = "dank-mcp"
	mcpServerVersion = "0.0.1"

	defaultSSEHostPort = ":8889"
	defaultDBFile      = "dank-mcp.duckdb"
	defaultLogDest     = "dank-mcp.log"
)

type Config struct {
	DuckDBFile string // DuckDB file to connect to

	LogJSON bool // Log in JSON format instead of text
	Verbose bool // Verbose logging

	MCPConfig mcp.Config // MCP config
}

///////////////////////////////////////////////////////////////////////////////

func main() {
	var config Config
	var dankRoot, logFilename string
	var showHelp bool

	pflag.StringVarP(&dankRoot, "root", "", "", "Set root location of '.dank' dir (Default: current dir)")
	pflag.StringVarP(&config.DuckDBFile, "db", "", "", "DuckDB data file to use, use ':memory:' for in-memory. Default is '.dank/dank-mcp.duckdb' under --root")
	pflag.StringVarP(&logFilename, "log-file", "l", "", "Log file destination (or MCP_LOG_FILE envvar). Default is stderr")
	pflag.BoolVarP(&config.LogJSON, "log-json", "j", false, "Log in JSON (default is plaintext)")
	pflag.StringVarP(&config.MCPConfig.SSEHostPort, "sse-host", "", "", "host:port to listen to SSE connections")
	pflag.BoolVarP(&config.MCPConfig.UseSSE, "sse", "", false, "Use SSE Transport (default is STDIO transport)")
	pflag.BoolVarP(&config.Verbose, "verbose", "v", false, "Verbose logging")
	var fetchID string
	var fetchOnly, forceFetch bool
	var listCatalog bool
	pflag.StringVarP(&fetchID, "fetch", "", "", "Dataset id to download from dank-data (e.g., us/ct)")
	pflag.BoolVarP(&fetchOnly, "fetch-only", "", false, "Download only; do not start the MCP server")
	pflag.BoolVarP(&forceFetch, "force", "", false, "Force re-download even if cache is fresh (requires --fetch)")
	pflag.BoolVarP(&listCatalog, "list", "", false, "List datasets from the dank-data catalog and exit")
	pflag.BoolVarP(&showHelp, "help", "h", false, "Show help")
	pflag.Parse()
	dbFlagSet := pflag.Lookup("db").Changed

	if forceFetch && fetchID == "" {
		fmt.Fprintln(os.Stderr, "--force requires --fetch <id>")
		os.Exit(2)
	}
	if fetchOnly && fetchID == "" {
		fmt.Fprintln(os.Stderr, "--fetch-only requires --fetch <id>")
		os.Exit(2)
	}

	if showHelp {
		fmt.Fprintf(os.Stdout, "usage: %s [opts]\n\n", os.Args[0])
		pflag.PrintDefaults()
		os.Exit(0)
	}

	if listCatalog {
		cat, err := catalog.Fetch(context.Background(), catalog.DefaultURL, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to fetch catalog: %s\n", err.Error())
			os.Exit(1)
		}
		ids := make([]string, 0, len(cat.Datasets))
		for id := range cat.Datasets {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		fmt.Fprintln(os.Stdout, "ID\tTITLE\tUPDATED\tDESCRIPTION")
		for _, id := range ids {
			e := cat.Datasets[id]
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\n", id, e.Title, e.UpdatedAt, e.Description)
		}
		os.Exit(0)
	}

	if config.MCPConfig.SSEHostPort == "" {
		config.MCPConfig.SSEHostPort = defaultSSEHostPort
	}

	config.MCPConfig.Name = mcpServerName
	config.MCPConfig.Version = mcpServerVersion

	if dankRoot != "" {
		data.SetDankRoot(dankRoot)
	}
	if _, err := data.EnsureDankPath(); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot access Dank root dir:'%s' err:%s\n", data.GetDankDir(), err.Error())
		os.Exit(1)
	}
	// Set up logging
	logWriter := os.Stderr // default is stderr
	if logFilename == "" { // prefer CLI option
		logFilename = os.Getenv("MCP_LOG_FILE")
	}
	if logFilename != "" {
		logFile, err := os.OpenFile(logFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open log file: %s\n", err.Error())
			os.Exit(1)
		}
		logWriter = logFile
		defer logFile.Close()
	}

	var logLevel = slog.LevelInfo
	if config.Verbose {
		logLevel = slog.LevelDebug
	}

	var logger *slog.Logger
	if config.LogJSON {
		logger = slog.New(slog.NewJSONHandler(logWriter, &slog.HandlerOptions{Level: logLevel}))
	} else {
		logger = slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{Level: logLevel}))
	}

	logger.Info("dank-mcp")

	// Optional data fetch from dank-data catalog.
	if fetchID != "" {
		cachePath := data.GetDatasetCachePath(fetchID)
		resolved, err := fetch.Download(context.Background(), fetchID, fetch.Options{
			CachePath: cachePath,
			Logger:    logger,
			Force:     forceFetch,
		})
		if err != nil {
			logger.Error("fetch failed", "id", fetchID, "error", err.Error())
			os.Exit(1)
		}
		if fetchOnly {
			logger.Info("fetch-only complete", "id", fetchID, "path", resolved)
			return
		}
		// If the user didn't explicitly pass --db, serve from the fetched file.
		if !dbFlagSet {
			config.DuckDBFile = resolved
		}
	}

	if config.DuckDBFile == "" {
		config.DuckDBFile = filepath.Join(data.GetDankDir(), defaultDBFile)
	}

	// Setup DuckDB
	if config.DuckDBFile == ":memory:" {
		logger.Warn("using in-memory database, no persistence")
	}
	duckdbConn, err := sql.Open("duckdb", config.DuckDBFile)
	if err != nil {
		logger.Error("failed to open duckdb", "error", err.Error())
		os.Exit(1)
	}

	err = db.RunMigration(duckdbConn)
	if err != nil {
		logger.Error("failed to run duckdb migration", "error", err.Error())
		duckdbConn.Close()
		os.Exit(1)
	}

	// Reload our DuckDB in read-only mode for security
	duckdbConn.Close()
	duckdbConnRO, err := sql.Open("duckdb", config.DuckDBFile+"?access_mode=read_only")
	if err != nil {
		logger.Error("failed to open duckdb read-only", "error", err.Error())
		os.Exit(1)
	}
	defer duckdbConnRO.Close()

	// Lock the connection down further via safe-mode SQL
	if err = db.RunSafeMode(duckdbConnRO); err != nil {
		logger.Error("failed to run safe mode", "error", err.Error())
		os.Exit(1)
	}

	// Run our MCP server
	mcp.SetDatabase(duckdbConnRO)
	err = mcp.RunRouter(config.MCPConfig, logger, mcp.ToolMap{
		"query": mcp.RegisterQueryTool,
	})
	if err != nil {
		logger.Error("MCP router error", "error", err.Error())
		os.Exit(1)
	}
}
