# Task — Apply: finalize the reviewed RFC 0162 MVP build

Read the review **finding** and the `DRAFT.md`. The source changes already exist in
the run (draft made them); your job is to ensure the final state is correct and
publish the summary.

- If the review verdict was a clearing one (`accept` / `accept_with_findings`):
  address any non-blocking nits the reviewer raised (small fixes only), then confirm
  the tree still builds/vets/lints clean and the named tests pass. Do not expand
  scope (no Layer 2 / prober; proximal repo untouched).
- If a revision was already done (the review→draft cycle ran), simply finalize.

Before publishing, re-run:
`go build ./... && go vet ./... && golangci-lint run` and
`go test ./go/pkg/metrics/... ./go/pkg/laneproviderauth/... ./go/pkg/mutations/... ./go/pkg/reads/...`
and confirm `metrics_allowlist.json` + the boot hash are regenerated and consistent.

## Deliverable — `synthesis` with EXACT front matter

Publish **`docs/operator/artifacts/rfc-0162-build/SUMMARY.md`** (kind `synthesis`).
Front matter exactly:

```yaml
---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
inputs:
  - "docs/operator/artifacts/rfc-0162-build/DRAFT.md"
  - "docs/operator/artifacts/rfc-0162-build/review/REVIEW.md"
---
```

Body:
1. **Files changed** and a one-line role for each.
2. **Build-step discharge table** — one row per PROPOSAL step (1–5): the metric
   family / event / fold / code site (file:symbol) and the FA-* test that proves it.
3. **F1 / F2 folds** — exactly how each is honored (census `unless on(lane)`;
   resolver fail-closed `resolver_mismatch`).
4. **Boundary (FA-7)** — confirm the existing `laneproviderauth` /
   `supervision_provider_auth` suites pass UNCHANGED and the writes are best-effort.
5. **Verification** — `go build` / `vet` / `golangci-lint` / `go test` results; the
   regenerated allowlist hash; and the explicit note that the **live game-day
   (GD-1…GD-5) is the `rfc-0162-verify` run's job**, plus that the alert rules in
   `go/pkg/metrics/rules/alerting_rules.yml` await operator re-vendoring into
   `halbritt/proximal`.
6. **Scope** — confirm steps 1–5 only; Layer 2 / prober deferred; proximal untouched.

Do not weaken any test or the FA-7 boundary. Do not touch `.striatum/`.
