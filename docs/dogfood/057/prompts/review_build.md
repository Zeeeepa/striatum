# Build Review Prompt: RFC 0048 Phase A handler port

Your work packet specifies the review posture (`threat_model`, `ergonomics_dx`, or adversarial `threat_model`) and the artifact path under `docs/dogfood/057/review/build/<lane>/REVIEW.md`. Front matter:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
inputs: ["docs/dogfood/057/build/track_a/HANDOFF.md", "docs/dogfood/057/build/track_b/HANDOFF.md", "docs/dogfood/057/DESIGN_SYNTHESIS.md"]
review_posture: "<copy from work packet>"
verdict: "<accept | accept_with_findings | needs_revision>"
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `reviewer-unknown-model-<NN>`.

Fresh-session: read the two HANDOFF.md files, the synthesis, and the source code under `src/striatum/daemon_pg/handlers/` + `src/striatum/daemon_rpc/server.py`. Do NOT read implementer chat transcripts or other reviewers' findings.

## Cross-posture mandatory checks

1. All 16 Phase A methods have a handler at the synthesis-locked path? (9 Track A + 7 Track B.)
2. Each handler has a test file at the synthesis-locked path?
3. `DaemonRpcRouter._route` actually routes those 16 method names to the new handlers (grep the code; don't trust the HANDOFF)?
4. Audit chain stays unbroken — find a failing test scenario or accept that one was run and pass.
5. Tests assert byte-equivalence vs SQLite-backed equivalent, not just "no exception"?

Any "no" → degrade verdict.

## Posture-specific scope

### threat_model (codex)

- Append-only events grant retained on the new handlers' insert paths (no UPDATE/DELETE leak).
- Audit chain unbroken under concurrent inserts (row-level lock or SERIAL TX documented).
- Capability-token check enforced BEFORE the first PG write in each handler.
- No SQLite fallback silently engages when a PG handler raises (test it).
- A malformed envelope reaches `_route` and is rejected before any handler runs.

### ergonomics_dx (claude)

- Handler error messages cite operator-actionable next commands.
- Byte-equivalence tests fail loudly with a diff on regression (no `assert state_a == state_b` without per-key diff).
- Delegation-swap pattern is greppable.
- `striatum doctor` or equivalent indicates which methods are PG-backed vs SQLite-backed.
- `docs/POSTGRES_TRANSITION.md` updated to reflect the new substrate path.

### adversarial (gemini)

- A method that "looks ported" but quietly falls back to SQLite for a specific input (try empty params, null repository_id, unknown method, replayed request_id).
- Audit-chain forge: insert into `striatumd.events` without correct `prev_hash` chaining — is it caught?
- A handler that bypasses capability auth (e.g., admin token leaks past the gate).
- A recovery handler that creates orphaned PG rows (no parent run/job).
- Transaction split-write: state row commits but audit event does not (or vice versa).
- A Phase A handler that silently reads/writes rows outside its `repository_id` scope.
- Tests that pass for the wrong reason (both SQLite and PG produce the same empty result on bad input).

## Output

Per-finding entries. Each finding has:

- Severity: HIGH / MEDIUM / LOW.
- Evidence (file:line, test path, or reproducer).
- Required follow-up (specific patch suggestion).

Verdict:

- `accept` — no HIGH findings; mandatory cross-posture checks pass.
- `accept_with_findings` — mandatory checks pass; MEDIUM/LOW findings recorded as follow-ups.
- `needs_revision` — any HIGH finding, or any mandatory cross-posture check fails.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `reviewer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
