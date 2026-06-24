# Task — Apply: finalize the reviewed RFC 0142 P4 build

Read the review **finding** and the `DRAFT.md`. The source changes already exist in
the run (draft made them); your job is to ensure the final state is correct and
publish the summary.

- If the review verdict was a clearing one (`accept` / `accept_with_findings`):
  address any non-blocking nits the reviewer raised (small fixes only), then confirm
  the tree still builds/vets/lints clean and the named tests pass. Do not expand
  scope (no P5 / rehearsal / fidelity tiering; no `LatestOwnerBundleVersion` advance;
  no live activation of 0021).
- If a revision was already done (the review→draft cycle ran), simply finalize.

Before publishing, re-run from `go/`:
```
go build ./... && go vet ./... && golangci-lint run
go test ./pkg/db/... ./pkg/reads/... ./pkg/pgtest/... ./cmd/striatumd/...
```
and confirm the tree is clean.

## Deliverable — `synthesis` with EXACT front matter

Publish **`docs/operator/artifacts/rfc-0142-p4-build/SUMMARY.md`** (kind `synthesis`).
Front matter exactly:

```yaml
---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
inputs:
  - "docs/operator/artifacts/rfc-0142-p4-build/DRAFT.md"
  - "docs/operator/artifacts/rfc-0142-p4-build/review/REVIEW.md"
---
```

Body:
1. **Files changed** and a one-line role for each (new files and modified files).
2. **Build-step discharge table** — one row per §6 step (1–7): the schema/type/function
   added, the code site (file:symbol), and the named F-/G- test(s) that prove it.
3. **B1.1/B1.2 discharge** — how `T-deploy-bootpath-decision-table` constructs the
   row-16 in-sync AND out-of-sync sub-cases for all three `applied_owner` buckets
   (`==0`, `==20`, `>=21`) with spy assertions, and that each concrete cursor-state
   enum value is table-driven.
4. **§6.5 acceptance criteria table** — one row per criterion (a)-(l): the named test
   and the code site that satisfies it.
5. **Shadow-first invariants** — confirm: `STRIATUM_DEPLOY_DECOUPLED` default OFF;
   `LatestOwnerBundleVersion` STAYS 20; `RequiredOwnerBundleVersion` STAYS 20;
   migration 0044 strictly additive runtime-owned; 0021 excluded from all apply routes;
   fresh `applied_owner == 0` bootstrap still serves; existing serve-boot suites pass
   unchanged.
6. **Verification** — `go build`/`vet`/`golangci-lint`/`go test` results; explicit note
   that the **live game-day (§6.5 (a)-(l) against a real two-role cluster) is the
   `rfc-0142-p4-verify` run's job**; operator choreography for 0021 activation is
   deferred (§4.3).
7. **Scope** — confirm P5 (rehearsal/fidelity tiering/full-data clone/expand-contract)
   is deferred; no `LatestOwnerBundleVersion` advance; no live 0021 activation.

Do not weaken any test or any shadow-first invariant. Do not touch `.striatum/` or
`docs/operator/workflows/**`.
