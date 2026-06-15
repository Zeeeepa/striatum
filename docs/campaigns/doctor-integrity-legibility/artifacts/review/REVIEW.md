---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept_with_findings
severity: info
tags:
  - doctor
  - integrity
  - reads
  - legibility
---

# Review — Doctor integrity legibility (P0)

author: reviewer-agent-001

Verdict: accept_with_findings

Fresh-eyes review of `artifacts/DRAFT.md` **and the committed implementation on
`striatum/doctor-integrity-legibility-p0`** (`f719ff1a`, diffed against the
scaffold base `0d5021f3`). I did not take the draft's PASS claims on trust: I
built, vetted, and ran the tests myself from a throwaway worktree checked out to
the run branch, and read every reclassification branch and helper against the
prompt's verification checklist.

## Verdict rationale

All four `needs_revision` triggers are clear:

1. genuine-loss detection does **not** regress (3 load-bearing safety tests pass,
   confirmed by reading the code path, not just the test names);
2. `ok` is **not** weakened beyond the three rules (`ok = len(problems)==0` is
   unchanged; only un-actionable items move to an additive `warnings` channel);
3. the default branch is **not** hardcoded; and
4. the load-bearing safety tests are present and exercised.

So the verdict is **accept**, recorded as `accept_with_findings` to carry three
non-blocking follow-ups (§ Non-blocking findings). None blocks the apply.

## What I verified (independently, from the run branch)

| Check | Command (worktree on `f719ff1a`) | Result |
| --- | --- | --- |
| Build | `make -C go build` | **PASS** — `striatum`, `striatumd`, `striatum-supervisor-helper` all built, exit 0. |
| Vet | `go vet ./pkg/reads/` | **PASS** — clean, exit 0. |
| Format | `gofmt -l` on the 5 edited files | **PASS** — no files listed. |
| New + safety tests | `go test ./pkg/reads/ -run 'Legibility\|GenuineLoss\|Debris\|Legacy\|PreservedOnDefault\|ReadGitDefaultBranch\|TerminalRunDebris\|UnreachableHeadPreserved' -v -p 1 -timeout 900s` | **PASS** — 8/8 ran (not skipped), incl. the 3 safety reds. |
| Full package | `go test ./pkg/reads/ -count=1 -p 1 -timeout 900s` | **PASS** — `ok … 0.982s` (widened 5-value signatures threaded cleanly through existing anchor tests). |

### Load-bearing safety — reproduced, not trusted

The verdict's hinge is that content on **no** durable ref **and not** on the
default branch, from a non-terminal run, **still reds `ok`**. Three tests prove
each surface and all pass:

- `TestDoctorWorktreeGenuineLossStillReds` — live-run worktree HEAD on no ref /
  not on default branch ⇒ `worktree_head_unreachable` +
  `job_completed_without_anchor`, **zero warnings**.
- `TestDoctorArtifactGenuineLossStillReds` — git-anchor artifact absent
  everywhere ⇒ `artifact_anchor_missing_file`, **zero warnings**.
- `TestDoctorArtifactLegacyBlobKeyGenuineLossStillReds` — empty `blob_key` with
  content absent everywhere ⇒ stays `artifact_blob_metadata_missing`, **not** a
  legacy warning.

I read the corresponding code paths and confirmed the disposition is real, not a
test artifact: `checkArtifactAnchor` / `checkBlobExhaustArtifact` /
`doctorWorktreeRefSafety` only divert to a warning when (preservation matches a
ref or default-branch tip) **or** (`terminalDebrisRunState` is `canceled`/
`failed`); otherwise they fall through to the original `problem`.

### Three reclassification rules — correct

1. **Default-branch preservation.** Worktree HEAD reachable from the resolved
   default ref ⇒ `worktree_unanchored_on_default_branch` warning (stays visible
   so the operator still pins it); a git-anchor artifact whose `content_sha256`
   matches its `repo_path` at the default-branch tip ⇒ fully clean (nothing to
   anchor), exactly as the prompt scopes it.
2. **Terminal-run debris.** `terminalDebrisRunState` matches **only**
   `canceled`/`failed`; `completed` is deliberately excluded, so a genuinely-lost
   deliverable from a successful run is **not** blinded. Confirmed in code and by
   `TestDoctorArtifact*PreservedOnDefaultBranch*` using `run_state: completed`.
3. **Legacy pre-blob-storage.** Empty `blob_key` ⇒ `artifact_legacy_unverifiable`
   warning **only** when `artifactContentPreserved` is true (durable ref or
   default-branch tip); otherwise problem. The `blob_key != "" && expected == ""`
   case correctly stays an `artifact_blob_metadata_missing` problem.

### Default-branch resolution — not hardcoded, degrades safely

`readGitDefaultBranchRef` prefers `git symbolic-ref --quiet
refs/remotes/origin/HEAD`, then falls through `origin/main`, `origin/master`,
`refs/heads/main`, `refs/heads/master`; returns `""` on any failure, and callers
fall back to the prior run-branch/pin-only behavior. Every git call is
`exec.CommandContext(ctx, …)` (`readGitOutput`/`readGitCommit`/`readGitAncestor`)
so the check cannot hang. Resolution is memoized per repo root via
`resolveDefaultRefCached` (at most one resolution per repo per pass).
`TestReadGitDefaultBranchRefDegradesSafely` proves `""` for empty/non-repo paths
and `refs/heads/main` for a local-only repo.

### Scope & blast radius — clean

- Exactly 7 files changed; the only source files are
  `go/pkg/reads/{doctor.go,doctor_artifact_anchor.go,worktree_refs.go}` plus
  their tests. **No** migration / schema / `*.sql` file touched; **no**
  owner-table DDL. The SQL deltas are additive read-only projections
  (`r.state AS run_state`).
- `ok`, `problems`, and `problem_records` are unchanged; `warnings`,
  `warning_records`, and the artifact block's `warning_count` are purely
  additive. `worktreeReclassRecord`'s `record["context"].(map[string]any)`
  assertion is safe — `worktreeProblemRecord` always sets `context` to a
  `map[string]any`.
- Decision-log **D204** exists, is `accepted`, and ties explicitly to the
  `AGENTS.md` "Do not paste over a broken runner" guardrail — matching the
  intent (restore doctor's `ok=false` to mean an actionable, genuine durability
  gap).

## Non-blocking findings (carry to follow-ups; none blocks apply)

1. **`warnings`/`warning_records` are not in the `doctor` redaction allowlist.**
   `go/pkg/reads/redaction.go:177` allowlists only `doctor.ok` and
   `doctor.problems._items`; unlisted keys default to `"redacted"`. So in
   redacted-tier evidence/corpus **exports**, the new warnings are dropped. The
   live `doctor` RPC/CLI output (the operator's primary surface) is unaffected and
   shows warnings in full — and `redaction.go` was correctly left untouched, as
   the draft claims — so this is a conservative, safe default, not a leak.
   *Follow-up:* if redacted exports should surface the reclassified warnings, add
   `warnings` (and `warning_count`) to the doctor allowlist. **severity: info.**

2. **`docs/decisions/decision-log.md` table has no Markdown `|---|` separator
   row.** D204 is inserted directly under the header, matching the existing
   format — D203/D202/… already sit there with no separator. This is a
   **pre-existing** rendering quirk, not introduced by this slice; flagging only
   so it is not mistaken for a new defect. *Follow-up (out of scope):* repair the
   table header once, repo-wide. **severity: info.**

3. **"Preserved" means default-branch *tip*, not history** (already noted in the
   draft's own follow-ups). A legitimately-merged-then-further-edited artifact
   path could still red. Acceptable for P0; a future change could consider
   default-branch history. **severity: info.**

## Conclusion

The implementation does exactly what the draft and D204 claim, the build/vet/
format/tests are green on the run branch (PG-gated suites skipped only because
`STRIATUM_PG_TEST_URL` is unset in this lane — the new logic is not PG-gated and
uses in-package fakes + real temp git repos), and the verdict's hinge —
genuine loss still reds `ok` — is proven by code-read and three passing safety
tests. **accept_with_findings.**
