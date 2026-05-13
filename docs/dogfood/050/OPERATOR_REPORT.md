# Dogfood-050 Operator Report

**Run:** `run_59807e8872024bafa1a42e3f36f50a4a`
**Branch:** `striatum/dogfood-050-rfc-0043-v1-5`
**Scope:** RFC 0043 V1.5 — close D102 follow-up findings (F-crash, F-escape, F-parser, F-test).

Implementer is **claude** (deliberately not codex — 5th codex/codex
anti-pattern risk flagged in D102).

## Interventions

### Intervention 1: Kickoff
- Scaffold workflow (9 jobs: 3 designs → synth → design review → 1 impl claude → 3-way build review).
- 3 designer sessions launched (codex/claude/gemini), supervisors attached.

### Intervention 2: Design synth + design review
- All 3 designers shipped naturally.
- Synth + design review (claude) completed; design verdict accept_with_findings.

### Intervention 3: Implementer publish-on-behalf
- Claude impl session (sess_b7956ea87866425991c171e804f9a706) shipped 12.5KB
  HANDOFF.md at `docs/dogfood/050/build/HANDOFF.md` but stalled before
  publish.
- Operator did ack + publish-artifact + complete (artifact
  art_3d80b6b7584f427683a43f8e417a2a9f).
- **4th claude-no-explicit-publish instance** — pattern is now: claude
  writes the file, stalls, operator finalizes. Folded into harness backlog.

### Intervention 4: 3-way build review with double override
- Registered 3 fresh reviewer sessions (codex/claude/gemini), supervised,
  claim-next.
- Codex (threat_model): **needs_revision (high)** — STRIATUM_DAEMON_REQUIRED=0
  documented as operator migration path bypasses daemon-required boundary
  (verdict_350316ea339a4d13a805f9a3c172eb27 → waiting_human, no cycle).
- Claude (ergonomics_dx): natural submit blocked (4th instance) — operator
  submit-on-behalf using on-disk artifact, verdict accept_with_findings (low).
- Gemini (adversarial): wrote review on-disk with verdict_intent=needs_revision
  flagging A1 critical (server-side substrate mismatch — daemon delegates to
  SQLite-backed CLI logic), A2 high (split-brain via fresh-DB creation),
  A3 medium (concurrent migrate locking). Operator submit-on-behalf →
  waiting_human checkpoint.

### Intervention 5: Verdict overrides
- Codex override → accept_with_findings (verdict_010fe0a1d6534358ad2c42e0eb52565b).
  Rationale: env-var-bypass finding is real but V1.5 deliberately retains
  it as transition affordance; V1.6 will remove.
- Gemini override → accept_with_findings (verdict_5e7bf515fae14734b0bb7fafc8f2d747).
  Rationale: A1/A2/A3 reveal substrate flip is incomplete at daemon RPC
  layer (delegation still routes through SQLite-backed CLI); requires
  daemon business-logic migration, which is V1.6 scope.
- 9/9 jobs completed, run state `completed`.

## Run Outcome

- Run state `completed`. 9/9 jobs, 0 canceled.
- v1.38.0: RFC 0043 V1.5 — F-crash transactional rollback + checkpointed
  resume + sentinel; F-parser migrate-repo-local subcommand wired with
  argparse + dispatch; F-test exit-code-12 e2e regression; F-escape
  default-flip (STRIATUM_DAEMON_REQUIRED unset = enforce).

## Anti-patterns observed

- **claude-no-explicit-publish (4th instance)** — claude reviewer + implementer
  wrote artifacts on disk but did not call publish/complete CLI verbs.
  Operator submit-on-behalf workflow now feels routine; harness should
  detect on-disk artifacts and auto-publish on stale lease.
- **gemini-no-frontmatter** did NOT recur this run — both gemini designer
  and reviewer wrote schema-conformant front matter. Possibly the new
  skills bundle helped.
- **codex-reviewer-on-claude-implementer** — codex reviewer found a real
  high-severity threat issue against claude's impl. This is the
  *productive* form: distinct lane review keeps tightening the loop
  without same-lane echo chamber. Not an anti-pattern.

## V1.6 Follow-ups (to schedule)

1. **Remove STRIATUM_DAEMON_REQUIRED=0 runtime escape** — keep only as
   test-only or rip entirely. Codex finding owner.
2. **Daemon-side substrate migration (gemini A1)** — port single-repo
   business logic from SQLite-backed CLI dispatch to PG-backed
   daemon-internal logic. This is the actual V1 → V2 substrate flip
   completion. Material scope: maybe a separate phase RFC.
3. **Split-brain protection (gemini A2)** — `striatum.db.connect` must
   refuse to create a fresh SQLite when a migration checkpoint exists.
4. **Concurrent migrate-repo-local locking (gemini A3)** — exclusive
   lock on source SQLite during migration.
5. **Per-flag help text on migrate-repo-local (claude F-dx-1)** —
   ergonomics polish on argparse subcommand.

## Follow-ups absorbed into harness

- Auto-publish on stale-lease when reviewer/implementer wrote on-disk
  artifact (covers the 4-instance claude pattern).
