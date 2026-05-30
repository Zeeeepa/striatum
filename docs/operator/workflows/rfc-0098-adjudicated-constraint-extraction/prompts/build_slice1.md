# Build RFC 0098 slice 1 (from the vetted design)

Task: Implement **RFC 0098 slice 1** exactly as specified in the
already-converged, panel-vetted design synthesis at
`docs/operator/workflows/rfc-0098-adjudicated-constraint-extraction/artifacts/DESIGN_SYNTHESIS.md`
(committed). The interrogating design panel already vetted this design; build to
it, do not redesign.

## Scope (slice 1 only)

1. **`collaboration_ledger.v1.1`** — extend the existing
   `striatum.collaboration_ledger.v1` front-matter schema **additively** in
   `go/pkg/artifactcontracts`: add an optional `constraints[]` table (rows with
   `id`, `source_finding`, `posture` (non-empty string, NOT a closed enum),
   `severity`, `kind`, `binding`, `text`, `verification`) and an optional
   `branches{}` posture-disposition map and `cycle`. Every existing v1 ledger
   MUST still validate (additive only).
2. **Productive-refusal gate** — in `validateCollaborationLedger`
   (`go/pkg/artifactcontracts/contracts.go:~550`, the single function all three
   write paths funnel through): when `verdict == needs_revision`, require a
   non-empty `constraints[]` (binding constraints or unresolved-question rows);
   reject otherwise with the existing `artifact_error` (CLI exit 6). Do NOT add a
   new error code or daemon method.
3. **Do NOT widen the front-matter `verdict` enum.** Keep it exactly
   `accept | accept_with_findings | needs_revision | reject` — the design proved
   that adding `blocked_pending_answer`/`defer_with_successor` as verdicts wedges
   `recordVerdict` (`review.go`) with `invalid_transition`. Those two states live
   as `branches{}` dispositions only.
4. Accept the clearing verbs the contract advertises and natural ledger front
   matter (the #88 / #79 fixes for this shape), per the synthesis.

## Constraints

- Keep the change **additive**; add NO new daemon method or route (the
  command-authority matrix + guardrail tests stay green).
- Add/extend tests: a v1-ledger-still-validates canary, a `needs_revision` with
  empty `constraints[]` → rejected, a `needs_revision` with a binding constraint
  → accepted, and the #88/#79 acceptance cases.
- Stay inside `write_scope.allowed_paths`; never write `.striatum/`.
- Run `STRIATUM_PG_TEST_URL=postgres:///postgres go -C go test ./pkg/artifactcontracts/... ./pkg/mutations/...`
  and `make -C go vet` before handing off.

## Handoff

Write `docs/operator/workflows/rfc-0098-adjudicated-constraint-extraction/artifacts/build/HANDOFF.md`
with what landed, the exact files touched, and the verification commands you ran
with their results. Then emit the `submit-handoff` packet from your work packet.
