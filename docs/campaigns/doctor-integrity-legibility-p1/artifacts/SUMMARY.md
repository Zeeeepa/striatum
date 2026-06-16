---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
inputs:
  - docs/campaigns/doctor-integrity-legibility-p1/artifacts/DRAFT.md
  - docs/campaigns/doctor-integrity-legibility-p1/artifacts/review/REVIEW.md
  - docs/decisions/decision-log.md
---

# Doctor integrity legibility (P1) — SUMMARY

author: author-agent-002

The fresh-eyes review returned **`accept_with_findings`**: no `needs_revision`
trigger fired. The reviewer reproduced the draft's PASS claims independently from
the committed run-branch state (`striatum/doctor-integrity-legibility-p1`, HEAD
`763a4667`) in a throwaway detached worktree — build, lint, and every targeted
test (including the load-bearing safety reproductions) pass. The two review
findings are **non-blocking** (severity: info) and are carried to follow-ups
below — **none required a revision**, so the draft implementation ships as-is.
**No further code changes were made in this apply step.**

## What landed

Committed on `striatum/doctor-integrity-legibility-p1` (two lane commits,
`335fbb57` artifact publication + `763a4667` source changes, diffed against
scaffold base `a3dae461`). Read-only diagnostic change in `go/pkg/reads/`; **no
schema change, no migration, no new RPC method, no change to what `ok` *means***.

| File | Change |
| --- | --- |
| `go/pkg/reads/doctor_artifact_anchor.go` | Extended (+222/−16). New warning codes `artifact_superseded_on_default_branch` / `artifact_acknowledged_loss`; `defaultRefHistoryRevisionCap = 200`. New per-pass cache bundle `artifactAnchorPass` (default-ref / history / ack-baseline caches) threaded through `checkArtifactAnchor` and `checkBlobExhaustArtifact`. New helpers `artifactContentInDefaultRefHistory` / `computeContentInDefaultRefHistory` (Rule A) and `pathExistsOnRef` (Rule B). `artifactContentPreserved` now also consults default-branch *history*; `artifactWarning` switch extended; `acknowledgedLossWarning` carries `reason` + `acknowledged_by`. Surfaces the additive block field `acknowledged_loss_status`. |
| `go/pkg/reads/doctor_acknowledged_loss.go` | **New** (111 lines). Schema `striatum.doctor.acknowledged_loss.v1` reader for the curated baseline at `docs/operator/doctor-acknowledged-loss.json`, with sha-bound `honor`; load statuses `absent` / `loaded` / `parse_error`. |
| `go/pkg/reads/doctor.go` | **Unchanged.** The existing assembly already appends the artifact check's `(warnings, warningRecords)` channel and computes `ok = len(problems)==0`; the new rules only move rows from `problems` to `warnings`, so no assembly change was needed. |
| `go/pkg/reads/doctor_integrity_legibility_p1_test.go` | **New** (247 lines). Unit tests for Rule A (history clean), Rule B (superseded warning), Rule C accept, the load-bearing safety cases, and reader safe-degrade. |
| `go/pkg/reads/doctor_artifact_anchor_test.go` | Two existing fixtures re-seeded onto a ref-only path absent from the default branch so they still exercise the genuine `artifact_anchor_*` **problem** branch under the new ordering. |
| `docs/decisions/decision-log.md` | New decision **D205** (`accepted`), references D204 and the `AGENTS.md` "Do not paste over a broken runner" guardrail; records that the live 16-entry baseline is operator-curated provenance committed separately. |

**Decision-log entry: D205.**

### The three rules (applied in a fixed order)

D204 left a residual of **42** historical-loss artifact `problems`, split by the
operator into three classes; this slice clears each:

| Rule | Class | Condition | Result |
| --- | --- | --- | --- |
| **A** — default-branch *history* awareness | Recoverable | `content_sha256` matches `repo_path`'s blob at **any** reachable revision of the resolved default branch (not only its tip); bounded `--max-count=200`, `ctx`-cancellable, memoized per `root\|ref\|path\|sha`, safe-degrades to "not preserved" | Fully **clean** for a git-anchor artifact / `artifact_legacy_unverifiable` warning for an empty-`blob_key` legacy artifact |
| **B** — superseded-on-default-branch | Superseded | Content unverifiable on every ref/tip/history but `repo_path` is still **live on the default-branch tip** (any content) — the deliverable landed, only the recorded *draft* sha is unverifiable (`git cat-file -e <tip>:<path>`) | New `artifact_superseded_on_default_branch` **warning** (not a problem) |
| **C** — curated acknowledged-loss baseline | Genuine loss | Genuine loss (path absent from default branch, content on no ref) whose `artifact_id` **and** `content_sha256` match a curated entry in the tracked baseline | New `artifact_acknowledged_loss` **warning** carrying the operator's `reason` / `acknowledged_by`; an uncatalogued or sha-mismatched loss stays a `problem` |

Check order: **tip-exact → Rule A history → terminal-debris → Rule B superseded →
Rule C acknowledged → genuine-loss problem.** New warning codes:
`artifact_superseded_on_default_branch`, `artifact_acknowledged_loss`. New
additive block field: `acknowledged_loss_status`. The default branch is resolved
via the existing `resolveDefaultRefCached` (no `"main"` hardcode) and threaded
through the new `artifactAnchorPass` cache bundle.

## Before / after expectation

- **Before:** after D204, `striatum doctor` reports `ok=false` with **42**
  `problems` — all old historical-loss artifacts (14 recoverable via history, 12
  superseded-at-tip, 16 genuine-but-immaterial dogfood losses). A permanently-red
  doctor is exactly what erodes the `AGENTS.md` "Do not paste over a broken
  runner" guardrail.
- **After** (once the operator deploys this code **and** commits the curated
  baseline), the 42 `problems` split into:
  - **14 clean** (Rule A — durably preserved in default-branch history),
  - **12 `artifact_superseded_on_default_branch` warnings** (Rule B),
  - **16 `artifact_acknowledged_loss` warnings** (Rule C),

  taking `problems` to **0** and `ok` to **`true`** — while the warning channel
  still carries the full story.
- **What still reds `ok` (genuine loss, by design):** any artifact whose content
  is on no ref, **not** in default-branch history, whose `repo_path` is **absent**
  from the default branch, and which is **not** in the sha-bound ack baseline (or
  present only with a mismatched/empty sha). The fix does not blind doctor.
  Proven by the load-bearing safety tests
  (`TestDoctorArtifactAcknowledgedLossSafetyStillReds/not_in_baseline` and
  `/sha_mismatch`, plus the unchanged `TestDoctorArtifactGenuineLossStillReds` /
  `TestDoctorArtifactLegacyBlobKeyGenuineLossStillReds`).

Build (`make -C go build`), CI lint
(`golangci-lint run --default=none --enable=govet --enable=staticcheck
--enable=errcheck --enable=ineffassign ./pkg/reads/...` → `0 issues.`), and the
full targeted suite are green on the run branch — claimed by the draft and
**reproduced independently** by the review. PG-gated suites in the package fail
at connect time only (peer-auth in this lane); the new logic is not PG-gated.

## Acknowledged-loss file contract

- **Path:** `docs/operator/doctor-acknowledged-loss.json` (repository-relative,
  tracked, optional — a missing file is the normal state).
- **Schema:** `striatum.doctor.acknowledged_loss.v1`. A file with any other (or
  missing) schema is a `parse_error` and downgrades nothing.
- **Entry shape:** `artifact_id`, `repo_path`, `content_sha256`, `reason`,
  `acknowledged_by`, `acknowledged_at`.
- **Sha-bound downgrade:** an entry is honored **only** when its
  `content_sha256` equals the row's recorded `content_sha256` (case-insensitive
  hex, both trimmed, both non-empty). An id match with a mismatched or empty sha
  is **not** honored — the loss stays a `problem`. This binds each acknowledgment
  to the exact lost content, so a stale or wrong entry can never mask a
  *different* future problem at the same artifact id.
- **Safe-degrade when absent/broken:** `loadAcknowledgedLossSet` never errors. A
  missing file → `absent` empty set (no downgrades); an unreadable/malformed file
  or wrong schema → `parse_error` empty set (still no downgrades). A broken or
  missing baseline can only **fail to downgrade** a problem, never mask one — the
  load-bearing safety default. The status surfaces as the additive
  `acknowledged_loss_status` block field.
- **NOT authored here.** This slice ships only the reader + schema + fixtures +
  tests. The live 16-entry baseline is **operator-curated provenance** the
  operator commits separately after verifying the real residual against the
  deployed code.

## Accepted-review findings deferred to follow-ups

Both are severity **info**, non-blocking; neither is a durability or safety
concern, and neither changes `ok` behavior on the operator's primary surface.

1. **The baseline is the gating follow-up, not this slice.** Doctor does not
   actually reach `ok=true` until the operator curates and commits the 16-entry
   `docs/operator/doctor-acknowledged-loss.json` against the **deployed** code.
   Until then the 16 genuine-loss rows correctly remain `problems` (the reader
   safe-degrades while the file is absent). Track it as the explicit next action
   (operator follow-up 2 below), not an implied side effect of merging this code.
2. **History-scan cost on a degenerate residual.** Rule A runs one `git log` plus
   up to 200 `git cat-file` probes per *unpreserved* git-anchor row — gated behind
   the cheaper tip/ref checks, memoized per `root|ref|path|sha`, `ctx`-cancellable,
   and capped at 200. For today's ≤42-row residual this is negligible; D205 already
   names "cap exceeded by a real path" as a revisit trigger. Worth a glance if the
   artifact population grows large with many genuinely-lost paths.

Draft-noted follow-ups also carried (from D205's revisit triggers): raise/tune
the 200-revision history cap if a real path exceeds it; give `failed` runs a
disposition distinct from `canceled` if warranted; add provenance tooling/a
validation verb if the acknowledged-loss baseline grows large; revisit when
RFC 0127 plain-dir workspaces change how artifact durability is probed.

## Operator follow-ups (in order)

1. **Deploy the daemon code.** `make install` + `systemctl --user restart
   striatumd`. This is **daemon code** — the new rules are inert until the running
   image is replaced. Verify the restart actually swapped the binary: confirm
   `/proc/<MainPID>/exe` is **not** `(deleted)` (a stale, still-running old image
   would silently keep the pre-P1 behavior).
2. **Curate + commit the baseline.** Author the 16-entry
   `docs/operator/doctor-acknowledged-loss.json` (schema
   `striatum.doctor.acknowledged_loss.v1`) against the deployed code, binding each
   entry's `content_sha256` to the exact lost row. This is operator-curated
   provenance — it is deliberately **not** authored by this lane.
3. **Re-run and confirm.** `striatum doctor` should now report `ok=true` with
   **0 problems**, and the warning channel should carry the full story
   (14 clean, 12 `artifact_superseded_on_default_branch`, 16
   `artifact_acknowledged_loss`).
4. **Record + close.** Add the CHANGELOG entry and close #300.

**Do NOT merge this branch to `main`** as part of this apply step — merge follows
the normal review-to-`main` path. The live acknowledged-loss baseline file is
left for the operator to curate and commit separately.
