# Coordinator Role (Dogfood 060 — RFC 0048 Phase C read-surface)

You keep the operator-driven dogfood-060 moving. 9 jobs total, **single
implement track** (the 058 lesson: dual-track creates the boundary
conflicts that caused cycle exhaustion). Shape:

1. **3 designs** — codex, claude, gemini in parallel. Each covers the
   full 8-method read-surface port; synthesis reconciles.
2. **1 synthesis** — codex locks per-method file paths, return-shape
   parity contracts, test paths.
3. **1 design review** — claude `ergonomics_dx` gates implement.
   `max_iterations: 1` cycle (one revision attempt only — do not allow
   the 058 pattern of three synth attempts on the same finding).
4. **1 implementer** — codex single track. Sub-agents per cluster:
   core-reads (status/why/doctor/dashboard), reporting-reads (list.*),
   summary-reads (run.summary/evidence.export/corpus.export).
5. **3-way build review** — codex `threat_model`, claude
   `ergonomics_dx`, gemini adversarial `threat_model`.

After build review, the operator runs the version bump
(v1.51.0 → v1.52.0), CHANGELOG, ROADMAP §4.2, RFC 0048 status update,
merge to main, tag, push.

**Scope is the 8 read methods listed in the synthesis**. Designers may
propose dropping a method (with justification) or adding one (e.g., a
deprecated-but-still-used read verb). Implementers stay within the
synthesis list.

**No dual-track**: synthesis MUST lock single track. Reviewers MUST
bounce a dual-track synthesis. The 058 fix-up dogfood hit cycle
exhaustion on dual-track boundary conflicts; 060 inherits the lesson.

**Operating mode**: legacy SQLite via `STRIATUM_DAEMON_REQUIRED=0
STRIATUM_TEST_HARNESS=1` for the run itself. The implement output is
what makes future runs Postgres-clean. Record this in OPERATOR_REPORT.

**Post-landing acceptance**:
1. `striatum daemon migrate-repo-local` on a fresh checkout succeeds.
2. `striatum status --json` post-migration (no env-var escape) returns
   the same top-level keys as pre-migration.
3. `striatum dashboard --once`, `striatum list runs --json`,
   `striatum run summary --run-id <id>` all return real data
   post-migration.
4. The V1.5 follow-up dogfood (for the deferred F2/F3/F4/HIGH#1/#2
   items) becomes unblocked.
