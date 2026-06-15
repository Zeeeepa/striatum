# Doctor integrity legibility (P0) — DRAFT

author: author-agent-001

Makes `striatum doctor`'s artifact/worktree integrity checks **legible**: a red
`ok=false` again means an **actionable, genuine durability gap**. Today doctor
reports ~276 "problems" that are almost entirely un-actionable noise — artifacts
whose content is durably on the default branch, artifacts that predate blob
storage, and canceled-run worktree debris. That permanent red is exactly why
operators stopped trusting doctor and silently worked around the few real gaps,
the failure mode `AGENTS.md` → "Do not paste over a broken runner" forbids. This
slice reclassifies the noise to `warnings` so `ok` reds only on genuine loss.

Scope is **strictly P0**: read-only diagnostic code in `go/pkg/reads/`. No schema
change, no migration, no new RPC method, no change to what `ok` *means* beyond
moving reclassified items off `problems`.

---

## 1. Design note

### Root cause

Both integrity checks resolved durability **only** from the run branch +
`refs/striatum/<run>/*` pins (`durableWorktreeProbeRefs`), **never** the
repository default branch, and ignored run terminality and pre-blob-storage
legacy. So:

- Normal post-merge run-branch deletion (which `AGENTS.md` *encourages* — "Do
  not strand pushed branches") removes the run branch, leaving a completed-job
  worktree HEAD / artifact whose content is merged on `main` but unreachable
  from any probed ref → permanent `worktree_head_unreachable` /
  `artifact_anchor_missing_file` problems.
- Canceled-run worktree debris cannot be physically cleaned (`worktree release
  --force` refuses an undurable published artifact, and the worktree dir is
  owned by the `striatum-lane` sandbox user) → permanent
  `worktree_head_unreachable` problems.
- Blob-placement artifacts with an empty `blob_key` predate RFC 0125 blob
  storage → 150 permanent `artifact_blob_metadata_missing` problems even though
  the content is on `origin/main`.

### Edit sites

| File | Change |
| --- | --- |
| `go/pkg/reads/worktree_refs.go` | `doctorWorktreeRefSafety` now returns `(block, problems, problemRecords, warnings, warningRecords)`; SQL adds `r.state AS run_state`; per-row reclassification (rules 1–2). New helpers: `terminalDebrisRunState`, `resolveDefaultRefCached`, `readGitDefaultBranchRef`, `worktreeReclassRecord`. |
| `go/pkg/reads/doctor_artifact_anchor.go` | `doctorArtifactAnchorIntegrity` / `checkArtifactAnchor` / `checkBlobExhaustArtifact` now thread `(…, warnings, warningRecords)` and a resolved `defaultRef`; SQL adds `r.state AS run_state`; rules 1–3 applied. New helpers: `artifactContentPreserved`, `artifactContentMatchesAnyRef`, `artifactContentMatchesRef`, `blobBodyVerifyResult`, `artifactWarning`. |
| `go/pkg/reads/doctor.go` | Captures the new `warnings`/`warningRecords` from both checks, appends `warnings` next to the existing `warnings = append(...)`, and (verbose only) adds an additive `warning_records` field alongside `problem_records`. |
| `docs/decisions/decision-log.md` | New decision **D204** (tied to the "Do not paste over a broken runner" guardrail). |
| `go/pkg/reads/doctor_integrity_legibility_test.go` | New unit tests for every rule + the load-bearing safety cases + default-branch resolver degradation. |
| `go/pkg/reads/doctor_artifact_anchor_test.go` | Existing call sites updated for the widened 5-value signature (`_, _` for the new returns); behavior unchanged. |

### Default-branch resolution approach

`readGitDefaultBranchRef(ctx, repoRoot)` resolves the repo's default branch
**without hardcoding `"main"`**:

1. `git symbolic-ref --quiet refs/remotes/origin/HEAD` (the authoritative remote
   default), else
2. first existing of `refs/remotes/origin/main`, `refs/remotes/origin/master`,
   `refs/heads/main`, `refs/heads/master`.

It **degrades safely**: any failure / unresolvable ref returns `""`, and callers
then fall back to the prior run-branch/pin-only behavior (never crash, never
hang — every git call is `ctx`-cancellable via `readGitOutput`/`readGitCommit`).
Resolution is memoized per repo root (`resolveDefaultRefCached`) so a pass over
hundreds of rows issues **at most one** resolution per repo.

### Warning taxonomy (reclassified, non-`ok`-reddening)

| Warning code | Replaces problem | Condition |
| --- | --- | --- |
| `worktree_unanchored_on_default_branch` | `worktree_head_unreachable` (+`job_completed_without_anchor`) | Worktree HEAD reachable from the default branch (merged, run branch deleted). Stays visible — the operator *should* still create the `refs/striatum` pin. |
| `worktree_debris_terminal_run` | `worktree_head_unreachable` | Run state is terminal debris (`canceled`/`failed`); leftover, not an active gap. |
| `artifact_legacy_unverifiable` | `artifact_blob_metadata_missing` | Blob-placement artifact with empty `blob_key` (predates RFC 0125) **and** content verifiable on a durable ref or the default branch. |
| `artifact_debris_terminal_run` | `artifact_anchor_*` / `artifact_blob_*` | Artifact finding whose run is terminal debris (`canceled`/`failed`). |
| _(none — fully clean)_ | `artifact_anchor_missing_file` / `artifact_anchor_hash_mismatch` | Git-anchor artifact whose `content_sha256` matches its `repo_path` at the **default-branch tip** (durably preserved; nothing for the operator to do). |

Per the prompt, artifact default-branch preservation is **clean** (not even a
warning — the content is literally on `main`), whereas a default-branch-preserved
*worktree* stays a warning because the operator should still pin it.

### What stays a `problem` (genuine loss — preserved on purpose)

- A worktree HEAD on **no** durable ref **and not** reachable from the default
  branch, from a **non-terminal** run → `worktree_head_unreachable`
  (+ `job_completed_without_anchor` for a completed job).
- A git-anchor artifact whose content is on **no** durable ref **and not** at the
  default-branch tip, from a non-terminal run → `artifact_anchor_missing_file` /
  `artifact_anchor_hash_mismatch`.
- A legacy empty-`blob_key` artifact whose content is **absent everywhere**
  (no durable ref, not on the default branch) → `artifact_blob_metadata_missing`.
- A real blob-body verify failure on a non-terminal run →
  `artifact_blob_body_verify_failed`.

`completed` runs are intentionally **excluded** from terminal-debris
reclassification: their preservation is verified against durable refs / the
default branch, so the fix does not blind doctor to a genuinely-lost deliverable
from a successful run.

### Non-goals / safety

- `ok` is unchanged (`len(problems) == 0`); only the *contents* of `problems`
  shrink.
- `problems` / `problem_records` and existing JSON consumers (incl.
  `go/pkg/reads/redaction.go`, which lists `problems` / `doctor`) are untouched;
  `warning_records` / `warning_count` are **additive**.
- No `"main"` hardcoding; degrade-safe default-branch resolution.

---

## 2. Build & test results (honest)

Run from the per-job worktree on branch `striatum/doctor-integrity-legibility-p0`.

| Check | Command | Result |
| --- | --- | --- |
| Build | `make -C go build` | **PASS** — `striatum`, `striatumd`, `striatum-supervisor-helper` all built. |
| Vet | `go vet ./pkg/reads/` | **PASS** — clean. |
| Lint | `golangci-lint run --default=none --enable=govet --enable=staticcheck --enable=errcheck --enable=ineffassign ./pkg/reads/...` | **PASS** — `0 issues.` |
| Format | `gofmt -l` on all edited files | **PASS** — no files listed. |
| New tests | `go test ./pkg/reads/ -run 'Legibility\|GenuineLoss\|Debris\|Legacy\|PreservedOnDefault\|ReadGitDefaultBranch\|TerminalRunDebris\|UnreachableHeadPreserved' -v` | **PASS** — 8/8 ran (not skipped). |
| Full package | `go test ./pkg/reads/ -count=1` | **PASS** — `ok … 0.798s`. |

New tests (all in `doctor_integrity_legibility_test.go`):

- `TestDoctorWorktreeUnreachableHeadPreservedOnDefaultBranchWarns` — rule 1
  (worktree → warning).
- `TestDoctorWorktreeTerminalRunDebrisWarns` — rule 2 (canceled-run worktree →
  warning).
- `TestDoctorWorktreeGenuineLossStillReds` — **load-bearing safety** (live-run
  worktree on no ref / not on default branch still reds `ok`, no warning).
- `TestDoctorArtifactAnchorPreservedOnDefaultBranchIsClean` — rule 1 (artifact
  preserved at default-branch tip → fully clean).
- `TestDoctorArtifactLegacyBlobKeyPreservedOnDefaultBranchWarns` — rule 3 (legacy
  empty `blob_key`, preserved → `artifact_legacy_unverifiable` warning).
- `TestDoctorArtifactGenuineLossStillReds` — **load-bearing safety** (artifact on
  no ref / not on default branch still reds `ok`).
- `TestDoctorArtifactLegacyBlobKeyGenuineLossStillReds` — **load-bearing safety**
  (legacy empty `blob_key` with content absent everywhere stays a problem).
- `TestReadGitDefaultBranchRefDegradesSafely` — resolver returns `""` for an
  empty / non-repo path and `refs/heads/main` for a local-only repo.

### PG-gated checks — skipped, and why

The doctor integrity logic added here is **not** PG-gated: the new tests use the
in-package fake runner (`doctorWorktreeAnchorFakeRunner` /
`doctorArtifactAnchorRunner`) plus real temporary `git` repos, so they verify the
reclassification end-to-end without PostgreSQL. PG-backed tests in `pkg/reads`
(`*_pg_test.go`) are **gated on `STRIATUM_PG_TEST_URL`, which is unset in this
lane environment**, so `go test ./pkg/reads/` skipped them. I did **not** spin up
a throwaway Postgres to run them — that is outside this read-only diagnostic
slice and not required to validate the change. A reviewer with a test database
should run the doctor PG suite scoped (e.g. `-run 'Doctor' -p 1 -timeout 900s`
with `STRIATUM_PG_TEST_URL` set) to confirm the unchanged SQL projections still
hydrate `run_state` as expected against a live schema.

### Verification against the ~276 live problems

Not exercised in this lane (the live daemon's PostgreSQL state is not the test
fixture here). The unit tests prove each reclassification rule and, critically,
that genuine loss still reds `ok`. The expected live effect: the
preserved-on-`main` artifacts, the 150 empty-`blob_key` legacy artifacts, and the
canceled-run worktree debris move from `problems` to `warnings`, leaving `ok`
red only for content on no durable ref and not on the default branch.

---

## 3. Follow-ups (out of P0 scope)

- "Preserved" currently means matching at the **default-branch tip**. A
  legitimately-merged-then-edited artifact path could still red; a future change
  could consider default-branch *history*.
- `failed` runs are treated identically to `canceled` for debris
  reclassification; revisit if they warrant a distinct disposition.
- RFC 0127 plain-dir workspaces would change how worktree/artifact durability is
  probed.
