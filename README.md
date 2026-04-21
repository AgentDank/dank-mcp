# AgentDank: &nbsp;&nbsp; `dank-mcp`

`dank-mcp` is a Model Context Protocol (MCP) server for querying cannabis datasets. It is brought to you by AgentDank for educational and legal purposes.

> [!NOTE]
> **Status: under refactor.** `dank-mcp` is being reworked toward a generic, declarative dataset-binding architecture (see [Notes on Design](#notes-on-design)). You can bootstrap a dataset via `--fetch <id>`, which downloads a prebuilt snapshot from [`dank-data`](https://github.com/AgentDank/dank-data). Today the only dataset is `us/ct`.

<p align="center"><video controls src="https://private-user-images.githubusercontent.com/26842/430672597-adb8e56f-178d-4646-9e0f-4fcf57614dea.mp4" title="AgentDank "></video></p>

----

  * [Installation](#installation)
  * [Using with LLMs](#usage)
    * [Claude Desktop](#claude-desktop)
    * [Ollama and mcphost](#ollama-and-mcphost)
  * [Loading Data](#loading-data)
  * [Command Line Usage](#command-line-usage)
  * [Building](#building)
  * [Notes on Design](#notes-on-design)
  * [Contribution and Conduct](#contribution-and-conduct)
  * [Credits and License](#credits-and-license)

Snapshots of cannabis datasets are curated at the [AgentDank `dank-data`](https://github.com/AgentDank/dank-data) repository; you can point `dank-mcp` at any DuckDB file via `--db`.

## Installation

While we'd like to have pre-built binaries and Homebrew packages, we're having an issue with that right now.  So the preferred way to install is using `go install` or [building from source](#building):

```sh
$ go install github.com/AgentDank/dank-mcp@latest
```

It will be installed in your `$GOPATH/bin` directory, which is often `~/go/bin`.


## Using with LLMs

To use this `dank-mcp` MCP server, you must configure your host program to use it.  We will illustrate with [Claude Desktop](https://claude.ai/download).  We must find the `dank-mcp` program on our system; the example below shows where `dank-mcp` is installed with my `go install`.

The following configuration JSON ([also in the repo as `mcp-config.json`](./mcp-config.json)) sets this up:

```json
{
    "mcpServers": {
      "dank": {
        "command": "~/go/bin/dank-mcp",
        "args": [
          "--root", "~"
        ]
      }
    }
  }
```

### Claude Desktop

Using Claude Desktop, you can follow [their configuration tutorial](https://modelcontextprotocol.io/quickstart/user) but substitute the configuration above.  With that in place, you can ask Claude questions and it will use the `dank-mcp` server.

### Ollama and `mcphost`

**I'm currently having issues with this working well, but leaving instructions for those interested.**

For local inferencing, there are MCP hosts that support [Ollama](https://ollama.com/download).  You can use any [Ollama LLM that supports "Tools"](https://ollama.com/search?c=tools).  We experimented with [`mcphost`](https://github.com/mark3labs/mcphost), authored by the developer of the [`mcp-go` library](https://github.com/mark3labs/mcp-go) that performed the heavy lifting for us.

Here's how to install and run it with the configuration above, stored in `mcp-config.json`:

```
$ go install github.com/mark3labs/mcphost@latest
$ ollama pull llama3.3
$ mcphost -m ollama:llama3.3 --config mcp-config.json
...chat away...
```

## Loading Data

`dank-mcp` can download a prebuilt DuckDB snapshot from the [AgentDank `dank-data`](https://github.com/AgentDank/dank-data) repo:

```sh
$ dank-mcp --list                     # show available datasets
$ dank-mcp --fetch us/ct              # download and serve
$ dank-mcp --fetch us/ct --fetch-only # download and exit
$ dank-mcp --fetch us/ct --force      # force re-download
```

Downloads are cached at `.dank/cache/<id>/dank-data.duckdb` under `--root` (or the current directory). The cache is re-used for 7 days before a new download happens; use `--force` to override.

The snapshot's SHA-256 is verified against the catalog before install, and the local file is atomically replaced via rename — there's no window where a torn file is visible.

## Command Line Usage

Here is the command-line help:

```
usage: ./bin/dank-mcp [opts]

      --db string         DuckDB data file to use, use ':memory:' for in-memory. Default is '.dank/dank-mcp.duckdb' under --root
      --fetch string      Dataset id to download from dank-data (e.g., us/ct)
      --fetch-only        Download only; do not start the MCP server
      --force             Force re-download even if cache is fresh (requires --fetch)
  -h, --help              Show help
      --list              List datasets from the dank-data catalog and exit
  -l, --log-file string   Log file destination (or MCP_LOG_FILE envvar). Default is stderr
  -j, --log-json          Log in JSON (default is plaintext)
      --root string       Set root location of '.dank' dir (Default: current dir)
      --sse               Use SSE Transport (default is STDIO transport)
      --sse-host string   host:port to listen to SSE connections
  -v, --verbose           Verbose logging
```

The server currently registers a single MCP tool, `query`, which takes a `sql` string argument and returns CSV. The DuckDB is opened read-only and further locked down via `SET enable_external_access=false`, so only pure SQL over local data is permitted.

## Building

Building is performed with [task](https://taskfile.dev/):

```
$ task
task: [build] go build -o bin/dank-mcp
```

## Notes on Design

`dank-mcp` is being reworked around a small set of ideas:

1. **Locked-down read-only DuckDB** — the server opens DuckDB with `access_mode=read_only` and applies `SET enable_external_access=false` so exposing a generic SQL tool is safe.
2. **Generic `query` MCP tool** — rather than hand-rolling per-dataset tools, `dank-mcp` exposes one `query(sql)` tool that runs against whatever DuckDB you point it at.
3. **Declarative dataset bindings** (in progress) — `pkg/dank` defines a `Registrar` interface and JSON-taggable `Binding` / `ResourceQuery` / `ToolQuery` structs. The intent is that new datasets can be added as config-driven bindings rather than bespoke Go code.

The previous surface bundled a Connecticut cannabis brands dataset directly into the binary. That code has been removed; datasets will be re-introduced via the binding mechanism above.

If all you want is SQL access to a DuckDB file, MotherDuck's [`mcp-server-motherduck`](https://github.com/motherduckdb/mcp-server-motherduck) is a more general-purpose alternative. The goal of `dank-mcp` is to bundle cannabis-specific prompts, resources, and tools on top of that.

----

## Contribution and Conduct

Pull requests and issues are welcome.  Or fork it.  You do you.

Either way, obey our [Code of Conduct](./CODE_OF_CONDUCT.md).  Be shady, but don't be a jerk.

## Credits and License

Copyright (c) 2025 Neomantra Corp.  Authored by Evan Wies for [AgentDank](https://github.com/AgentDank).

Released under the [MIT License](https://en.wikipedia.org/wiki/MIT_License), see [LICENSE.txt](./LICENSE.txt).

----
Made with :herb: and :fire: by the team behind [AgentDank](https://github.com/AgentDank).
