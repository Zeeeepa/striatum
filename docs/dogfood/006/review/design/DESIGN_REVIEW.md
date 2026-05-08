---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["dogfood-006", "rfc-0012"]
---

# RFC 0012 V1 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-08
Target: `docs/dogfood/006/DESIGN_SYNTHESIS.md`
Read (fresh, artifact_augmented):

- target synthesis;
- `docs/dogfood/006/research/SERVICE_SURFACE.md`;
- `docs/rfcs/0012-local-service-api.md`.

Verdict intent: **accept_with_findings**. The synthesis is
implementation-ready. Three improvement-grade refinements (F1–F3)
that the implementer should fold in.

## Boundary Compliance

- **D020 (no remote serving)** — non-loopback host refusal at
  startup with exit 8; loopback whitelist (`127.0.0.1`, `localhost`,
  `::1`); explicit `0.0.0.0` rejection. Test
  `test_serve_refuses_non_loopback_host` enforces.
- **D006 (api.invoke is the dispatch path)** — every endpoint
  except SSE goes through `api.invoke`. SSE reads the events table
  directly via a dedicated read connection; this is acceptable
  because SSE is a streaming view of immutable rows, not a state
  mutation.
- **D028 (no transcripts)** — request/response bodies are not
  logged. `test_serve_no_transcripts_logged` enforces.

## Mutation Whitelist Adequacy

The whitelist of read verbs (`status`, `why`, `doctor`, `list`,
`evidence`, plus subcommand-aware reads under `workflow`,
`supervise`, `worktree`, `run`) is conservative and forward-safe:
new mutating CLI commands default to blocked. The synthesis's
subcommand-aware refinement (when argv[0] is `workflow`/`supervise`/
`worktree`/`run`, inspect argv[1]) is the right shape.

## Test Plan Completeness

Twenty test cases covering: every endpoint shape, mutation gating
both ways, SSE replay via two channels, run-terminal end-of-stream,
non-loopback refusal, token auth, Unix-socket permissions,
single-instance enforcement, stale PID file recovery, graceful
shutdown, concurrent SSE clients, no-external-URL invariant,
no-transcripts invariant. A smoke test in
`tests/test_service_smoke.py` for end-to-end. Adequate.

## Findings

### F1 (low) — Token comparison must guard against length-based timing leak

**Issue.** Synthesis § 6 says `hmac.compare_digest`. That is correct
for equal-length inputs but `compare_digest` returns False
immediately on length mismatch, leaking the expected token's length
through wall-clock time. For a localhost-only token the leak is
small but not zero.

**Recommendation.** Pad both inputs to a fixed length before
comparison, or compare-and-discard via a constant-time wrapper:

```python
def _tokens_match(provided: str, expected: str) -> bool:
    p = provided.encode("utf-8")
    e = expected.encode("utf-8")
    target = max(len(p), len(e), 64)
    p_padded = p.ljust(target, b"\x00")
    e_padded = e.ljust(target, b"\x00")
    return hmac.compare_digest(p_padded, e_padded) and len(p) == len(e)
```

The accompanying test should send a same-length-but-wrong token and
a wrong-length token and assert both return 401 with no observable
timing difference (just functional, not statistical).

### F2 (low) — SSE poll holds an open SQLite connection for the stream's lifetime

**Issue.** The SSE handler opens a read connection at stream start
and polls every 250ms. With concurrent SSE clients (cap 32 per
synthesis), the service holds 32 file descriptors per run plus the
service's own write path. SQLite handles this but the implementer
should ensure the connection is closed promptly on disconnect (not
held until GC).

**Recommendation.** Use a `try/finally` that closes the read
connection on disconnect, on run-terminal, and on graceful
shutdown. Verify with `lsof` in the concurrent SSE test, or just
inspect that the handler's exit path closes the cursor.

### F3 (low) — `--web` flag accepted as a no-op needs explicit messaging

**Issue.** Synthesis § 11 says `--web` is accepted as a no-op in
V1 because RFC 0013 lands the static assets. If an operator passes
`--web` and gets a silent 404 on `/`, that's confusing.

**Recommendation.** When `--web` is set, log a warning at startup:
"--web flag accepted but the web UI is not yet bundled (RFC 0013
not implemented); / will return 404." This way the operator knows
exactly what they're getting.

## Acceptance Recommendation

**accept_with_findings.** Design is implementation-ready. F1–F3
are refinements the implementer should fold in during the build.
A human can reasonably record an acceptance decision and proceed.
