# Task — Review: does the build correctly implement the RFC 0162 MVP?

You are a **fresh-session reviewer**. Read `SEED.md`,
`docs/operator/artifacts/rfc-0162-design/commit/proposal/PROPOSAL.md` (the contract),
the cycle-2 ledger (F1/F2), the upstream **`DRAFT.md`**, and the **actual source
diff** the draft produced (inspect the changed files in the worktree:
`go/pkg/metrics/`, `go/pkg/laneproviderauth/`, `go/pkg/mutations/`, `go/pkg/reads/`,
`go/pkg/metrics/rules/`). Review the **CODE**, not just the handoff.

## Check, concretely (against the PROPOSAL contract)

1. **Metric surface.** Do all eight MVP families exist in `DefaultRegistry()` with
   the **exact names + closed label sets** from PROPOSAL §"Exact metric surface", all
   `ClassificationOperational`? Is `metrics_allowlist.json` + the boot hash
   regenerated and consistent (no drift)? No raw id/path/sha/branch/prompt/byline on
   any label?
2. **F1 fold.** Is the census rule `…expected unless on(lane) (…sample_present == 1)`
   — **preserving the `lane` label** (no aggregate-only `count(...) < scalar`)? Is a
   healthy `api_key` lane covered by `sample_present` without being forced to emit an
   expiry series, and never paged while healthy? Is the scalar `expected_count`
   retired?
3. **F2 fold.** Does the resolver **fail closed** into `lane_cred_resolver_mismatch`
   when the runtime source is unproven (never a green gauge)? Does it read the
   **launch-env-resolved** credential, not a fresher `HOME` decoy? Is
   `credential_path_template` only a drift cross-check?
4. **Layer 3 (FA-5).** Is `lane.auth_success` emitted strictly at
   `supervision_provider_auth.go:56` (`result.Passed()`), **never** on the
   `supported==false` early return, so the heartbeat is codex-scoped and no synthetic
   heartbeat exists for claude/agy/gemini?
5. **Boundary (FA-7).** Do the existing `laneproviderauth` and
   `supervision_provider_auth` suites pass **UNCHANGED**? Is `Check()` still a pure
   classifier (no DB handle)? Are the writes best-effort/error-swallowing so a write
   failure cannot flip a gate decision or alter a timeout? No preflight/timeout/trust
   change?
6. **Tests + build.** Are all named FA-* tests present and asserting the *right*
   thing (not weakened to pass)? Does the tree `go build` / `go vet` /
   `golangci-lint` clean, and do
   `go test ./go/pkg/metrics/... ./go/pkg/laneproviderauth/... ./go/pkg/mutations/... ./go/pkg/reads/...`
   pass? Is the guardrail `rules_test.go` green (rules reference only registered
   families)? Is the diff minimal and idiomatic (reuses `applySeriesBudget`, the
   event-fold pattern, the `doctor_problems{class}` bounded-label precedent)?
7. **Scope.** Steps 1–5 only; Layer 2 / the prober is NOT built (follow-up). Alert
   rules in `go/pkg/metrics/rules/alerting_rules.yml` only — the proximal repo is
   untouched.

## Verdict

Record a `finding` (the verdict path). Use:
- **`needs_revision`** if any of: a family/label deviates from the contract or the
  allowlist drifts; the census is aggregate-only or drops the `lane` label; the
  resolver can read green from a decoy/fallback; a synthetic non-codex heartbeat
  exists; an existing suite changes verdict (FA-7 breach); a named test is missing /
  fabricated / weakened; or it won't build/vet/lint/test. List each defect precisely
  (one revision cycle is available).
- **`accept`** / **`accept_with_findings`** if the MVP matches the contract, the
  F1/F2 folds and FA-7 boundary hold, and the named tests pass (note minor nits as
  findings).

Write only your review finding artifact at the declared path. Do not modify source.
