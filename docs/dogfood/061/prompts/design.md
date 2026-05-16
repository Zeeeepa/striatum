# Design task — RFC 0051 V1 auto-finalize from frontmatter

Produce `docs/dogfood/061/design/<lane>/DESIGN.md` (front-matter
`handoff.v1` — author line `author: designer-<lane>-<model>-001`)
answering the seven design questions in
[`roles/designer.md`](../roles/designer.md). Use the locked-choice
discipline: one concrete answer per question, no menus.

## Required inputs to read first

- [`docs/rfcs/0051-auto-finalize-from-frontmatter.md`](../../../rfcs/0051-auto-finalize-from-frontmatter.md)
  — full RFC body.
- [`docs/rfcs/0046-lane-attestation-and-on-behalf-publish.md`](../../../rfcs/0046-lane-attestation-and-on-behalf-publish.md)
  — the RFC 0046 V1 operator-on-behalf path that V1 preserves on
  the refusal branch.
- `src/striatum/daemon_pg/handlers/workflow_loop/{complete_job,ack_work,submit_review}.py`
  — the V1.5-exported `*_inline` helpers you'll compose from.
- `src/striatum/daemon_pg/handlers/context.py::RepoHandlerContext` —
  the transaction primitive (`conn.transaction()` Schema v6 anchored).
- `src/striatum/recovery/watch.py` + `src/striatum/recovery/auto.py`
  — existing periodic ticks; you may colocate or split out.
- `docs/dogfood/056/OPERATOR_REPORT.md` — the 8-on-behalf-publish
  pattern this RFC closes.

## Deliverable shape

DESIGN.md sections:

1. **Reconciliation hook location** (module + function signature).
2. **Per-session scan sequence** (numbered list of guards in
   order — first failure short-circuits).
3. **Atomic finalize sequence** (the exact internal calls + their
   order, inside one `conn.transaction()`).
4. **Event payload shapes** (concrete JSON shape for each of
   `artifact.auto_finalized` and `job.auto_finalized`).
5. **Feature flag plumbing** (env-var read, hook-entry check, no-op
   path).
6. **Refusal paths table** (condition → fall-through behavior +
   blocker preserved or not).
7. **Acceptance tests** (4 test names with fixture paths).
8. **Anti-race details** (mtime > 10s; what if the agent rewrites
   the artifact mid-scan).

## Bouncing conditions reviewers will use

- A menu instead of a locked choice for any of 1-7.
- Inotify or any platform-specific filesystem watcher.
- Auto-finalize that touches missing or malformed artifacts.
- Bypass of capability_token authorization.
- Cross-job auto-finalization.

## Write scope

`docs/dogfood/061/design/<your lane>/DESIGN.md`. Nothing outside that
path.
