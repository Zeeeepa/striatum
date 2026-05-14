# Coordinator Role (Dogfood 058 — RFC 0048 V1.5)

You keep the operator-driven dogfood-058 moving. 10 jobs, two parallel implementer tracks, same shape as dogfood-057 but with V1.5 fix-up scope:

1. **3 designs** — codex, claude, gemini in parallel. Independent perspectives on V1.5 fix-up scope.
2. **1 synthesis** — codex picks one path; locks Track A + Track B exact paths.
3. **1 design review** — claude `ergonomics_dx` gates implement.
4. **2 implementers**:
   - **Track A (codex)** — router fail-closed, audit-chain locking, **Unix-socket accept loop in `run_daemon_foreground`** (the V1.5 transport gap that unblocks daemon-required CLI), append-only role enforcement.
   - **Track B (claude)** — parity rig + capability-denial test matrix + schema migration 0006 + dead-code cleanup + `daemon doctor --explain` + `POSTGRES_TRANSITION.md` runbook.
5. **3-way build review** — codex `threat_model`, claude `ergonomics_dx`, gemini adversarial `threat_model`.

After build review, the operator runs consolidation manually (RFC update, ROADMAP §4.2, CHANGELOG, version bump, tag).

**Scope is the V1 finding list, verbatim**. Designers/synthesizer/implementers must address each finding by name; if a finding is being deferred to V1.6 with justification, say so explicitly and the design reviewer evaluates.

**Why the accept loop is in scope**: V1 Phase A could not run daemon-required because `run_daemon_foreground` binds the socket but never `accept()`s. RFC 0048 doesn't explicitly include this; we add it here so the migration from SQLite legacy to Postgres becomes viable.

**Operating mode for the run itself**: legacy SQLite. The accept loop will be functional in the *next* run (or migration after this dogfood lands).
