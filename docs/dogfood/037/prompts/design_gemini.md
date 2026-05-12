# Gemini Design Prompt

Produce `docs/dogfood/037/design/gemini/DESIGN.md`.

Design an implementation plan for RFC 0035 with attention to cross-platform reality, CI matrix integration, wall-clock budget, determinism + cleanup hygiene, and the harness's exposed inspection helpers.

Your plan must cover:

**Cross-platform reality:**

- Linux + system PG: full coverage (existing CI job).
- macOS + system PG: full coverage (existing CI job).
- macOS no PG: skip with clear message.
- Windows: not supported (Windows daemon mode is deferred per RFC 0030 V2).
- The harness uses subprocess + Unix socket, so no port-collision concerns on any platform.

**CI matrix integration:**

- `make test-multi-repo` is the new make target that runs only harness-backed tests.
- The existing `make pg-test` recipe ensures local PG is available.
- The existing `make test` includes harness tests by default if PG is available; skips them with a clear message if not.
- CI's existing `daemon-pg` extras path (psycopg[binary]) is reused; no new optional dependency.

**Wall-clock budget:**

- Total added wall-clock for the new harness tests should be under 60 seconds on a developer laptop.
- Per-class fixture scope amortizes daemon startup across multiple tests in the class.
- `reset_daemon_db()` uses TRUNCATE rather than DROP+CREATE to keep per-test reset cheap.

**Determinism + cleanup hygiene:**

- Every harness instance gets its own ephemeral PG database (dropped on stop).
- Every harness instance gets its own scratch directory (rmtree on stop).
- Every harness instance gets its own Unix socket path under the scratch directory (deleted on stop).
- SIGTERM the daemon on stop; wait for clean exit; force-kill only after a timeout.
- Re-running the harness back-to-back in the same test session must work (no leftover sockets, no port conflicts).

**Subprocess + Unix socket means no port collision:**

- The daemon listens on the ephemeral Unix socket under the harness's scratch directory.
- No TCP port allocation, no race for an open port.
- Two harness instances in the same test session each have their own socket path; they cannot interfere.

**Exposed inspection helpers:**

- `harness.daemon_db_query(sql, args)` — return daemon Postgres rows for assertions (cross-repo run rows, audit rows, capability tokens, etc.).
- `harness.repo_sqlite_query(repo_index, sql, args)` — return per-repo SQLite rows for assertions (runs, jobs, leases, etc.).
- `harness.audit_rows(transport=None)` — return daemon audit chain rows in order, optionally filtered by transport (`chat`/`rpc`/etc.).
- `harness.mcp_client(token)` — return a small client with `tools_list()` and `tools_call(name, args)`; raises with structured error info on refusal.

**Adversarial cases to exercise in tests:**

- Hostile MCP client requesting `tools/list` to enumerate then `tools/call` with elevated args.
- Expired token replay attempt.
- Revoked token replay attempt.
- Scope mismatch (token scoped to repo A trying to write repo B).
- Operator-confirmation bypass (write call with the confirmation field missing).
- Audit chain tamper attempt via the daemon API (role-enforced append-only refuses).

**State which parts are cross-platform and which need platform-specific work:**

- All harness Python code is cross-platform (Python subprocess, pathlib, psycopg).
- The daemon binary's behavior is the same on Linux + macOS.
- chmod-based "one repo unreachable" simulation works on Linux + macOS; would need different simulation on Windows (Windows daemon is not in scope anyway).

**Explicitly deferred:**

- Go-client testing surface (RFC 0035 §Open Questions; D084 future).
- Two-repos-with-worktree-isolated-lanes example workflow under `examples/` (follow-up).
- Docker-based ephemeral Postgres (separate hardening RFC).
- Windows daemon harness.
- Cross-machine multi-tenant testing.
- Performance/load testing.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim.

- Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.
- Correct: `author: designer-gemini-pro-001`
- Wrong: `**Author:** designer-gemini-pro-001` (bolded variant)
- Wrong: `Author: designer-gemini-pro-001` (capital A)
- Wrong: `author: "designer-gemini-pro-001"` (quoted)

If you produce schema-bearing artifacts (synthesis, finding), the file must start with a JSON-encoded `key: <value>` front matter block. Example for `finding`:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0035"]
---
```

The byline appears AFTER the front matter block and a blank line, NOT inside it. `verdict_intent` (not `verdict`); `severity` from {low,medium,high,critical} (not `none`); `tags` as a JSON array.

**IMPORTANT — produce the artifact, do not surface strategy and exit.** Per the dogfood-036 OPERATOR_REPORT.md intervention #2, a previous gemini design session surfaced a strategy summary and asked the operator "should I proceed?" and exited without writing the file. Do not repeat that pattern. The work packet's `expected_artifacts` requires the file on disk; the operator is not on a back-and-forth chat with you — you are inside a supervised wrapper that runs `gemini --prompt -` once per packet, and there is no follow-up turn. Write the DESIGN.md file with byline + body in this single invocation.

Do not call striatum CLI; the operator publishes on your behalf otherwise.
