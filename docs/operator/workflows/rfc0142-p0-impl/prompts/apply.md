# Task — Apply: finalize the reviewed P0 change

Read the review **finding** and the `DRAFT.md`. The source changes already exist in
the run (draft made them); your job is to ensure the final state is correct and
publish the summary.

- If the review verdict was a clearing one (`accept` / `accept_with_findings`):
  address any non-blocking nits the reviewer raised (small fixes only), confirm the
  code still `go build`s / `go vet`s clean, and leave the P0 change ready for
  verification. Do not expand scope.
- If a revision was already done (the review→draft cycle ran), simply finalize.

## Deliverable — `synthesis` with EXACT front matter

Publish **`docs/operator/artifacts/cc_rfc0142_p0/SUMMARY.md`** (kind `synthesis`).
Front matter exactly:

```yaml
---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
inputs:
  - "docs/operator/artifacts/cc_rfc0142_p0/DRAFT.md"
  - "docs/operator/artifacts/cc_rfc0142_p0/review/REVIEW.md"
---
```

Body:
1. **Files changed** and a one-line role for each.
2. **C1–C5 discharge table** — one row per constraint: how the code discharges it
   (file:symbol) and which test/gate proves it.
3. **Red test + green control** — names and what they assert.
4. **Verification** — `go build` / `go vet` status; note the PG test needs a live
   two-role cluster (`STRIATUM_PG_TEST_URL`) and is skipped without it (real
   assertions retained), so the Stage 3 verifier covers build/vet, not the PG run.
5. **Boundary** — confirm test-harness + test code only; no migration/bundle/daemon
   change; no later-layer symbols.

Do not weaken any constraint or the tests. Do not touch `.striatum/`.
