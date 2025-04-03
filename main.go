// Copyright 2025 Neomantra Corp

package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/AgentDank/dank-mcp/data"
	"github.com/AgentDank/dank-mcp/data/us/ct"
	"github.com/AgentDank/dank-mcp/internal/db"
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

	defaultTmpDir = ".dank"
)

type Config struct {
	AppToken string // data.ct.gov App Token

	DuckDBFile string // DuckDB file to connect to
	NoFetch    bool   // Don't fetch any data, only use what is in current DB

	LogJSON bool // Log in JSON format instead of text
	Verbose bool // Verbose logging

	MCPConfig mcp.Config // MCP config
}

///////////////////////////////////////////////////////////////////////////////

func main() {
	var config Config
	var dankRoot, logFilename string
	var onlyDump bool
	var showHelp bool

	pflag.StringVarP(&dankRoot, "root", "", "", "Set root location of '.dank' dir (Default: current dir)")
	pflag.StringVarP(&config.AppToken, "token", "t", "", "ct.data.gov App Token")
	pflag.StringVarP(&config.DuckDBFile, "db", "", "", "DuckDB data file to use, use ':memory:' for in-memory. Default is '.dank/dank-mcp.duckdb' under --root")
	pflag.StringVarP(&logFilename, "log-file", "l", "", "Log file destination (or MCP_LOG_FILE envvar). Default is stderr")
	pflag.BoolVarP(&config.LogJSON, "log-json", "j", false, "Log in JSON (default is plaintext)")
	pflag.StringVarP(&config.MCPConfig.SSEHostPort, "sse-host", "", "", "host:port to listen to SSE connections")
	pflag.BoolVarP(&config.MCPConfig.UseSSE, "sse", "", false, "Use SSE Transport (default is STDIO transport)")
	pflag.BoolVarP(&onlyDump, "dump", "", false, "Only download files and populate DB, no MCP server")
	pflag.BoolVarP(&config.NoFetch, "no-fetch", "n", false, "Don't fetch any data, only use what is in current DB")
	pflag.BoolVarP(&config.Verbose, "verbose", "v", false, "Verbose logging")
	pflag.BoolVarP(&showHelp, "help", "h", false, "Show help")
	pflag.Parse()

	if showHelp {
		fmt.Fprintf(os.Stdout, "usage: %s [opts]\n\n", os.Args[0])
		pflag.PrintDefaults()
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
	if err := data.EnsureDankRoot(); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot access Dank root dir:'%s' err:%s\n", data.GetDankDir(), err.Error())
		os.Exit(1)
	}
	if config.DuckDBFile == "" {
		config.DuckDBFile = filepath.Join(data.GetDankDir(), defaultDBFile)
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

	// Setup DuckDB
	if config.DuckDBFile == ":memory:" {
		logger.Warn("using in-memory database, no persistence")
	}
	duckdbConn, err := sql.Open("duckdb", config.DuckDBFile)
	if err != nil {
		logger.Error("failed to open duckdb", "error", err.Error())
		os.Exit(1)
	}
	defer duckdbConn.Close()

	err = db.RunMigration(duckdbConn)
	if err != nil {
		logger.Error("failed to run duckdb migration", "error", err.Error())
		os.Exit(1)
	}

	// Prime our data
	if !config.NoFetch {
		err = primeData(config, duckdbConn, logger)
		if err != nil {
			fmt.Fprintf(os.Stderr, "primeData failed %s\n", err.Error())
			os.Exit(1)
		}
	}

	if onlyDump {
		logger.Info("dumped data",
			"duckdb", config.DuckDBFile,
			"ct_brands_json", data.GetDankCachePathname(ct.BRAND_JSON_FILENAME),
			"ct_brands_csv", data.GetDankCachePathname(ct.BRAND_CSV_FILENAME))
		os.Exit(0)
	}

	// Reload our DuckDB in read-only mode for security
	duckdbConn.Close()
	duckdbConnRO, err := sql.Open("duckdb", config.DuckDBFile+"?access_mode=read_only")
	if err != nil {
		logger.Error("failed to open duckdb read-only", "error", err.Error())
		os.Exit(1)
	}
	defer duckdbConnRO.Close()

	// Run our MCP server
	mcp.SetDatabase(duckdbConnRO)
	err = mcp.RunRouter(config.MCPConfig, logger, mcp.ToolMap{
		"us_ct": ct.RegisterMCP,
	})
	if err != nil {
		logger.Error("MCP router error", "error", err.Error())
		os.Exit(1)
	}
}

////////////////////////////////////////////////////////////////////////////

// TODO: check DuckDB for latest, etc
func primeData(config Config, duckdbConn *sql.DB, logger *slog.Logger) error {
	// Fetch the Brands from ct.data.gov
	logger.Info("fetching brands from ct.data.gov")
	maxCacheAge := 24 * time.Hour
	brands, err := ct.FetchBrands(config.AppToken, maxCacheAge)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	// Clean the data
	brands = ct.CleanBrands(brands)

	// let's save a CSV file for the tokers out there
	csvFile, err := data.MakeCacheFile(ct.BRAND_CSV_FILENAME)
	if err != nil {
		return fmt.Errorf("failed to create CSV cache file: %w", err)
	}
	defer csvFile.Close()
	csvFile.WriteString(ct.Brand{}.CSVHeaders())
	for _, brand := range brands {
		csvFile.WriteString(brand.CSVValue())
	}

	// Drop it into DuckDB
	logger.Info("inserting brands into db", "count", len(brands))
	if err = ct.DBInsertBrands(duckdbConn, brands); err != nil {
		return fmt.Errorf("ct.DBInsertBrands failed: %w", err)
	}
	logger.Info("finished")
	return nil
}
