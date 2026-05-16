# Designer Role (Dogfood 061 — RFC 0051 V1 auto-finalize)

author: designer-role-001

You design the runner-side auto-finalize feature per
[RFC 0051](../../../rfcs/0051-auto-finalize-from-frontmatter.md).
Three designer lanes (codex / claude_code / gemini) work in parallel
and produce one DESIGN.md each. Synthesis picks one concrete plan.

## What you must answer

Each of these gets ONE answer, not a menu:

1. **Reconciliation hook location.** Where does the scan run?
   Candidates: `src/striatum/recovery/watch.py`,
   `src/striatum/recovery/auto.py`, new `src/striatum/auto_finalize.py`,
   or inline in `work.heartbeat`. Justify with reference to existing
   tick cadence + dependency surface.
2. **Per-session scan sequence.** For each session in
   `claimed`/`running` with a healthy lease, what's the exact step
   list? (resolve expected_artifacts → exists → mtime > 10s →
   open + parse frontmatter → schema validate per `kind` → byline
   equals `expected_author_line` → derive verdict).
3. **Atomic finalize sequence.** Inside one `conn.transaction()`:
   the exact internal calls (use the V1.5 helpers `complete_inline`,
   `ack_inline`, `publish_artifact_pg`). Order matters: ack →
   publish → verdict (review jobs only) → complete.
4. **Two new event types.** `artifact.auto_finalized` and
   `job.auto_finalized` payload shapes (RFC 0051 §Audit markers
   is the locked contract). Include the
   `lane_finalization=auto_from_artifact` marker.
5. **Feature flag.** Where does the `STRIATUM_AUTO_FINALIZE_ENABLE`
   check live? (Hook entry, not buried in publish.)
6. **Refusal paths.** Enumerate the disqualifying conditions and
   what happens on each (fall through to existing lane-stall, with
   the original blocker hint preserved).
7. **Acceptance tests.** Name the 4 test functions per RFC
   §Acceptance with their fixture paths. Be concrete.

## Anti-patterns reviewers will bounce on

- Menu of options for any of (1)-(7) above.
- Inotify or other platform-specific filesystem watcher (V1 polls).
- Auto-finalize for missing/malformed artifacts (V1 keeps RFC 0046
  override path; runner only ever lifts agent stalls on **valid**
  on-disk artifacts).
- Bypassing capability authorization (auto-finalize uses an internal
  capability path that's still audit-chained — flag this concern).
- Cross-job auto-finalization (only the lane's own expected
  artifact is in scope per RFC §Non-goals).

## Write scope

Your own design lane only (`docs/dogfood/061/design/<lane>/`).
