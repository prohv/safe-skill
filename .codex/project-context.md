# Project Context

## Repo Status

- Workspace: `C:\Users\shado\Windsurf\outskill-hack\safe-skill`
- Branch: `dev`
- Phase 1: **Complete** — 16 atomic commits, all tests passing
- Phase 2: **Complete** — 10 atomic commits, all tests passing
- Phase 3: **Complete** — 8 atomic commits, all tests passing
- Phase 4: **Complete** — 9 atomic commits, all tests passing
- Next: v1.0 Release — Evaluate feedback, decide Phase 5

## Product Brief

SafeSkill CLI = local Go-based proxy + scanner + API for secure package installs. Intercepts npm traffic via proxy, extracts tarballs, scans concurrently with static/heuristic rules, scores risk, enforces allow/warn/block, persists reports to disk, exposes machine-readable API. Pure Go, offline-first, deterministic.

## Phase 1 — Core Scanner (Standalone CLI)

**What's built:**
- 6 rule categories: ShellExec, DynamicEval, NetworkAccess, EnvAccess, Obfuscation, PostinstallHook
- Scanner engine: file traversal (Walk), concurrent worker pool (Pool), signal dedup+sort (Aggregate)
- Decision engine: additive scoring → SAFE (0-29) / WARNING (30-69) / BLOCKED (70+)
- Report engine: structured JSON output with report_id, signatures, signals, summary
- CLI: `safeskill scan <path>` via stdlib `flag` (`--workers`, `--output`)
- 42 unit tests across 6 rules + 8 boundary tests for classification + 2 integration scenarios

## Phase 3 — Local API + Agent Integration

**What's built:**
- HTTP API server on separate port (9090) with configurable reports directory
- `POST /scan` — submit package path, returns full scan report JSON
- `POST /scan-install` — submit from proxy flow, returns compact `{report_id, action, risk}`
- `GET /report/{id}` — fetch past scan report by UUID v4
- Report persistence: JSON files on disk (`.safeskill/reports/{uuid}.json`)
- UUID v4 report IDs (previously 8-byte hex)
- Proxy auto-persists scan reports to disk after every intercept
- CLI: `safeskill api start [--port] [--reports-dir] [--workers]`
- CLI: `safeskill report <id>` — load and print saved report
- 9 unit tests + 6 integration tests for API handlers and pipeline

**Usage:**
```bash
go run ./cmd/safeskill/ api start
# → api listening on :9090

curl -X POST http://localhost:9090/scan -d '{"path":"./pkg"}'
# → full report JSON

go run ./cmd/safeskill/ report <uuid>
# → prints saved report
```

## Phase 4 — Hardening + Production Polish

**What's built:**
- HTTP client timeout (30s) — prevents hang on slow upstream
- Response body size limit (100MB LimitReader) — OOM guard
- Hop-by-hop header filter: added TE, Trailer, Upgrade to strip list
- Combination boost scoring: base64+eval +30, network+env +25, postinstall+exec +40
- Critical signal override: severity ≥ 80 → instant BLOCKED (returned as score 100)
- SHA256 tarball caching: `.safeskill/cache/{hash}.json` with configurable TTL (default 24h)
- JSON config file: `.safeskill/config.json` — proxy, cache, threshold, workers settings
- CLI flag defaults from config file (CLI flags override config)
- ANSI color output: green SAFE, yellow WARNING, red BLOCKED
- Interactive block prompt: proxy mode prompts `[f]orce  [a]bort` on block
- Performance benchmarks: Walk, Pool, ExtractTarball, FullPipeline
- Edge case tests: 1000-file tar bomb, 50MB total size limit, 10-goroutine concurrent proxy, 500-file walk
- Combination boost table tests: 9 subtests covering all pair combinations and critical override
- README: install guide, config schema, rule authoring, agent integration examples
- 6 new packages: `cache/`, `config/`, `cli/`, `engine/boosts.go`

## Phase 2 — Proxy Layer (Intercept + Enforce)

**What's built:**
- HTTP reverse proxy with configurable port + upstream URL
- NPM tarball detection via URL pattern (`.tgz`/`.tar.gz`) + Content-Type header
- Streaming tar+gzip extraction with safety guards (zip-slip, depth max 10, per-file 1MB, total 50MB, symlink escape)
- Scan pipeline: fetch upstream → extract temp dir → Walk → Pool → Aggregate → Classify
- Block behavior: HTTP 403 + JSON body `{reason, status, risk, signals, report_id}`
- Allow behavior: forward original response body with hop-by-hop headers stripped
- Subcommand dispatch: `safeskill scan <path>` and `safeskill proxy start [--port] [--upstream] [--threshold] [--workers]`
- Structured per-package logging to stderr
- 35 unit tests across proxy components + 5 integration tests (mock upstream, mock proxy, verify block/allow/passthrough)

**Usage:**
```bash
go run ./cmd/safeskill/ proxy start
# → proxy listening on :8080, upstream https://registry.npmjs.org

go test -count=1 ./...
# → all packages pass
```
```bash
go run ./cmd/safeskill/ scan ./testdata/suspicious-pkg/
# → JSON report with risk score, signals, classification

go test ./...
# → all packages pass

go vet ./...
# → no issues
```

## Project Structure

```
cmd/safeskill/main.go        # CLI entry: subcommand dispatch (scan / proxy start)
internal/
├── types/                   # Shared types (zero dependencies)
│   ├── rule.go              #   Rule interface + severity constants
│   └── signal.go            #   Signal struct (JSON-tagged)
├── rules/                   # Rule implementations
│   ├── shell_exec.go        #   curl|sh, wget|bash, exec, spawn, child_process
│   ├── dynamic_eval.go      #   eval(), new Function()
│   ├── network.go           #   fetch, axios, XMLHttpRequest, URLs
│   ├── env_access.go        #   process.env, os.environ, getenv()
│   ├── obfuscation.go       #   base64, lines >500 chars
│   ├── postinstall.go       #   lifecycle hooks in JSON
│   └── registry.go          #   BuiltinRules() → []types.Rule
├── scanner/                 # Scanner orchestrator
│   ├── traversal.go         #   Walk(root): file discovery by extension
│   ├── pool.go              #   Pool.Run(files): concurrent rule application
│   └── aggregator.go        #   Aggregate(results): dedup + sort by severity + score
├── engine/                  # Decision engine
│   ├── decision.go          #   Classify(score) → "SAFE"/"WARNING"/"BLOCKED"
│   └── boosts.go            #   ApplyBoosts(): combination boost scoring
├── api/                     # API server (Phase 3)
│   ├── server.go            #   HTTP server, Config, routes
│   └── handlers.go          #   POST /scan, /scan-install, GET /report/{id}
├── report/                  # Report output + persistence
│   ├── report.go            #   Report struct, Save(), Load(), UUID v4
│   └── report_test.go       #   Unit tests for persistence
├── proxy/                   # Proxy server (Phase 2)
│   ├── server.go            #   Config, Server.New(), Start(), handler, serveHTTP
│   ├── tarball.go           #   isTarballURL(), isTarballContent(), packageNameFromURL()
│   ├── extract.go           #   ExtractTarball() with streaming + safety guards
│   ├── pipeline.go          #   RunScan(): Walk → Pool → Aggregate → Classify
│   ├── respond.go           #   writeAllowResponse(), writeBlockResponse()
│   └── log.go               #   LogIntercept() structured output
├── cache/                   # Tarball result caching (Phase 4)
│   └── cache.go             #   SHA256 hash, Check(), Store(), Prune()
├── config/                  # JSON config loader (Phase 4)
│   └── config.go            #   FileConfig struct, Load()
└── cli/                     # Terminal helpers (Phase 4)
    └── color.go             #   ANSI colors, IsTerminal(), FormatBlocked()
testdata/
├── safe-pkg/index.js        #   Clean fixture (score 0)
└── suspicious-pkg/evil.js   #   Suspicious fixture (score 110)
```

## Commands Reference

| Command | Purpose |
|---------|---------|
| `go build ./cmd/safeskill/` | Build CLI binary |
| `go run ./cmd/safeskill/ scan <path>` | Scan a directory for threats |
| `go run ./cmd/safeskill/ proxy start` | Start proxy server |
| `go run ./cmd/safeskill/ proxy start --port 9090 --upstream https://registry.yarnpkg.com` | Custom proxy config |
| `go run ./cmd/safeskill/ api start` | Start API server |
| `go run ./cmd/safeskill/ report <id>` | Fetch a saved scan report |
| `go run ./cmd/safeskill/ api start --port 9090 --reports-dir .safeskill/reports` | Custom API config |
| `go test -count=1 ./...` | Run all tests (no cache) |
| `go test -race -count=1 ./...` | Run all tests with race detector |
| `go test -bench=. -benchmem ./internal/proxy/` | Run proxy benchmarks |
| `go test -v ./internal/api/` | Run API tests with verbose |
| `go test -v ./internal/proxy/` | Run proxy tests with verbose |
| `go vet ./...` | Static analysis |

## Design Decisions

- **No external dependencies** — All phases use only Go stdlib. No cobra, no uuid, no CLI frameworks. Minimizes surface area.
- **Package `types` instead of `models`** — zero-import types package prevents circular dependencies. Every other package imports from types.
- **`flag` with subcommand dispatch** — `os.Args[1]` switch + per-command `flag.NewFlagSet`. Stdlib handles subcommand pattern cleanly without cobra.
- **Worker pool pattern** — fixed worker count (default 4), buffered channels, sync.WaitGroup. Matches PRD §6.1.
- **Tarball streaming** — resp.Body → gzip.Reader → tar.Reader → temp file. Never full tarball in memory.
- **Safety-first extraction** — zip-slip via `filepath.Rel` prefix check, depth limit 10, per-file 1MB, total 50MB, symlink escape guarded.
- **Proxy reuse of scanner** — proxy.handleTarball calls Walk → Pool → Aggregate → Classify unchanged. No duplicate scanning logic.
- **`regexp.MatchString` inline** — no pre-compilation. Simple, readable. Optimize in Phase 4 if needed.
- **Additive scoring** — sum of unique signal severities. Combination boosts (base64+eval, network+env, postinstall+exec) deferred to Phase 4.

## Key Artifacts

| File | What It Has |
|------|------------|
| `PRD.md` | Full product requirements, architecture, output formats |
| `PLAN.md` | Phased development roadmap (4 phases + future) |
| `temp-phase.md` | Atomic commit breakdown for all phases (gitignored) |
| `HANDOFF.md` | This session's handoff document (gitignored) |
| `AGENTS.md` | Working notes and context rules |
| `.codex/project-context.md` | This file — current project state |
| `.codex/skills/` | Loaded skills (caveman, handoff, code-review, vulnhunter, semgrep, security-review) |

## Phase 4 Checkpoint

Production-ready v1.0 baseline complete. All PLAN.md Phase 4 deliverables implemented.

## What's Next — v1.0 Release

Evaluate real-world feedback. Decide Phase 5 candidates:
- Remote rule updates from config URL
- Signature-based detection (known-malicious hash DB)
- Trust scoring (package reputation)
- Plugin system (Go plugins or WASM)
- Multi-ecosystem support (PyPI, RubyGems, Maven)

## Suggested Skills for Next Session

- **caveman** — token-efficient communication during development
- **code-review** — full project review before v1.0
- **vulnhunter** — audit rule patterns, identify detection gaps
- **semgrep** — static analysis on Go codebase
