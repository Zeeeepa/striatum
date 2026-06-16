---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept_with_findings
---

# Evidence Audit
author: evidence-auditor-codex-via-live-striatum-mcp-001

## Verdict

accept_with_findings

`RESET_SYNTHESIS.md` is broadly supported by `SUPPORT_LEDGER.md`. The ledger
covers all major synthesis claims with explicit claim IDs, citations,
confidence, and notes, and it already marks the weaker portions as partial or
needing rerun. The synthesis is acceptable if those caveats stay visible and the
release-facing claims are not acted on without the named reruns.

Scope note: the packet prompt named `RESET_SYNTHESIS.md` and
`SUPPORT_LEDGER.md` as the audit targets. The packet `inputs` field was null, so
this review audited those two artifact bodies and daemon registry metadata
needed to fetch them; it did not independently reopen every cited source file or
upstream finding body.

## Unsupported or weakly supported claims

- `RESET_SYNTHESIS.md:36`, `RESET_SYNTHESIS.md:148`, and
  `RESET_SYNTHESIS.md:169-171` correctly identify escalation-driven completion
  as a P0 lock-path risk, but `SUPPORT_LEDGER.md:17` says the lane verified the
  invariant and state docs, not the named implementation. This is supported as a
  state/invariant risk, not yet as a source-confirmed implementation diagnosis.
- `RESET_SYNTHESIS.md:38`, `RESET_SYNTHESIS.md:161-164`, and
  `RESET_SYNTHESIS.md:185-187` rely on a prior audit's command summary for the
  red `TestSpawnRunAsSpecResolvesLaneUser` gate. `SUPPORT_LEDGER.md:21` is clear
  that the ledger lane did not rerun `make typecheck` or `make test`; the claim
  is good enough to block release action, but not enough to describe current
  tree health.
- `RESET_SYNTHESIS.md:127-133` groups stale compatibility aliases and obsolete
  recovery service surfaces with better-supported demotion and warning-cleanup
  work. `SUPPORT_LEDGER.md:30` marks that deletion claim as partial and based on
  high-level state-doc support. Keep it conditional until current client
  emissions and replacement paths are verified.
- `RESET_SYNTHESIS.md:159-181` presents a useful two-week operating sequence,
  but `SUPPORT_LEDGER.md:32` correctly frames the exact day allocation as
  planner judgment. It should remain a sequencing proposal, not an evidence
  claim that the work fits exactly in fourteen days.

## Evidence that contradicts the synthesis

No direct contradiction appears between the synthesis and the support ledger.
The ledger mostly supports the synthesis and explicitly limits the same claims
that need caution: escalation implementation verification (`SUPPORT_LEDGER.md:17`),
local gate freshness (`SUPPORT_LEDGER.md:21`), conditional alias/service deletion
(`SUPPORT_LEDGER.md:30`), and schedule precision (`SUPPORT_LEDGER.md:32`).

The synthesis also preserves the major support boundaries reflected in the
ledger: keep the core architecture (`RESET_SYNTHESIS.md:18-29`,
`SUPPORT_LEDGER.md:13-14`), demote overbroad divergent-ideation support rather
than deleting the generator (`RESET_SYNTHESIS.md:72-97`,
`SUPPORT_LEDGER.md:18-20`), and treat doctor warning normalization as the risk
rather than the D204/D205 split itself (`RESET_SYNTHESIS.md:45-68`,
`SUPPORT_LEDGER.md:24-26`).

## Claims that need command reruns before acting

- Before any release decision, rerun `make typecheck` and `make test` or record a
  bounded quarantine for `go/pkg/mutations/spawn_grant_test.go::TestSpawnRunAsSpecResolvesLaneUser`
  (`RESET_SYNTHESIS.md:161-164`, `RESET_SYNTHESIS.md:185-187`,
  `SUPPORT_LEDGER.md:21`).
- Before treating the two-week reset as validated, rerun the full gate named by
  the synthesis: `make lint`, `make typecheck`, `make test`, affected workflow
  validation, and `striatum doctor --verbose --json` from an authorized operator
  context (`RESET_SYNTHESIS.md:175-181`, `RESET_SYNTHESIS.md:185-200`).
- Before deleting stale aliases or obsolete recovery surfaces, verify current
  clients no longer emit the old methods and that the P0 recovery fixes are
  green (`RESET_SYNTHESIS.md:132`, `SUPPORT_LEDGER.md:30`).
- Before implementing the escalation fix as a source change, reopen the relevant
  completion redrive implementation and add the lock-first/race tests named by
  the synthesis (`RESET_SYNTHESIS.md:169-171`, `RESET_SYNTHESIS.md:192-193`,
  `SUPPORT_LEDGER.md:17`).

## Minimal edits required for accept

1. Add one caveat near the failure taxonomy that SL005 is state-doc and
   invariant supported, but still needs direct source verification before code
   routing.
2. Keep the red local gate wording tied to the prior audit summary unless the
   commands are rerun in the active lane.
3. Preserve the conditional wording for stale alias and obsolete recovery-surface
   deletion; do not promote it to an immediate delete instruction.
4. Label the two-week plan as a proposed operating sequence and keep the
   definition of done as the actual release gate.
