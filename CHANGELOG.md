# Changelog

## [0.0.3-alpha] — 2026-05-27

### Added
- HTTP API server on configurable port (9090) with graceful shutdown
- `POST /scan` — submit package path, returns full scan report JSON
- `POST /scan-install` — submit from proxy flow, returns compact `{report_id, action, risk}`
- `GET /report/{id}` — fetch past scan report by UUID v4
- Report persistence: JSON files on disk (`.safeskill/reports/{uuid}.json`)
- UUID v4 report IDs (replaced 8-byte hex) with RFC 4122 version/variant bits
- `Save(dir, *Report)` and `Load(dir, id)` functions on report package
- CLI: `safeskill api start [--port] [--reports-dir] [--workers]`
- CLI: `safeskill report <id>` — load and print saved report
- Proxy auto-persists scan reports to disk after every tarball intercept
- 9 unit tests covering API handlers (scan, scan-install, report, errors, config)
- 6 integration tests covering API pipeline (safe/suspicious scan, report round-trip, not found, disk persistence)

## [0.0.2-alpha] — 2026-05-27

### Added
- HTTP reverse proxy server with configurable port and upstream registry (`proxy start`)
- NPM tarball detection via URL pattern (`.tgz`, `.tar.gz`) and Content-Type header
- Streaming tar+gzip extraction with safety guards:
  - Zip-slip path traversal protection
  - Maximum extraction depth (10 levels)
  - Per-file size limit (1 MB)
  - Total extraction size limit (50 MB)
  - Symlink escape detection
- Scan pipeline integration: fetch upstream → extract → Walk → Pool → Aggregate → Classify
- Block behavior: HTTP 403 with JSON response body (`reason`, `status`, `risk`, `signals`, `report_id`)
- Allow behavior: forward original upstream response unmodified, hop-by-hop headers stripped
- Subcommand CLI dispatch (`scan` + `proxy start`) via stdlib `flag` with per-command flag sets
- Structured per-package logging to stderr (`[package] STATUS risk=N files=M`)
- 35 unit tests covering proxy components (tarball detection, extraction, response builders, pipeline)
- 5 integration tests (mock upstream, proxy start/stop, block/allow/passthrough verification)

## [0.0.1-alpha] — 2026-05-26

### Added
- Go module bootstrap (`safeskill`) with zero external dependencies
- Shared types package: `Rule` interface, `Signal` struct, severity constants
- 6 detection rules:
  - `ShellExec` — curl|sh, wget|bash, exec, spawn, child_process
  - `DynamicEval` — eval(), new Function()
  - `NetworkAccess` — fetch, axios, XMLHttpRequest, external URLs
  - `EnvAccess` — process.env, os.environ, getenv()
  - `Obfuscation` — base64 patterns, lines >500 characters
  - `PostinstallHook` — npm lifecycle scripts in package.json
- Rule registry (`BuiltinRules()`) for pluggable detection
- Scanner engine with concurrent worker pool (buffered channels, sync.WaitGroup)
- File traversal (`Walk`) with source extension filtering and size limits
- Signal aggregation with deduplication and severity-sorted output
- Decision engine: additive scoring → SAFE (0–29) / WARNING (30–69) / BLOCKED (70+)
- JSON report output with `report_id`, `risk`, `status`, `signals`, `summary`
- CLI: `safeskill scan <path>` with `--workers` and `--output` flags
- 42 unit tests across all rules + 8 boundary tests for classification + 2 integration scenarios
- Full test pass and zero vet warnings
