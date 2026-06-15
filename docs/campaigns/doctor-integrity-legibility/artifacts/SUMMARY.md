---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
inputs:
  - docs/campaigns/doctor-integrity-legibility/artifacts/DRAFT.md
  - docs/campaigns/doctor-integrity-legibility/artifacts/review/REVIEW.md
  - docs/decisions/decision-log.md
---

# Doctor integrity legibility (P0) — SUMMARY

author: author-agent-002

The fresh-eyes review returned **accept_with_findings**: all four
`needs_revision` triggers were cleared (genuine-loss detection does not regress,
`ok` is not weakened beyond the three reclassification rules, the default branch
is not hardcoded, and the load-bearing safety tests are present and exercised).
The three review findings are **non-blocking** (severity: info) and are carried
to follow-ups below — **none required a revision**, so the draft implementation
ships as-is. No further code changes were made in this apply step.

## What landed

Committed on `striatum/doctor-integrity-legibility-p0` (`f719ff1a`, diffed
against scaffold base `0d5021f3`). Read-only diagnostic change in
`go/pkg/reads/`; no schema change, no migration, no new RPC method, no change to
what `ok` *means*.

| File | Change |
| --- | --- |
| `go/pkg/reads/worktree_refs.go` | `doctorWorktreeRefSafety` now returns `(block, problems, problemRecords, warnings, warningRecords)`; worktree SQL adds `r.state AS run_state`; per-row reclassification (rules 1–2). New helpers `terminalDebrisRunState`, `resolveDefaultRefCached`, `readGitDefaultBranchRef`, `worktreeReclassRecord`. |
| `go/pkg/reads/doctor_artifact_anchor.go` | `doctorArtifactAnchorIntegrity` / `checkArtifactAnchor` / `checkBlobExhaustArtifact` thread `(…, warnings, warningRecords)` plus a resolved `defaultRef`; artifact SQL adds `r.state AS run_state`; rules 1–3 applied. New helpers `artifactContentPreserved`, `artifactContentMatchesAnyRef`, `artifactContentMatchesRef`, `blobBodyVerifyResult`, `artifactWarning`. |
| `go/pkg/reads/doctor.go` | Captures the new `warnings` / `warningRecords` from both checks, appends `warnings` next to the existing channel, and (verbose only) adds an additive `warning_records` field alongside `problem_records`. |
| `go/pkg/reads/doctor_integrity_legibility_test.go` | New unit tests for every rule, the three load-bearing safety cases, and default-branch-resolver degradation. |
| `go/pkg/reads/doctor_artifact_anchor_test.go` | Existing call sites updated for the widened 5-value signature; behavior unchanged. |
| `docs/decisions/decision-log.md` | New decision **D204** (`accepted`), tied to the `AGENTS.md` "Do not paste over a broken runner" guardrail. |

**Decision-log entry: D204.**

### Warning taxonomy introduced

A new, additive `warnings` channel (the artifact block also gains `warning_count`;
verbose adds `warning_records`). `ok`, `problems`, and `problem_records` are
unchanged. Reclassified codes:

| Warning code | Replaces problem | Condition |
| --- | --- | --- |
| `worktree_unanchored_on_default_branch` | `worktree_head_unreachable` (+`job_completed_without_anchor`) | Worktree HEAD reachable from the resolved default branch (merged, run branch deleted). Stays visible — operator should still create the `refs/striatum` pin. |
| `worktree_debris_terminal_run` | `worktree_head_unreachable` | Run state is terminal debris (`canceled`/`failed`). |
| `artifact_legacy_unverifiable` | `artifact_blob_metadata_missing` | Blob-placement artifact with empty `blob_key` (predates RFC 0125) **and** content verifiable on a durable ref or the default branch. |
| `artifact_debris_terminal_run` | `artifact_anchor_*` / `artifact_blob_*` | Artifact finding whose run is terminal debris (`canceled`/`failed`). |
| _(none — fully clean)_ | `artifact_anchor_missing_file` / `artifact_anchor_hash_mismatch` | Git-anchor artifact whose `content_sha256` matches its `repo_path` at the default-branch tip (durably preserved). |

The default branch is resolved without hardcoding `"main"` (`readGitDefaultBranchRef`:
`git symbolic-ref refs/remotes/origin/HEAD`, then common remote/local
`main`/`master`), memoized per repo root, and degrades safely to `""`
(run-branch/pin-only behavior) on any failure.

## Before / after expectation

- **Before:** `striatum doctor` reported ~276 "problems" that were almost
  entirely un-actionable — artifacts whose content is durably on the default
  branch, 150 blob-placement artifacts that predate RFC 0125 blob storage, and
  canceled-run worktree debris. The permanent red is why operators stopped
  trusting `doctor`.
- **After:** `doctor`'s `problems` count drops to **only genuinely-unpreserved
  content**. Preserved-on-default-branch, terminal-run (`canceled`/`failed`),
  and legacy pre-blob-storage findings now appear as `warnings` and **do not red
  `ok`**. A red `ok=false` again means an actionable, genuine durability gap.
- **What still reds `ok` (genuine loss, by design):** a worktree HEAD on no
  durable ref **and not** reachable from the default branch from a non-terminal
  run; a git-anchor artifact whose content is on no ref and not at the
  default-branch tip from a non-terminal run; a legacy empty-`blob_key` artifact
  whose content is absent everywhere; a real blob-body verify failure on a
  non-terminal run. `completed` runs are intentionally **excluded** from
  terminal-debris reclassification, so a genuinely-lost deliverable from a
  successful run is never blinded. Proven by three load-bearing safety tests
  (`TestDoctorWorktreeGenuineLossStillReds`,
  `TestDoctorArtifactGenuineLossStillReds`,
  `TestDoctorArtifactLegacyBlobKeyGenuineLossStillReds`).

Build / vet / gofmt / new tests / full `pkg/reads` suite are green on the run
branch (per the draft and reproduced independently in the review). PG-gated
suites were skipped only because `STRIATUM_PG_TEST_URL` is unset in this lane;
the new logic is not PG-gated (in-package fakes + real temp git repos).

## Accepted-review findings deferred to follow-ups

All three are severity **info**, non-blocking; none changes `ok` behavior on the
operator's primary surface (the live `doctor` RPC/CLI output shows warnings in
full).

1. **`warnings` / `warning_records` not in the `doctor` redaction allowlist**
   (`go/pkg/reads/redaction.go:177` allowlists only `doctor.ok` and
   `doctor.problems._items`). Redacted-tier evidence/corpus **exports** drop the
   new warnings; live output is unaffected. `redaction.go` was deliberately left
   untouched — a conservative safe default, not a leak. *Follow-up:* if redacted
   exports should surface the reclassified warnings, add `warnings` (and
   `warning_count`) to the doctor allowlist.
2. **`decision-log.md` table has no Markdown `|---|` separator row** — a
   **pre-existing** repo-wide rendering quirk (D203/D202/… already sit under the
   header with no separator), not introduced by this slice. *Follow-up
   (out of scope):* repair the table header once, repo-wide.
3. **"Preserved" means default-branch *tip*, not history** — a
   legitimately-merged-then-further-edited artifact path could still red.
   Acceptable for P0; a future change could consider default-branch history.

Draft-noted follow-ups also carried: `failed` runs are currently treated
identically to `canceled` for debris reclassification (revisit if they warrant a
distinct disposition); RFC 0127 plain-dir workspaces would change how
worktree/artifact durability is probed.

## Operator follow-ups

This is **daemon code** — the reclassification takes effect only after the
daemon is rebuilt and restarted:

1. `make install`
2. `systemctl --user restart striatumd`
3. Re-run `striatum doctor` and confirm `ok` now reds **only** on genuine loss
   (the preserved-on-`main` artifacts, the ~150 empty-`blob_key` legacy
   artifacts, and the canceled-run worktree debris should have moved from
   `problems` to `warnings`).
4. **A-data reconciliation of the residual true-loss tail:** whatever still reds
   after the restart is genuinely-unpreserved content — reconcile that remaining
   tail (pin/anchor or accept the loss) so `doctor` returns to a clean,
   trustworthy `ok`. **Do not merge this branch to `main`** as part of this step;
   merge follows the normal review-to-`main` path.
