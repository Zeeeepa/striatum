---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: adjudicator-reviewer-001
title: "RFC 0170 P0 design v5 final collaboration summary"
run_id: "run_76bfeaefa3976ddf516a6113ea38f207"
status: accept_with_findings
cycle: 1
inputs:
  - "docs/operator/workflows/rfc-0170-p0-design-v5/SEED.md"
  - "docs/operator/workflows/rfc-0170-p0-design-v5/context/HOLDER_v4.md"
  - "docs/operator/workflows/rfc-0170-p0-design-v5/context/LEDGER_v4_cycle_1.md"
  - "docs/rfcs/0170-self-culling-repository-and-cull-workflow-class.md"
  - "docs/operator/artifacts/rfc-0170-p0-design-v5/dialogue/holder/HOLDER.md"
  - "docs/operator/artifacts/rfc-0170-p0-design-v5/dialogue/falsifier_1/FALSIFIER.md"
  - "docs/operator/artifacts/rfc-0170-p0-design-v5/dialogue/falsifier_2/FALSIFIER.md"
  - "docs/operator/artifacts/rfc-0170-p0-design-v5/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md"
  - "docs/operator/artifacts/rfc-0170-p0-design-v5/commit/proposal/PROPOSAL.md"
---

# RFC 0170 P0 Design v5 Final Collaboration Summary

author: adjudicator-reviewer-001

## Verdict

verdict: accept_with_findings

The RFC 0170 P0 design v5 collaboration gate clears for downstream build. The
adjudicator ledger records `accept_with_findings`, judged against the re-scoped
P0 bar recorded as operator decision D271, not against exhaustive whole-tree
Tier-1 exactness.

The clearing condition is narrow and explicit: P0 is observe-only. It writes a
`cullable_entity` candidacy ledger that nothing acts on: no deletion, no
tombstone, no page, no doctor RED or amber, and no run-admission effect. Under
that bar, the dangerous failure direction is a false positive. The v5 gate pins
that direction to zero for the known preserved set, proves read-only safety for
the cull fold, carries G3/G4 unchanged, and records the two remaining seams as
P1 work.

## Gate Record

| stage | disposition | effect |
| --- | --- | --- |
| v4 ledger | `needs_revision` | The prior gate failed on two residuals: whole-tree Tier-1 exactness around `status:frozen` records outside `docs/records/_frozen/`, and the over-claimed non-cooperative filesystem hang bound. |
| v5 seed / D271 bar | re-scoped P0 | P0 now requires mechanical soundness plus the known-set corpus test with zero false positives, and read-only safety rather than adversarial cull-liveness. |
| v5 falsifier pass | no P0 blocker | Falsifier 1 and Falsifier 2 landed attacks, but both were rebutted under the re-scoped bar. Their surviving concerns are accepted P1 deferrals, not design blockers. |
| v5 adjudication | `accept_with_findings` | G1, G2, G3, and G4 all clear; #618 and #619 are accepted findings with build-carry obligations. |
| downstream proposal | published | `docs/operator/artifacts/rfc-0170-p0-design-v5/commit/proposal/PROPOSAL.md` is the consolidated SPEC for `rfc-0170-p0-build`. |

## What Cleared

G1 clears as Tier-1 soundness plus known-set exactness. The predicate is
mechanical and sound: fixed grep/link/row rules, fixed closure lexicon, and a
static tree-local protected pathspec, with no LLM and no network, clock,
open-issue, or other mutable outside-tree input. The known preserved set has zero
false positives: `rfc:0097`, `rfc:0027`, `rfc:0039`, `rfc:0041`, `D174`, the
banked `backup/rfc-*` branches, `docs/records/_frozen/**`, RFC 0170 prose, the
SEED, workflow files, and decision-log prose are not nominated. The known-dead
`decision:D267` is nominated end-to-end.

G2 clears as read-only safety. The `DecayTickSweep` fold is off the recovery
scheduler wait path, does only O(1) slot-check/spawn/skip work on the recovery
tick, and never joins the detached scan. The cursor refresh is not deferred, the
recovery loop is not blocked, panic/error isolation stays inside the detached
goroutine, and L4 compute-then-commit prevents torn writes by writing
`cullable_entity` rows only after a scan completes before `cullCtx` fires.

G3 substrate correctness clears unchanged. The build contract is a runtime
migration at `go/pkg/db/sql/0045_cullable_entity.sql`; both read and write
authority-inventory rows are required; `striatumd_rw` receives
`SELECT, INSERT, UPDATE`; owner DDL/FK remains forbidden for migrations `>=27`;
and all `cullable_entity` reads must use explicit column lists, never
`SELECT *`.

G4 forward-compatibility clears unchanged. P0 uses the sweep/peer writer, the
`(kind, ref)` UPSERT shape, and an extensible `candidacy_state` ledger without
building downstream deletion machinery. The proposal extends the P0/P1+ boundary
table with #618 and #619 while keeping tombstone, reaper, `cull_gate`,
counterforce, and Tiers 2-4 out of P0.

## Falsifier Disposition

Falsifier 1 attacked the G1 false-positive surface. The old `rfc:0097`
contradiction is rebutted by the carried clause-4 active-baseline rule: live
non-successor citations preserve `rfc:0097`, and the same rule preserves the
other named RFCs. The D081 audit-file challenge is real, but it is a safe
false-negative: D081 is genuinely dead and is conservatively withheld by a live
`status:frozen` audit citation outside `_frozen/`. It under-nominates cleanup; it
does not nominate a live artifact. The ledger records this as `landed_and_rebutted`.

Falsifier 2 attacked the G2 scheduler and stale-write surface. A synchronous fold
would still be a blocker, but v5 keeps the fold off the `wait` path, so the
persisted cursor refresh and the next recovery tick proceed on the no-cull
schedule. The non-cooperative filesystem hang is not falsely claimed to
self-terminate: in P0 it holds the cull slot until restart, every later tick
logged-skips, and the stuck scan never reaches the L4 write phase. That is a
restart-recoverable cull-liveness gap, not a safety break. The ledger records
this as `landed_and_rebutted`.

## Findings Carried Downstream

#618 is the accepted G1 whole-tree completeness deferral. P0 does not complete
exhaustive Tier-1 exactness for every tracked `status:frozen` provenance record.
Instead, P0 must ship the known-set table-driven corpus fixture: zero false
positives on the known preserved set, `decision:D267` nominated, and
`decision:D081` recorded as the documented withheld member with the audit
citation as its cause. P1 #618 completes the whole-tree exactness work by
extending the static frozen/protected source set to all tracked `status:frozen`
provenance, or by adding an equivalent tree-local clause-4c non-live rule, then
re-deriving D081 with zero counted hits.

#619 is the accepted G2 non-cooperative-filesystem-hang liveness deferral. P0
must preserve the detached O(1) fold, the refresh-not-deferred B5 assertion, and
the no-torn-write L4 rule. It must also add the late-return-zero-write guard
test: a scan that ignores `ctx.Done()` and returns after `DefaultCullFoldTimeout`
performs zero `cullable_entity` UPSERTs. P1 #619 adds cull-slot release plus a
late-writer generation or epoch fence, proven by a non-cooperative-hang test that
does not merely cooperate with `ctx.Done()`.

## Downstream Contract

`rfc-0170-p0-build` should implement the consolidated SPEC exactly as published
in `PROPOSAL.md`: the `cullable_entity` ledger, the read-only Tier-1
`DecayTickSweep`, the known-set corpus, the mechanical decision-successor rule,
the clause-4 active-baseline citation rule, the static tree-local protected
pathspec, and the bounded refresh-safe cull fold.

The named implementation proof surface is the proposal's assertion table:
A1/A1'/A2-A8 for the Tier-1 predicate and corpus, B1-B5 plus B4' for read-only
safety and hang behavior, C1-C3 and PC1/PC2 for the migration and authority
surface, and D1 for forward compatibility. The build must treat `BC-618` and
`BC-619` as required proof obligations even though the underlying issues are P1
deferrals.

P1+ remains explicitly downstream: `cull_tombstone`, doctor integrity for voided
or soak-expired receipts, `cull_gate`, two-phase tombstone, manual and timed
reap, soak window, resurrection-rate governor, blast-radius cap,
`accretion_ledger`, the run-start throttle, release clearing-house, and richer
Tiers 2-4 are named seams, not P0 deliverables.

## Closing Note

The v5 collaboration gate cleared because it stopped pretending P0 proves every
future cull property. It preserves the P0 safety invariant that matters now:
observe-only nomination can be mechanically explained, cannot nominate the known
preserved set, cannot block recovery, and cannot write partially. The remaining
gaps are visible, tracked, and carried into the correct downstream phase.
