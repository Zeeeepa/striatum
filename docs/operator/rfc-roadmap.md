# RFC Roadmap — sequenced, themed, "do the next one"

Living doc. Last triaged: 2026-06-23. Owner: operator. This orders every RFC
that is **proposed, accepted-but-unbuilt, or partially implemented** into a
single execution sequence. Items not listed are `accepted / implemented`
(shipped) or superseded/closed-out.

When an item ships, mark it ✅ and move the wave boundary down; the sequence
numbers are stable so "do the next one" always resolves to the lowest-numbered
unshipped item whose blocker is clear.

---

## How an item ships: Design → Build → Verify

Every roadmap item passes through **three Striatum workflows**, in order, before
it reaches `main` and a deploy. Do not hand-implement an RFC; drive it through
the runner so the provenance is real (AGENTS.md: "anything needing an RFC also
gets a Striatum workflow").

1. **Design workflow — harden the spec into falsifiable acceptance criteria.**
   Scaffold a `falsification_gate` (or `implementation_panel` / `committee`)
   design-run seeded with the RFC. A holder lane proposes the spec; independent
   cross-model falsifier lanes attack it; an independent adjudicator ratifies it
   into *binding, verification-gated constraints*. Output: an accepted design
   with acceptance criteria + a recorded decision (`D###`). **Skip only when the
   RFC is already `accepted` with concrete criteria** (the "Design" cell below
   says `done` for those — go straight to Build).

2. **Build workflow — implement the ratified design in reviewed slices.**
   Scaffold a `code_change` run (draft → review → apply) per slice. The author
   lane builds; an independent reviewer lane returns `accept_with_findings` or
   `needs_revision`; the daemon integrates the accepted slice onto the run
   branch. One slice = one tracer-bullet vertical cut. The design's acceptance
   criteria are the reviewer's checklist.

3. **Verify workflow — mint an independent sealed receipt, then ship.**
   `striatum verifier run` mints sealed `go-build` / `go-vet` / `go-test`
   receipts (RFC 0134/0141) — the **second key**; the executing agent may not be
   its own verifier. Land to `main` (daemon run-integration for lane work, or a
   sync-guarded direct commit for operator-class fixes), confirm **CI green**,
   then deploy on the **next quiescent daemon restart** (never restart while
   design dogfoods are live — fixes are ancestors of `main` and auto-apply).

### "Do the next one" — operator protocol

1. `striatum operator bootstrap --markdown` (cold start; follow `next_actions`).
2. Open this file. Find the lowest-numbered item **not** marked ✅ whose
   **Blocked-by** is satisfied. That is "the next one."
3. Run **Design** (unless `Design: done`) → **Build** → **Verify** for it.
4. On ship: mark it ✅ here, update its tracking issue, note the deploy in
   `docs/operator/BRIEF.md`, and bump the "Last triaged" date.
5. Respect **in-flight** items (🔵) — a design dogfood is already running for
   those; monitor/continue it rather than starting a second run for the same RFC.

### Themes

- 🛡 **Reliability** — keeps self-hosting runs alive, correct, and recoverable.
  These break live dogfoods today, so they lead the sequence.
- ✨ **Feature** — new product surface / capability. Sequenced after the
  reliability spine is solid.

---

## The sequence

### Wave 0 — In flight (finish what's running; do **not** start a second run)

| # | RFC | Theme | What it is | Stage | Track |
|---|---|---|---|---|---|
| 1 | **0142 P4** | 🛡 | One-shot `striatum daemon deploy` — make it the only schema mutator; lift auto-apply out of serve-boot; revoke serving-role DDL | 🔵 design-run live (`rfc-0142-p4-design-v6`) | #571 |
| 2 | **0143** | 🛡 | Lane credential survival across a daemon boot-epoch rotation (reseal without the owner-only client-token) | 🔵 design-run live (`rfc-0143-design-v6`) | #512 |
| 3 | **0165** | 🛡 | Claude provider-cred freshness + spawn-time hydration (supervisor side; complements the host cred-resync timer) | 🔵 design v2 quarantined → needs v3 | #583 |

### Wave 1 — Stop the bleeding (lane-health reliability that wedges live runs)

| # | RFC | Theme | What it is | Design | Blocked-by | Track |
|---|---|---|---|---|---|---|
| 4 | **0166** | 🛡 | Completion deadline for an alive-but-never-completing lane (sealed-progress silence budget) | drafted; needs RFC_REVIEW | — | #576 |
| 5 | **0162 + #569** | 🛡 | Lane auth silent-failure observability — detect absence-of-success; finish the detection layers + a live game-day | done (MVP shipped) | — | #569 |
| 6 | **0133** | 🛡 | Fan-in deferred-join barrier cutover — wire `recordFaninFreezePoint`, flip `STRIATUM_BARRIER_FANIN`, retire `fanInIntegrateRunBranch` | done | equivalence run | #527 |

### Wave 2 — Deployment-safety chain (build on Wave 0's P4)

| # | RFC | Theme | What it is | Design | Blocked-by | Track |
|---|---|---|---|---|---|---|
| 7 | **0142 P3-arm** | 🛡 | Arm schema-drift refuse-to-serve (flip `STRIATUM_SCHEMA_DRIFT_REFUSE`) after a clean prod bake | done | one clean prod deploy cycle + P4 (#1) | #578 |
| 8 | **0142 P5** | 🛡 | Rehearsal receipt + expand/contract on an ephemeral two-role clone (highest-risk owner DDL) | done (D258 scope) | P4 (#1) | #572 |
| 9 | **0136** | 🛡 | Range-partition `events`/`audit_log` by `created_at`; partition `DROP` as the retention path | needs design | P5 (#8) | #387 |

### Wave 3 — Hardening tail (correctness + security)

| # | RFC | Theme | What it is | Design | Blocked-by | Track |
|---|---|---|---|---|---|---|
| 10 | **0158** | 🛡 | `verified_stale` staleness rung + `verifier resweep --builtins` (needs a sealed version basis + migration) | done (D252); migration sub-decision open | — | #577 |
| 11 | **0164** | 🛡 | Untrusted-substrate hardening — read-side git neutralization + gate-evidence recovery contract | needs design | — | — |
| 12 | **0095** | 🛡 | Revision-safe lifecycle — remaining phases past 1–3 | done (per-phase) | — | — |
| 13 | **0100** | 🛡 | Self-describing artifact contracts — phases past 1 (packet + error ergonomics) | done | — | — |
| 14 | **0113** | 🛡 | Runtime read-scope least-privilege remainder (mostly carried by accepted 0114; confirm residual) | done | re-confirm vs 0114 | — |

### Wave 4 — Features (once the reliability spine is solid)

| # | RFC | Theme | What it is | Design | Blocked-by | Track |
|---|---|---|---|---|---|---|
| 15 | **0099** | ✨ | Constrained operator mode — control-surface-only AI operator; phases past 1–2 | done | — | — |
| 16 | **0163** | ✨ | Staged-not-adopted offline self-improvement — nightly consolidation that can never ship an unreviewed change | needs design + product decision | — | — |
| 17 | **0052** | ✨ | Committee deliberation workflow shape (arbitration, panels, adversarial review) | needs scheduling + design | — | #403 |
| 18 | **0094** | ✨ | Deferred collaboration shapes — fog-of-war review, synaptic prune, adjudicator reliability | done (slices 1–3); remainder | — | — |
| 19 | **0115** | ✨ | Precise token-usage telemetry for supervised lanes | done | dashboard-ingest landing | #404 |
| 20 | **0067** | ✨ | Optional git + PR integration | — | **product decision first** | — |

---

## Notes & rationale

- **Why reliability leads:** Waves 0–1 are the RFCs that wedge live self-hosting
  runs today (stuck lanes, dead creds, never-completing reviewers, fan-in
  correctness). Every feature in Wave 4 depends on a dogfood loop that doesn't
  stall, so they come last.
- **The two dependency chains:** (a) *deployment safety* — 0142 P4 → P3-arm →
  P5 → 0136 (the big DB reshape only rehearses safely once P5's ephemeral-clone
  rehearsal exists); (b) *lane-health/credential* — 0143, 0162, 0165, 0166, all
  tracing to self-hosting friction.
- **Already done, not on the sequence (optional residual only):** 0042, 0061,
  0062, 0066, 0069, 0070, 0098, 0102, 0119 (runtime evictor deferred), 0130.
- **Closed-out (do not pick up):** superseded/deprecated 0027, 0028, 0039, 0041,
  0049, 0097.
- This roadmap is a snapshot; re-triage when a wave empties or a new RFC lands.
  The authoritative per-RFC status is each file's `Status:` line under
  `docs/rfcs/`.
