# SafeSkill CLI

**Local security gateway for package installations.**

SafeSkill intercepts npm package installs via a local HTTP proxy, extracts the tarball, scans it concurrently with static and heuristic detection rules, scores the risk, and enforces allow/warn/block decisions — all offline, with zero external dependencies.

## Features

- **Proxy interception** — transparent HTTP reverse proxy hooks into `npm install` traffic
- **Tarball detection** — identifies npm packages by URL pattern (`.tgz`, `.tar.gz`) and Content-Type
- **Streaming extraction** — gzip→tar pipeline with zip-slip protection, depth limits, and size guards
- **Concurrent scanning** — worker pool (default 4) runs 6 detection rules in parallel across all extracted files
- **Decision engine** — additive scoring classifies packages as SAFE (0–29), WARNING (30–69), or BLOCKED (70+)
- **Block/allow enforcement** — returns HTTP 403 with structured JSON on block; forwards original response unmodified on allow
- **Structured reporting** — JSON output with report ID, risk score, status, signal details, and summary
- **Standalone scan** — `safeskill scan <path>` works offline without a proxy server
- **Zero dependencies** — pure Go standard library. No cobra, no uuid, no external frameworks.

## Installation

### Go install

```bash
go install github.com/yourorg/safeskill/cmd/safeskill@latest
```

### Build from source

```bash
git clone <repo-url>
cd safe-skill
go build ./cmd/safeskill/
```

Requires Go 1.25.2 or later.

## Usage

### Scan a package directory

```bash
safeskill scan ./path/to/package
```

Output:

```json
{
  "report_id": "a1b2c3d4e5f6",
  "risk": 85,
  "status": "BLOCKED",
  "signals": [
    { "rule": "ShellExec", "message": "executes shell commands", "severity": 50 },
    { "rule": "NetworkAccess", "message": "makes external network requests", "severity": 30 }
  ],
  "summary": "executes shell commands"
}
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--workers` | 4 | Number of concurrent scan workers |
| `--output` | "" | Write JSON report to file |

### Start the proxy server

```bash
safeskill proxy start
```

The proxy listens on `:8080` and forwards requests to `https://registry.npmjs.org`. Configure npm to use it:

```bash
npm config set proxy http://localhost:8080
npm config set https-proxy http://localhost:8080
npm install <package>
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | 8080 | Proxy listen port |
| `--upstream` | https://registry.npmjs.org | Upstream npm registry URL |
| `--threshold` | 0 (engine default 70) | Override block score threshold |
| `--workers` | 4 | Number of scan workers |

### Start the API server

```bash
safeskill api start
```

The API server listens on `:9090` and provides machine-readable endpoints:

```bash
curl -X POST http://localhost:9090/scan -d '{"path":"./some-package"}'
# → full scan report JSON

curl -X POST http://localhost:9090/scan-install -d '{"path":"./some-package"}'
# → {"report_id":"...","action":"SAFE","risk":0}

curl http://localhost:9090/report/<uuid>
# → saved scan report
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | 9090 | API listen port |
| `--reports-dir` | .safeskill/reports | Reports directory |
| `--workers` | 4 | Number of scan workers |

### Fetch a saved report

```bash
safeskill report <uuid>
# → prints saved JSON report
```

## Configuration

SafeSkill supports optional JSON-based configuration at `.safeskill/config.json`.

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
| `threshold` | int | 70 | Score threshold for blocking |
| `workers` | int | 4 | Number of scan workers |
| `proxy.port` | int | 8080 | Proxy listen port |
| `proxy.upstream` | string | registry.npmjs.org | Upstream registry URL |
| `cache.enabled` | bool | true | Enable tarball result caching |
| `cache.ttl` | string | "24h" | Cache TTL (Go duration format) |

CLI flags override config file values. Config file sets defaults.

## Architecture

```
CLI Command
  → Proxy Server (intercept + extract + scan)
    → Decision Engine (score + classify + boost)
      → Report Engine (JSON + persistence)
        → API Server (agent integration)
```

Standalone mode: `CLI → Scanner → Decision → Report`. API mode: `HTTP → API Server → Scanner → Decision → Report`.

### Package structure

```
cmd/safeskill/main.go        # CLI entry: subcommand dispatch
internal/
├── types/                   # Rule interface, Signal struct, severity constants
├── rules/                   # 6 detection rules + registry
├── scanner/                 # File traversal, worker pool, signal aggregation
├── engine/                  # Scoring, classification, combination boosts
├── report/                  # JSON report output, Save/Load, UUID v4
├── proxy/                   # HTTP reverse proxy, tarball extraction, pipeline, response
├── api/                     # HTTP API server (POST /scan, GET /report/{id})
├── cache/                   # SHA256 tarball result caching with TTL
├── config/                  # JSON config file loader
└── cli/                     # ANSI color helpers for terminal output
testdata/                    # Test fixtures (safe + suspicious packages)
```

### Detection rules

| Rule | Severity | Detects |
|------|----------|---------|
| ShellExec | 50 | curl\|sh, wget\|bash, exec(), spawn(), child_process |
| DynamicEval | 50 | eval(), new Function() |
| NetworkAccess | 30 | fetch(), axios, XMLHttpRequest, external URLs |
| EnvAccess | 30 | process.env, os.environ, getenv() |
| Obfuscation | 20 | base64 patterns, lines >500 characters |
| PostinstallHook | 50 | npm lifecycle scripts (postinstall, preinstall, install) |

## Rule Authoring

Custom rules implement the `types.Rule` interface:

```go
import "safeskill/internal/types"

type MyRule struct{}

func (r MyRule) Name() string                   { return "MyCustomRule" }
func (r MyRule) Severity() int                  { return types.SeverityMedium }
func (r MyRule) Check(content string) (bool, string) {
    // Return (matched, message) if pattern found
    return strings.Contains(content, "dangerous"), "uses dangerous pattern"
}
```

Register by adding to `rules.BuiltinRules()` in `internal/rules/registry.go`.

```go
func BuiltinRules() []types.Rule {
    return []types.Rule{
        ShellExecRule{},
        DynamicEvalRule{},
        NetworkRule{},
        EnvAccessRule{},
        ObfuscationRule{},
        PostinstallRule{},
        MyRule{},  // <-- add your custom rule
    }
}
```

## Design Principles

- **Local-first** — no cloud dependency, no data exfiltration
- **Deterministic** — same package always produces the same verdict (no AI/ML)
- **Fast** — concurrent worker pool targets <1–2s scan time
- **Safe** — never executes scanned code; zip-slip, symlink, and size limits enforced
- **Agent-friendly** — structured JSON output for machine consumption

## Changelog

See [CHANGELOG.md](./CHANGELOG.md) for version history and detailed release notes.

## Development

```bash
go test -count=1 ./...       # run all tests
go test -race -count=1 ./... # run with race detector
go test -bench=. -benchmem ./internal/proxy/  # run benchmarks
go vet ./...                 # static analysis
go build ./cmd/safeskill/    # build binary
```

## License

MIT

---

Made with Codex
