# Build Review Prompt: RFC 0048 V1.5 fix-up

Your work packet specifies the review posture (`threat_model`, `ergonomics_dx`, adversarial `threat_model`). Produce REVIEW.md at the packet's path. Front matter:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
inputs: ["docs/dogfood/058/build/track_a/HANDOFF.md", "docs/dogfood/058/build/track_b/HANDOFF.md", "docs/dogfood/058/DESIGN_SYNTHESIS.md"]
review_posture: "<copy from work packet>"
verdict: "<accept | accept_with_findings | needs_revision>"
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `reviewer-unknown-model-<NN>`.

Fresh-session: read the two HANDOFFs + synthesis + source code under `src/striatum/daemon.py`, `daemon_rpc/`, `daemon_pg/handlers/`, `daemon_pg/sql/`, `tests/daemon_*`, and `docs/POSTGRES_TRANSITION.md`. Do NOT read implementer chat transcripts or sibling reviewers' findings.

## Cross-posture mandatory checks

1. **F1 fail-closed routing actually fails closed**: there's a test that monkeypatches a PG handler to raise and asserts no SQLite read/write. Run it mentally — does it cover the right call sites?
2. **F2 capability-denial matrix is complete**: 16 handlers × 6 denial cases = 96 test cases. Spot-check 3.
3. **F3 audit-chain locking actually serializes**: concurrent test produces a verifiable unbroken chain. Walk the assertion.
4. **F4 role-enforcement REVOKEs are in SQL AND tested**: grants in 0007 (or amendment); test asserts `permission denied` on UPDATE/DELETE attempts.
5. **HIGH#1 parity rig runs by default**: env-gate removed; per-key diff helper exists and prints actual vs expected on failure.
6. **HIGH#2 dead code resolved**: each of `complete_inline` / `ack_inline` / `recovery.resume --complete` / `recovery.auto` is either defined+wired+tested OR deleted+callers-removed. No orphan imports.
7. **Schema migration 0006 idempotent and byte-equivalent**: existing rows re-anchored from `payload_json._event_chain`; test fixture confirms.
8. **Accept loop end-to-end**: real CLI client process calls real daemon via Unix socket; response comes back. Not mocked.
9. **`daemon doctor --explain`**: lists the 16 PG-backed methods correctly.
10. **`POSTGRES_TRANSITION.md`**: role-provisioning runbook is copy-pasteable; cites the doctor refusal.

Any "no" → degrade verdict.

## Posture-specific scope

### threat_model (codex)

- Append-only events grant retained on new handlers' insert paths.
- Audit chain unbroken under concurrent inserts (verifiable, not aspirational).
- Capability-token check enforced BEFORE the first PG write in each handler.
- Fail-closed: no SQLite fallback engages when a PG handler raises.
- Accept loop rejects malformed envelopes before any handler runs.
- Replay protection: duplicate `request_id` is caught.

### ergonomics_dx (claude)

- Parity diff helper output is readable (per-key, not raw dict).
- `daemon doctor --explain` table is operator-actionable.
- `POSTGRES_TRANSITION.md` works on Ubuntu/macOS (no platform-specific assumptions).
- Capability-denial tests run by default (no env-gate).
- v1.49.0 → v1.50.0 path is clean: `daemon doctor --apply-migrations` brings schema to 6 without manual SQL.

### adversarial (gemini)

- A PG handler raises mid-transaction → chain breaks: is it caught? Test it.
- Replay attack against fail-closed routing: duplicate `request_id` accepted by accept-loop path.
- Accept loop accepts an envelope from a non-authorized client.
- Audit-chain SERIALIZABLE deadlocks under concurrent load; retries silently corrupt the chain.
- Schema migration 0006 leaves orphan rows when `payload_json._event_chain` is malformed.
- Capability-denial test passes for the wrong reason (e.g., same response on missing-token AND invalid-token).
- A non-PG-backed CLI method routes incorrectly through the new accept loop.

## Output

Per-finding entries: severity (HIGH/MEDIUM/LOW), evidence (file:line, test path, or reproducer), required follow-up.

Verdict:
- `accept` — no HIGH findings; cross-posture mandatory checks pass.
- `accept_with_findings` — mandatory pass; MEDIUM/LOW recorded as follow-ups.
- `needs_revision` — any HIGH or mandatory failure.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `reviewer-unknown-model-<NN>`.
