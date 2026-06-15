# Review — Doctor integrity legibility (P0)

Fresh-eyes review of the draft
(`docs/campaigns/doctor-integrity-legibility/artifacts/DRAFT.md`) **and the
committed implementation on the run branch**. Do NOT take the draft's PASS
claims on trust — verify independently from the committed branch state.

## Verify

- `make -C go build` clean; CI lint clean: `golangci-lint run --default=none
  --enable=govet --enable=staticcheck --enable=errcheck --enable=ineffassign
  ./...` ⇒ `0 issues.`
- The three reclassification rules are implemented and correct:
  1. preserved-on-default-branch → warning, not an `ok`-reddening problem;
  2. terminal-run (`canceled`) debris → warning;
  3. legacy empty-`blob_key` → `artifact_legacy_unverifiable` warning, not
     `artifact_blob_metadata_missing`.
- **Load-bearing safety:** an artifact/worktree whose content is on NO durable ref
  AND NOT on the default branch MUST still be a `problem` (still reds `ok`).
  Confirm a test proves this. The fix must not blind doctor — this is the verdict's
  hinge.
- Default-branch resolution does NOT hardcode `"main"` and degrades safely when
  unresolvable (falls back to prior behavior; never crashes/hangs the check).
- No schema/migration; changes confined to read-only `go/pkg/reads/`; no
  owner-table DDL.
- The decision-log entry exists and matches the `AGENTS.md` "Do not paste over a
  broken runner" guardrail intent.
- Run the new/affected tests yourself (PG-gated via `STRIATUM_PG_TEST_URL`, scoped
  `-run <names> -p 1 -timeout 900s`); reproduce the load-bearing safety test.

## Record

Publish a `finding` artifact at the declared path
(`.../artifacts/review/REVIEW.md`) with valid `striatum.finding.v1` front matter
(`schema_version`, `artifact_kind: finding`, `verdict_intent`, `severity`, `tags`)
and a verdict. Return **`needs_revision`** if genuine-loss detection regresses,
if `ok` is weakened beyond the three rules, if the default branch is hardcoded, or
if the safety test is missing. Otherwise `accept` / `accept_with_findings` with
any non-blocking notes carried to follow-ups.
