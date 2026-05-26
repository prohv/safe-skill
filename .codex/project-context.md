# Project Context

## Repo Status

- Workspace: `C:\Users\shado\Windsurf\outskill-hack\safe-skill`
- Branch: `dev`
- Phase 1: **Complete** — 16 atomic commits, all tests passing
- Next: Phase 2 — Proxy Layer

## Product Brief

SafeSkill CLI = local Go-based proxy + scanner for secure package installs. Intercepts npm traffic via proxy, extracts tarballs, scans concurrently with static/heuristic rules, scores risk, enforces allow/warn/block, outputs JSON reports. Agent-friendly API planned (Phase 3). Pure Go, offline-first, deterministic.

## Phase 1 — Core Scanner (Standalone CLI)

**What's built:**
- 6 rule categories: ShellExec, DynamicEval, NetworkAccess, EnvAccess, Obfuscation, PostinstallHook
- Scanner engine: file traversal (Walk), concurrent worker pool (Pool), signal dedup+sort (Aggregate)
- Decision engine: additive scoring → SAFE (0-29) / WARNING (30-69) / BLOCKED (70+)
- Report engine: structured JSON output with report_id, signatures, signals, summary
- CLI: `safeskill scan <path>` via stdlib `flag` (`--workers`, `--output`)
- 42 unit tests across 6 rules + 8 boundary tests for classification + 2 integration scenarios

**Usage:**
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
cmd/safeskill/main.go        # CLI entry point: flag parsing, pipeline wiring
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
│   └── decision.go          #   Classify(score) → "SAFE"/"WARNING"/"BLOCKED"
└── report/                  # Report output
    └── report.go            #   Report struct → JSON() with indent
testdata/
├── safe-pkg/index.js        #   Clean fixture (score 0)
└── suspicious-pkg/evil.js   #   Suspicious fixture (score 110)
```

## Commands Reference

| Command | Purpose |
|---------|---------|
| `go build ./cmd/safeskill/` | Build CLI binary |
| `go run ./cmd/safeskill/ scan <path>` | Scan a directory for threats |
| `go test ./...` | Run all tests |
| `go test -v ./internal/rules/` | Run rule tests with verbose output |
| `go vet ./...` | Static analysis |
| `go build ./internal/rules/` | Build a specific internal package |

## Design Decisions

- **No external dependencies** — Phase 1 uses only Go stdlib. No cobra, no uuid, no CLI frameworks. Minimizes surface area.
- **Package `types` instead of `models`** — zero-import types package prevents circular dependencies. Every other package imports from types.
- **`flag` not cobra** — single command `scan` doesn't warrant a CLI framework. Migrate to cobra when Phase 2 adds `proxy start` subcommand.
- **Worker pool pattern** — fixed worker count (default 4), buffered channels, sync.WaitGroup. Matches PRD §6.1.
- **`regexp.MatchString` inline** — no pre-compilation. Simple, readable. Optimize in Phase 4 if needed.
- **Additive scoring** — sum of unique signal severities. Combination boosts (base64+eval, network+env, postinstall+exec) deferred to Phase 4.

## Key Artifacts

| File | What It Has |
|------|------------|
| `PRD.md` | Full product requirements, architecture, output formats |
| `PLAN.md` | Phased development roadmap (4 phases + future) |
| `temp-phase.md` | Atomic commit breakdown for Phase 1 (gitignored) |
| `HANDOFF.md` | This session's handoff document (gitignored) |
| `AGENTS.md` | Working notes and context rules |
| `.codex/project-context.md` | This file — current project state |
| `.codex/skills/` | Loaded skills (caveman, handoff, code-review, vulnhunter, semgrep, security-review) |

## What's Next — Phase 2: Proxy Server

From `PLAN.md`:
- HTTP reverse proxy (`net/http`), configurable port + upstream registry
- Tarball detection in npm traffic (Content-Type, URL patterns)
- Streaming extraction to temp dir (no full memory load)
- Scan pipeline: download → extract → scan → decide → respond
- Block behavior: HTTP 403 with JSON body
- Allow behavior: forward original response unmodified
- CLI: `safeskill proxy start [--port] [--upstream] [--threshold]`
- **Checkpoint:** Review rule effectiveness with real data after Phase 2

## Suggested Skills for Next Session

- **caveman** — token-efficient communication during development
- **code-review** — review Phase 1 Go code before starting Phase 2
- **vulnhunter** — audit rule patterns, identify detection gaps
- **semgrep** — static analysis on Go codebase
