# SafeSkill CLI — Phased Development Plan

## Overview

SafeSkill CLI is a local, high-performance security gateway that intercepts package/skill installations via proxy, performs concurrent static + heuristic analysis, enforces allow/warn/block decisions, exposes agent-friendly APIs, and generates structured reports.

4 product phases + 1 future extension phase. Each phase is independently testable and shippable. Only Go stdlib.

---

## Phase 1: Core Scanner — Standalone CLI

**Goal:** Scan any local package directory. Get risk score + report. No proxy.

| Deliverable | Detail |
|-------------|--------|
| Go project bootstrap | `go mod init safeskill`, package layout |
| Rule interface | `type Rule interface { Name() string; Check(content string) (bool, string); Severity() int }` |
| Built-in rules | All PRD categories: ShellExec, DynamicEval, NetworkAccess, Obfuscation, PostinstallHook, EnvAccess |
| Worker pool concurrency | Files → jobs chan → N workers → results chan → aggregator (4–8 workers) |
| File traversal | Walk extracted dir, filter by extension/size |
| Signal aggregation | Collect per-rule `{rule, message, severity}` signals |
| Decision engine | Additive scoring: safe (0–30), warn (30–70), block (70+) |
| CLI command | `safe-skill scan <path>` |
| JSON report output | `{report_id, risk, status, signals, summary}` stdout or file |
| Unit + integration tests | Per-rule unit tests, fixtures for known-bad packages |

**Testable:** `safe-skill scan ./testdata/suspicious-pkg` → risk score + signal list.

**Deliverable:** Standalone scanner binary. Offline. No server.

---

## Phase 2: Proxy Layer — Intercept + Enforce

**Goal:** Hook real install flow. Proxy intercepts, scans, decides.

| Deliverable | Detail |
|-------------|--------|
| HTTP proxy server | `net/http` reverse proxy, configurable port + upstream registry |
| Tarball detection | Inspect Content-Type + URL patterns for `.tgz` |
| Streaming extraction | Stream tarball to temp dir, avoid full memory load |
| Scan pipeline | Download → extract → scan → decide → respond |
| Block behavior | HTTP 403 with JSON `{reason, signals}` body |
| Allow behavior | Forward original response unmodified |
| CLI command | `safe-skill proxy start [--port] [--upstream] [--threshold]` |
| Structured logging | Per-intercepted-package log lines |

**Testable:** `npm config set proxy http://localhost:8080` → `npm install some-pkg` → intercept log + verdict.

**Deliverable:** Working proxy security gate. Real npm traffic intercepted.

> **Discussion checkpoint:** Review rule effectiveness with real data. Tune thresholds, weights, reduce false positives.

---

## Phase 3: Local API + Agent Integration

**Goal:** Machine-readable API. Agents query decisions programmatically.

| Deliverable | Detail |
|-------------|--------|
| HTTP API server | Separate port from proxy (e.g. 9090) |
| `POST /scan` | Submit package path → scan result JSON |
| `POST /scan-install` | Submit from proxy flow → async scan + decision |
| `GET /report/{id}` | Fetch past scan report by UUID |
| Report persistence | JSON files on disk (`.safeskill/reports/`) |
| Report ID | UUID v4 per scan |
| CLI command | `safe-skill report <id>` |
| Structured response | `{risk, action, signals, summary, report_id}` — deterministic JSON |

**Testable:** `curl -X POST http://localhost:9090/scan -d '{"path":"./pkg"}'` → structured verdict.

**Deliverable:** Agent-integrable security API. Machine-first, no UI.

> **Discussion checkpoint:** Full system functional. Ship as MVP or continue to Phase 4? What agent integrations needed most?

---

## Phase 4: Hardening + Production Polish

**Goal:** Fast, secure, configurable, polished. Production-ready v1.0.

| Deliverable | Detail |
|-------------|--------|
| Combination boost scoring | base64+eval = +30, network+env = +25, postinstall+exec = +40 |
| Early exit short-circuit | High-risk signal → stop scanning, still generate report |
| Caching | Hash tarball (SHA256), skip rescan, configurable TTL |
| Security hardening | Zip-slip protection, depth limits, file size limits, input sanitization |
| Config file support | YAML/TOML config for rules, thresholds, proxy settings, cache |
| CLI UX polish | Colors, interactive block prompt (force install / view report / alternatives) |
| Performance benchmarks | Benchmarks per phase, optimize hot paths |
| Comprehensive test suite | Integration tests, edge cases, security boundary tests |
| README + docs | Install guide, agent integration examples, rule authoring |

**Testable:** Full workflow — configure proxy, `npm install`, see cached rescan skip, polished CLI output.

**Deliverable:** Production-ready v1.0.

> **Discussion checkpoint:** Product complete. Evaluate user feedback, real-world performance, rule quality. Decide Phase 5.

---

## Phase 5 (Future): Extensions

*Prioritized after Phase 4 based on real feedback.*

| Candidate | Detail |
|-----------|--------|
| Remote rule updates | Fetch updated rules from config URL |
| Signature detection | Known-malicious hash DB |
| Trust scoring | Package reputation over time |
| Plugin system | External rules via Go plugins or WASM |
| Backend push | Optional report upload to central service |
| Multi-ecosystem | Plugins for PyPI, RubyGems, Maven (not just npm) |

---

## Architecture Flow

```
CLI Command
  → Proxy Server (intercept tarball)
    → Scanner Engine (extract + file traversal + concurrent rules)
      → Decision Engine (score + classify)
        → Report Engine (JSON output)
          → API Layer (agent queries)

Standalone mode: CLI → Scanner → Decision → Report (no proxy)
```

## Key Principles

- Local-first enforcement
- Deterministic scanning (no black-box AI)
- Modular rule engine
- Fast feedback (<1–2s target)
- Agent-compatible outputs
- Offline capable
