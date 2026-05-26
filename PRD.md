
# Product Requirements Document (PRD)

## Component: SafeSkill CLI (Go Proxy + Scanner Engine)

---

# 1\. Overview

SafeSkill CLI is a **local, high-performance security gateway** that:

- intercepts package/skill installations (via proxy)

- performs **concurrent static + heuristic analysis**

- enforces **allow / warn / block decisions**

- exposes **agent-friendly APIs**

- generates structured reports for external systems

It is designed to be:

- **offline-first**

- **low-latency**

- **extensible**

- **agent-integrable**

---

# 2\. Core Responsibilities

### Mandatory

- run local proxy server

- intercept npm install traffic

- detect tarball downloads

- extract + scan package

- compute risk score

- enforce decision (allow/block)

- print CLI output

### Optional (but included)

- expose local API

- generate structured report

- optionally send report to backend

---

# 3\. Non-Goals

- UI rendering

- cloud-based scanning dependency

- runtime sandboxing

- deep deobfuscation engines

---

# 4\. System Architecture

```text
CLI Command → Proxy Server → Scanner Engine → Decision Engine → Output + Report
```

---

## Components

### 4.1 CLI Layer

- command parsing

- process lifecycle management

### 4.2 Proxy Server

- HTTP server

- request interception

- tarball detection

- forwarding logic

### 4.3 Scanner Engine

- file traversal

- rule execution (concurrent)

- signal aggregation

### 4.4 Rule Engine

- modular rule interface

- pluggable detection rules

### 4.5 Decision Engine

- scoring system

- classification (safe/warn/block)

### 4.6 Report Engine

- structured JSON output

- report\_id generation

### 4.7 Local API Layer

- expose scan results

- support agent integration

---

# 5\. Tech Stack (optimized for speed + control)

## Language

- Go (primary)

## Core Libraries

- `net/http` → proxy + API

- `archive/tar`, `compress/gzip` → tar extraction

- `regexp` → rule matching

- `sync` → concurrency (worker pools)

## Optional

- `cobra` → CLI

- `uuid` → report\_id

---

# 6\. Performance Design

## 6.1 Concurrency Model

Worker pool pattern:

```text
Files → Jobs Channel → Workers → Results Channel → Aggregator
```

- fixed worker count (4–8)

- avoids uncontrolled goroutines

- parallel rule evaluation per file

---

## 6.2 I/O Optimization

- stream tarball (avoid full memory load)

- temp directory extraction

- limit file size for scanning

---

## 6.3 Early Exit Strategy

- if high-risk signal detected early:

    - short-circuit scoring (optional)

    - still generate minimal report

---

## 6.4 Caching (optional, later)

- hash tarball

- skip rescanning known packages

---

# 7\. Scanner Design

---

## 7.1 Rule Interface

```go
type Rule interface {
    Name() string
    Check(content string) (bool, string)
    Severity() int
}
```

---

## 7.2 Rule Categories

### Execution Rules

- curl | sh

- wget | bash

- exec / spawn

---

### Dynamic Execution

- eval()

- new Function()

---

### Network Behavior

- fetch / axios / http

- external domains

---

### Obfuscation

- base64 patterns

- high entropy strings

- long unreadable lines

---

### Package Metadata

- postinstall scripts

- lifecycle hooks

---

### Environment Access

- process.env usage

---

## 7.3 Signal Aggregation

Each rule produces:

```json
{
  "rule": "ShellExec",
  "message": "Uses curl | sh",
  "severity": 50
}
```

---

# 8\. Decision Engine

---

## Scoring Model

- additive scoring

- rule-based weights

Example:

| Signal | Score |
| --- | --- |
| curl | sh |
| eval | +30 |
| base64 | +20 |
| postinstall | +40 |

---

## Classification

```text
0–30 → SAFE
30–70 → WARNING
70+ → BLOCKED
```

---

## Combination Boost

- base64 + eval → +30

- network + env → +25

- postinstall + exec → +40

---

# 9\. Proxy Behavior

---

## Interception Flow

```text
Request → Forward → Response → Detect Tarball → Scan → Decide → Respond
```

---

## Block Behavior

- return HTTP 403

- include reason message

---

## Allow Behavior

- forward original response

---

# 10\. CLI UX

---

## Output Format

### Safe

```text
SAFE (Risk: 12)
Installed successfully
```

### Warning

```text
WARNING (Risk: 45)
- external network usage
```

### Blocked

```text
BLOCKED (Risk: 85)
- executes remote shell

Options:
[1] Force install
[2] View report
[3] See alternatives
```

---

# 11\. Report System

---

## Structure

```json
{
  "report_id": "abc123",
  "risk": 85,
  "status": "BLOCKED",
  "signals": [...],
  "summary": "Executes remote shell"
}
```

---

## Storage Options

- in-memory (MVP)

- file-based (temp)

- optional backend push

---

# 12\. Agent Integration

---

## Local API Endpoints

```http
POST /scan
POST /scan-install
GET /report/{id}
```

---

## Response Format

```json
{
  "risk": "HIGH",
  "action": "BLOCK",
  "reason": [...]
}
```

---

## Design Goals

- deterministic output

- machine-readable

- no UI dependency

---

# 13\. Commands

---

## Start Proxy

```bash
safe-skill proxy start
```

---

## Optional

```bash
safe-skill scan <path|repo>
safe-skill report <id>
```

---

# 14\. Security Considerations

- do not execute scanned code

- restrict file access to extracted directory

- sanitize inputs

- limit tar extraction depth (zip-slip protection)

---

# 15\. Future Extensions

- rule updates via config

- signature-based detection

- trust scoring

- plugin system for rules

---

# 16\. Key Design Principles

- local-first enforcement

- deterministic scanning (no black-box AI)

- modular rule engine

- fast feedback (<1–2s target)

- agent-compatible outputs

---

# Final Summary

SafeSkill CLI is:

- a **local, concurrent scanning engine**

- a **proxy-based enforcement layer**

- an **agent-integrable decision system**

It prioritizes:

- speed

- clarity

- extensibility

- real-world usability

---