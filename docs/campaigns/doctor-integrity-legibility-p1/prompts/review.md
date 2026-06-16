# Review — Doctor integrity legibility P1

Fresh-eyes review of the draft
(`docs/campaigns/doctor-integrity-legibility-p1/artifacts/DRAFT.md`) **and the
committed implementation on the run branch**. Do NOT take the draft's PASS claims
on trust — verify independently from the committed branch state.

## Verify

- `make -C go build` clean; CI lint clean: `golangci-lint run --default=none
  --enable=govet --enable=staticcheck --enable=errcheck --enable=ineffassign
  ./pkg/reads/...` ⇒ `0 issues.`
- The three rules are implemented and correctly ordered (tip → **history** →
  terminal-debris → **superseded** → **acknowledged** → genuine-loss problem):
  1. **Rule A** default-branch *history* preservation → clean (matches a
     historical, not only tip, revision of the path); bounded + memoized +
     `ctx`-cancellable + safe-degrading.
  2. **Rule B** `artifact_superseded_on_default_branch` → warning when the path is
     live on the default tip with different content.
  3. **Rule C** `artifact_acknowledged_loss` → warning ONLY for a baseline entry
     whose `artifact_id` AND `content_sha256` both match the row.
- **Load-bearing safety (the verdict's hinge):** a genuine-loss artifact (path
  absent from the default branch, content on no ref, history empty) that is NOT in
  the ack baseline MUST stay a `problem` (still reds `ok`). Confirm a test proves
  this, AND that an id-match/sha-mismatch baseline entry does NOT downgrade. The
  fix must not blind doctor.
- The ack-file reader safe-degrades: absent / unparseable file → empty set, no
  panic, no spurious downgrades. The live baseline file is NOT authored by the
  lane (operator curates it post-merge); confirm the dogfood did not fabricate it.
- No `"main"` hardcode (reuses `resolveDefaultRefCached`); no schema/migration/RPC;
  changes confined to read-only `go/pkg/reads/`.
- The decision-log entry (D205) exists, references D204 + the "Do not paste over a
  broken runner" guardrail, and notes the operator-curated baseline.
- Run the new/affected tests yourself (PG-gated via `STRIATUM_PG_TEST_URL`, scoped
  `-run <Names> -p 1 -timeout 900s`, `-v` to confirm they executed); reproduce the
  load-bearing safety tests.

## Record

Publish a `finding` artifact at the declared path
(`.../artifacts/review/REVIEW.md`) with valid `striatum.finding.v1` front matter
(`schema_version`, `artifact_kind: finding`, `verdict_intent`, `severity`,
`tags`) and a verdict. Return **`needs_revision`** if genuine-loss detection
regresses, if the ack downgrade is not sha-bound, if any rule is mis-ordered, if
the default branch is hardcoded, if the lane fabricated the live baseline file, or
if the safety tests are missing. Otherwise `accept` / `accept_with_findings` with
non-blocking notes carried to follow-ups.
