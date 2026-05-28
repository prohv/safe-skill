# SafeSkill CLI

**Local security gateway for package installations.**

Intercept, scan, and block malicious npm packages at install time — offline-first, deterministic, zero external dependencies. Proxy mode streams tarballs entirely in-memory — no temp files, no disk I/O. Written in pure Go (stdlib only).

## Installation

### npm (recommended)

```bash
npm install safe-skill-cli
safe-skill scan ./path/to/package
```

The npm package ships platform binaries for Windows, macOS, and Linux. On `postinstall`, `install.js` copies the correct binary to `bin/safeskill`.

### Build from source

```bash
git clone <repo-url>
cd safe-skill
go build ./cmd/safeskill/
```

Requires Go 1.25.2 or later.

## Quick Start

```bash
# Scan a local package directory
safeskill scan ./some-package

# Intercept live npm installs (one-shot)
safeskill proxy wrap -- npm install express

# Intercept with lifecycle (setup → Ctrl+C → teardown)
safeskill proxy run

# REST API for agent/CI integration
safeskill api start
```

## How It Works

### 1. Three Operational Modes

SafeSkill runs in three independent modes, each sharing the same core scanner engine:

| Mode | Command | Purpose |
|------|---------|---------|
| **Standalone Scan** | `safeskill scan <path>` | Offline directory scan — no network, no proxy |
| **Proxy Interception** | `safeskill proxy start/run/wrap` | Transparent HTTP reverse proxy that intercepts `npm install` traffic |
| **API Server** | `safeskill api start` | REST API for agent, CI, and programmatic integration |

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          safeskill CLI                                   │
│  cmd/safeskill/main.go — flag-based subcommand dispatch                  │
│                                                                          │
│  ┌──────────┐  ┌──────────────────┐  ┌────────────┐  ┌───────────────┐   │
│  │ scan     │  │ proxy            │  │ api        │  │ report <id>   │   │
│  │ <path>   │  │ start/run/wrap/  │  │ start      │  │               │   │
│  │          │  │ setup/tear       │  │            │  │               │   │
│  └────┬─────┘  └────────┬─────────┘  └──────┬─────┘  └───────┬───────┘   │
│       │                 │                   │                │           │
│       ▼                 ▼                   ▼                ▼           │
│  ┌──────────┐     ┌──────────┐        ┌────────┐      ┌──────────┐       │
│  │ scanner/ │     │  proxy/  │        │  api/  │      │ report/  │       │
│  └──────────┘     └──────────┘        └────────┘      └──────────┘       │
│       │                 │                    │               │           │
│       └─────────────────┴──── engine/ ───────┘───────────────┘           │
│                              │                                           │
│                         ┌────┴─────┐                                     │
│                         │ rules/   │                                     │
│                         │ (6 rules)│                                     │
│                         └──────────┘                                     │
└──────────────────────────────────────────────────────────────────────────┘
```

### 2. Data Flow: Standalone Scan

```
safeskill scan ./some-package
  │
  ├─ 1. scanner.Walk(root)
  │      → Recursively walks directory tree
  │      → Filters by source extensions: .js, .mjs, .cjs, .ts, .tsx, .sh, .bash, .py, .json
  │      → Skips: node_modules/, .git/, .safeskill/
  │      → Caps individual files at 1 MB
  │      → Returns []string (file paths)
  │
  ├─ 2. scanner.NewPool(workers, rules).Run(files)
  │      → Creates N goroutine workers (default 4)
  │      → Workers read from a buffered jobs channel
  │      → Each worker reads one file, runs all 6 builtin rules against content
  │      → Each rule returns (matched, message) → Signal{Rule, Message, Severity}
  │      → Results collected on a buffered results channel
  │
  ├─ 3. scanner.Aggregate(results)
  │      → Deduplicates signals by "Rule:Message" composite key
  │      → Sorts signals by severity descending
  │      → Sums all unique severities → raw score
  │
  ├─ 4. engine.ApplyBoosts(signals, score)
  │      → Combination boosts (when co-occurring signals detected)
  │      → Critical override (severity ≥ 80 → score = 100)
  │
  ├─ 5. engine.Classify(score)
  │      → 0–29: SAFE, 30–69: WARNING, 70+: BLOCKED
  │
  └─ 6. report.New(signals, score, status)
       → Generates UUID v4 report ID (RFC 4122, version 4)
       → Outputs formatted JSON to stdout
       → Optionally writes to file (--output flag)
```

### 3. Data Flow: Proxy Interception

```
npm install malicious-skill/package
  │
  ▼  HTTP request to registry.npmjs.org
  │
safeskill proxy (listening on :8080)
  │
  ├─ 1. handler(w, r) intercepts every request
  │
  ├─ 2. isTarballURL(r.URL.Path)
  │      → Checks for .tgz or .tar.gz suffix
  │      → NO → forward to upstream registry unmodified (passthrough)
  │      → YES → interception begins
  │
  ├─ 3. Fetch tarball from upstream registry
  │      → New HTTP request with preserved headers (Accept, User-Agent, etc.)
  │      → 30-second client timeout
  │      → 100 MB LimitReader on response body (OOM protection)
  │
  ├─ 4. isTarballContent(resp)
  │      → Checks Content-Type header (application/gzip, application/x-tar, etc.)
  │      → NOT a tarball → writeAllowResponse → forward original response unmodified
  │
  ├─ 5. Cache lookup
  │      → SHA256 hash of full tarball body
  │      → Check .safeskill/cache/{hash}.json
  │      → HIT → skip extraction + scan, use cached Report
  │      → MISS → continue to extraction
  │
  ├─ 6. ScanTarballInMemory(body, workers)
  │      → gzip.NewReader → tar.NewReader → streaming extraction
  │      → Safety guards applied inline (see section 7)
  │      → Each file read into memory buffer → run all 6 rules → discard buffer
  │      → No temp files, no disk writes, no cleanup
  │
  ├─ 7. Parallel worker pool
  │      → Goroutines scan in-memory buffers concurrently
  │      → Aggregate → Boost → Classify → Report (same pipeline as standalone)
  │
  ├─ 8. Cache result → cc.Store(hash, report)
  │
  ├─ 9. Persist report → report.Save(".safeskill/reports", report)
  │
  ├─ 10. Decision
  │      → Check: result.Status == BLOCKED? OR result.Score >= custom threshold?
  │      │
  │      ├─ BLOCKED → writeBlockResponse(w, result)
  │      │    → HTTP 403 Forbidden
  │      │    → JSON body: {reason, status:"BLOCKED", risk, signals, report_id}
  │      │    → Log: "[pkg] BLOCKED risk=N signals=M"
  │      │
  │      └─ ALLOWED → writeAllowResponse(w, code, headers, body)
  │           → Original status code + headers (hop-by-hop filtered)
  │           → Original tarball body forwarded to npm
  │           → Log: "[pkg] SAFE|WARNING risk=N signals=M"
  │
  └─ npm receives either original tarball or 403 JSON
```

### 4. Data Flow: API Server

```
safeskill api start → listens on :9090

POST /scan                  POST /scan-install          GET /report/{id}
  │                           │                           │
  ├─ Decode {path}            ├─ Decode {path}            ├─ report.Load(dir, id)
  ├─ proxy.RunScan(path)      ├─ proxy.RunScan(path)      └─ JSON encoded Report
  ├─ report.Save(report)      ├─ report.Save(report)
  └─ full Report JSON         └─ compact JSON
                               {report_id, action, risk}
```

### 5. Detection Pipeline (Core Components)

| Component | Package | File | Responsibility |
|-----------|---------|------|----------------|
| `Walk()` | `scanner/` | `traversal.go:29` | Filesystem recursion, ext/limit/skip filtering |
| `Pool.Run()` | `scanner/` | `pool.go:31` | Fan-out worker pool with buffered channels + WaitGroup |
| `Aggregate()` | `scanner/` | `aggregator.go:9` | Dedup, sort, additive scoring |
| `ApplyBoosts()` | `engine/` | `boosts.go:5` | Combo bonuses + critical override |
| `Classify()` | `engine/` | `decision.go:9` | Threshold classification (SAFE/WARNING/BLOCKED) |
| `Report{}` | `report/` | `report.go:14` | UUID v4 generation, JSON marshaling, disk Save/Load |

### 6. Detection Rules (6 Built-in)

| Rule | Severity | Constant | Patterns Detected |
|------|----------|----------|-------------------|
| **ShellExec** | 80 (Critical) | `SeverityCritical` | `curl \| sh`, `wget \| bash` |
| **DynamicEval** | 50 (High) | `SeverityHigh` | `eval(`, `new Function(` |
| **NetworkAccess** | 30 (Medium) | `SeverityMedium` | `fetch(`, `axios`, `XMLHttpRequest`, `http://` URLs, `require('net')` |
| **EnvAccess** | 30 (Medium) | `SeverityMedium` | `process.env`, `os.environ`, `getenv(`, `$ENV{` |
| **Obfuscation** | 30 (Medium) | `SeverityMedium` | Base64 strings (40+ chars), lines >500 characters |
| **ChildProcess** | 10 (Low) | `SeverityLow` | `exec(`, `spawn(`, `child_process` |

All rules are stateless structs implementing the `types.Rule` interface and registered in `rules.BuiltinRules()` (`internal/rules/registry.go`).

### 7. Tarball Extraction Safety Guards

Applied during proxy-mode tarball streaming (`internal/proxy/scan_inmem.go`):

| Guard | Limit | Implementation |
|-------|-------|----------------|
| Zip-slip protection | — | `isSubPath()` verifies resolved path stays under extraction root |
| Max extraction depth | 10 levels | `relDepth()` counts path separators from root directory |
| Per-file size limit | 1 MB | `hdr.Size > maxExtractSize` → file skipped |
| Total extraction limit | 50 MB | Cumulative `totalWritten` tracker rejects overflow |
| Symlink escape detection | — | Symlink target resolved and checked against root boundary |
| HTTP client timeout | 30s | `http.Client{Timeout: 30 * time.Second}` on upstream fetch |
| Response body limit | 100 MB | `io.LimitReader(resp.Body, maxTotalExtract*2)` |
| Hop-by-hop header strip | 7 headers | `Connection`, `Keep-Alive`, `Transfer-Encoding`, `TE`, `Trailer`, `Upgrade`, `Proxy-*` |

### 8. Caching System

| Property | Detail |
|----------|--------|
| **Cache key** | SHA256 hash of full tarball body |
| **Storage location** | `.safeskill/cache/{sha256hash}.json` |
| **Storage format** | JSON: `{hash, report, timestamp}` |
| **Default TTL** | 24 hours |
| **TTL = 0** | Cache disabled |
| **Pruning** | `Cache.Prune()` runs on startup, removes expired entries |
| **Sources** | `internal/cache/cache.go` — 100 lines, zero dependencies |

### 9. Scoring & Decision Model

```
Raw score = ∑(severity of each unique signal)

Combination boosts (applied after raw sum):
  Obfuscation + DynamicEval        → +30
  NetworkAccess + EnvAccess        → +25

Critical override (applied after boosts):
  Any signal with severity ≥ 80    → score = 100 (instant BLOCKED)

Final classification:
   0–29   → SAFE     (green)   — forwarded to npm
  30–69   → WARNING  (yellow)  — forwarded with logged report
  70+     → BLOCKED  (red)     — HTTP 403, install blocked
```

The scoring pipeline is in `internal/engine/decision.go` and `internal/engine/boosts.go`. Threshold is hardcoded at 70 but overridable via `--threshold` flag or config file.

## Commands Reference

### `scan <path>`

Scan a local directory for malicious patterns.

```bash
safeskill scan ./path/to/package --workers 8 --output report.json
```

| Flag | Default | Description |
|------|---------|-------------|
| `--workers` | 4 | Number of concurrent scan workers |
| `--output` | "" | Write JSON report to file path |

### `proxy start`

Start the HTTP reverse proxy (blocking, Ctrl+C to stop).

```bash
safeskill proxy start --port 8080 --upstream https://registry.npmjs.org
```

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | 8080 | Proxy listen port |
| `--upstream` | https://registry.npmjs.org | Upstream npm registry URL |
| `--threshold` | 0 | Override block threshold (0 = engine default 70) |
| `--workers` | 4 | Number of scan workers |

### `proxy run`

Setup npm config → start proxy → wait for Ctrl+C → teardown npm config. Single-command lifecycle.

```bash
safeskill proxy run --port 8080
```

Accepts the same flags as `proxy start`.

### `proxy wrap -- <npm command>`

Setup npm config → start proxy → run npm command → teardown. One-shot interception.

```bash
safeskill proxy wrap -- npm install express
safeskill proxy wrap -- npm install react axios lodash
```

Accepts the same flags as `proxy start` before the `--` separator.

### `proxy setup` / `proxy tear`

```bash
safeskill proxy setup    # npm config set registry=http proxy=localhost:8080 https-proxy=off
safeskill proxy tear     # npm config delete registry proxy https-proxy
```

Manual configuration management. Use when you want to control the proxy lifecycle yourself.

### `api start`

Start the REST API server for agent/CI integration.

```bash
safeskill api start --port 9090 --reports-dir .safeskill/reports
```

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | 9090 | API listen port |
| `--reports-dir` | .safeskill/reports | Report persistence directory |
| `--workers` | 4 | Number of scan workers |

### `report <id>`

Fetch a saved scan report by UUID.

```bash
safeskill report 0095f96f-58a4-4af7-a19d-93d60accfea8
```

Reports are loaded from `.safeskill/reports/{id}.json`.

## Configuration

SafeSkill reads an optional JSON config file from `.safeskill/config.json` in the working directory. CLI flags override config values.

### Config file schema

```json
{
  "threshold": 50,
  "workers": 8,
  "proxy": {
    "port": 8080,
    "upstream": "https://registry.npmjs.org"
  },
  "cache": {
    "enabled": true,
    "ttl": "24h"
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `threshold` | int | 0 (engine default: 70) | Score threshold for blocking. 0 = use engine default |
| `workers` | int | 4 | Number of concurrent scan workers |
| `proxy.port` | int | 8080 | Proxy listen port |
| `proxy.upstream` | string | `https://registry.npmjs.org` | Upstream registry URL |
| `cache.enabled` | bool | true | Enable SHA256 tarball result caching |
| `cache.ttl` | string | `"24h"` | Cache TTL in Go duration format (0 = caching disabled) |

Merge priority: **hardcoded defaults < config file < CLI flags**.

## Project Structure

```
cmd/
└── safeskill/
    └── main.go              # CLI entry — flag-based subcommand dispatch (scan/proxy/api/report)

internal/
├── api/
│   ├── server.go            # HTTP API server (port 9090, graceful shutdown)
│   └── handlers.go          # POST /scan, POST /scan-install, GET /report/{id}
├── cache/
│   └── cache.go             # SHA256 tarball hashing, disk cache with TTL pruning
├── cli/
│   └── color.go             # ANSI color helpers, terminal detection, block prompt format
├── config/
│   └── config.go            # .safeskill/config.json loader with CacheTTL() parser
├── engine/
│   ├── decision.go          # Classify(): SAFE/WARNING/BLOCKED threshold logic
│   └── boosts.go            # ApplyBoosts(): combo bonuses + critical severity override
├── proxy/
│   ├── server.go            # HTTP reverse proxy server, handler, tarball intercept flow
│   ├── scan_inmem.go        # In-memory tarball streaming + parallel rule scanning
│   ├── extract.go           # On-disk tar+gzip extraction for API/standalone mode
│   ├── tarball.go           # URL/Content-Type tarball detection, package name extraction
│   ├── pipeline.go          # RunScan() — Walk → Pool → Aggregate → Boost → Classify → Report
│   ├── respond.go           # writeAllowResponse / writeBlockResponse (HTTP 403 JSON)
│   └── log.go               # Structured per-package logging to stderr
├── report/
│   └── report.go            # Report struct, UUID v4, JSON marshal, Save/Load to disk
├── rules/
│   ├── shell_exec.go        # curl|sh, wget|bash detection (Critical)
│   ├── dynamic_eval.go      # eval(), new Function() detection (High)
│   ├── network.go           # fetch, axios, XMLHttpRequest, URL detection (Medium)
│   ├── env_access.go        # process.env, os.environ detection (Medium)
│   ├── obfuscation.go       # base64, long line detection (Medium)
│   ├── child_process.go     # exec(), spawn() detection (Low)
│   └── registry.go          # BuiltinRules() — returns all 6 rule instances
├── scanner/
│   ├── traversal.go         # Walk() — recursive dir walk with ext filter, skip dirs, size limit
│   ├── pool.go              # Pool — concurrent worker pool (buffered channels, WaitGroup)
│   └── aggregator.go        # Aggregate() — dedup, sort, additive scoring
└── types/
    ├── rule.go              # Rule interface, severity constants (Low/Medium/High/Critical)
    └── signal.go            # Signal struct (Rule, Message, Severity)

testdata/
├── safe-pkg/
│   └── index.js             # Benign test package (no signals)
├── suspicious-pkg/
│   └── evil.js              # Malicious test package (triggers all 6 rules)
└── fixtures/
    └── .gitkeep

bin/                          # Platform binaries (shipped via npm package)
├── safeskill-darwin
├── safeskill-linux
├── safeskill-win.exe
└── .gitkeep

install.js                    # npm postinstall — copies platform binary to bin/safeskill
package.json                  # npm package safe-skill-cli (v0.0.4)
go.mod                        # Go module: safeskill (Go 1.25.2, zero external dependencies)
```

## Detection Rules Detail

### ShellExec (Severity: 80 — Critical)

Detects pipelines that download and execute remote code:

```go
patterns: []string{
    `curl\s.*\|.*\s?(?:ba)?sh`,
    `wget\s.*\|.*\s?(?:ba)?sh`,
}
```

### DynamicEval (Severity: 50 — High)

Detects dynamic code evaluation:

```go
patterns: []string{
    `\beval\s*\(`,
    `new\s+Function\s*\(`,
}
```

### NetworkAccess (Severity: 30 — Medium)

Detects outbound network requests from within the package:

```go
patterns: []string{
    `\bfetch\s*\(`,
    `\baxios\b`,
    `XMLHttpRequest`,
    `https?://[^\s"'\)]+`,
    `require\(['"]https?`,
    `require\(['"]net['"]`,
}
```

### EnvAccess (Severity: 30 — Medium)

Detects environment variable access:

```go
patterns: []string{
    `process\.env`,
    `os\.environ`,
    `\bgetenv\s*\(`,
    `\$ENV\{`,
}
```

### Obfuscation (Severity: 30 — Medium)

Detects obfuscated payloads:

```go
- base64 strings ≥ 40 characters   → regex: [A-Za-z0-9+/]{40,}={0,2}
- single lines > 500 characters    → line length check
```

### ChildProcess (Severity: 10 — Low)

Detects child process spawning — primarily a telemetry signal:

```go
patterns: []string{`\bexec\s*\(`, `\bspawn\s*\(`, `child_process`}
```

## Rule Authoring

Custom rules implement the `types.Rule` interface:

```go
import "safeskill/internal/types"

type MyRule struct{}

func (r MyRule) Name() string                   { return "MyCustomRule" }
func (r MyRule) Severity() int                  { return types.SeverityMedium }
func (r MyRule) Check(content string) (bool, string) {
    // Return (matched, message) if the pattern is found
    return strings.Contains(content, "dangerous"), "uses dangerous pattern"
}
```

Register by adding to `rules.BuiltinRules()` in `internal/rules/registry.go`:

```go
func BuiltinRules() []types.Rule {
    return []types.Rule{
        ShellExecRule{},
        DynamicEvalRule{},
        NetworkRule{},
        EnvAccessRule{},
        ObfuscationRule{},
        ChildProcessRule{},
        MyRule{},  // <-- add your custom rule
    }
}
```

Severity constants:

| Constant | Value |
|----------|-------|
| `types.SeverityLow` | 10 |
| `types.SeverityMedium` | 30 |
| `types.SeverityHigh` | 50 |
| `types.SeverityCritical` | 80 |

## Design Principles

- **Local-first** — no cloud dependency, no data exfiltration. Every scan runs entirely on your machine.
- **Deterministic** — same package always produces the same verdict (no AI/ML). Output is reproducible.
- **Fast** — concurrent worker pool targets <1–2s scan time. Pure in-memory streaming for proxy mode avoids disk I/O entirely.
- **Safe** — never executes scanned code. Zip-slip, symlink, depth, and size limits enforced on all extractions.
- **Agent-friendly** — structured JSON output (report ID, risk score, signals, summary) for machine consumption. REST API for agent/CI integration.
- **Zero dependencies** — pure Go standard library. No cobra, no uuid, no external frameworks. Single `go.mod` line.

## Development

```bash
go test -count=1 ./...                        # run all unit + integration tests
go test -race -count=1 ./...                  # run with race detector
go test -bench=. -benchmem ./internal/proxy/  # run benchmarks (Walk, Pool, Extract, Pipeline)
go vet ./...                                  # static analysis
go build ./cmd/safeskill/                     # build binary
```

## Changelog

See [CHANGELOG.md](./CHANGELOG.md) for version history and detailed release notes.

## License

MIT

---

Made with Codex
