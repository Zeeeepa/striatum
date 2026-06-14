# RFC 0125: Durable Gate Artifact Provenance — daemon-owned last-mile, body-reconstructability completion gate, same-attempt reseal

Status: proposed
Date: 2026-06-14
author: proposer-claude-opus-4-8-001
Context: RFC 0118 / D-pending#240 (gate run completion on attested provenance — the
verdict half), RFC 0123 / D190 (blob-routed lane exhaust and git publication specs
— the placement taxonomy), RFC 0117 / D176+D178 (worktree/branch ref-safety, durable
`refs/striatum/` anchors, `worktree gc`), RFC 0072 (blob-backed artifact storage),
RFC 0103 (lane sandbox / least-privilege lane OS user), RFC 0008 (per-job worktree
isolation). Driven by the Hippo remaining-campaign retrospective
(`hippo/HIPPO_RUN_RETROSPECTIVE_HIPPO_REMAINING_CAMPAIGN_GPT_5_CODEX_2026-06-14.md`)
and the 14 friction issues it produced (GH #270–#283). Grounded reads at `main`
`c32ff9a9`: `go/pkg/mutations/mutations.go` (`verifyRequiredArtifacts:684`,
`maybeCompleteRun`), `go/pkg/mutations/run_completion_gate.go`
(`verifyRunCompletionProvenance:37`, `escalateProvenanceGateFailure:172`),
`go/pkg/mutations/run_completion_record.go` (`freezeRunCompletionRecord:190`,
`persistRunCompletionRecord:151`), `go/pkg/mutations/artifact.go`
(`HandleArtifactPublish`/`publishArtifact`), `go/pkg/mutations/artifact_durability.go`
(`ensurePerJobPublishedArtifactsDurable:29`, `publishedArtifactDurabilityProblems:62`),
`go/pkg/mutations/worktree.go` (`anchorActiveWorktreeForJob:851`,
`anchorWorktreeCommitStack:942`, the pin-sweep closeout), `go/pkg/mutations/recovery.go`
(`HandleRecoveryResume:212`, `requeueJobSameAttempt:1775`),
`go/pkg/mutations/recovery_auto_finalize.go` (`autoFinalizeCandidateRows`),
`go/pkg/mutations/revision_routing.go` (`reopenJobForAttempt:280`,
`resetJobToBlockedWithReason:321`), `go/pkg/artifactcontracts/placement.go`
(`PlacementBlobExhaust`/`PlacementGitPublication`/`PlacementGitPointerManifest`,
`ResolvePlacement:59`), `go/pkg/reads/artifact_content.go`,
`go/pkg/reads/doctor_artifact_anchor.go`, `go/pkg/cli/params/params.go:173`.

## Summary

RFC 0118 made a run prove that every required gate **verdict** is present,
attempt-scoped, and — for provenance-required gates — attested at record time.
It did **not** make the run prove that every required gate **artifact body** is
durable. RFC 0123 then routed bodies into three placements (`blob_exhaust`,
`git_publication`, `git_pointer_manifest`) but left a hole between "the artifact
**row** exists" and "the artifact **body** is reconstructable." A run can reach a
clean `completed` while a required review or design body is gone from disk and
from git — exactly the Hippo incident, where 7 of 12 required artifacts (every
review + one design) were never durably committed yet the finalizer counted those
gates as passed.

This RFC closes that hole with three load-bearing mechanisms and one cheap
shift-left guard, all on the existing daemon boundary, all without hosted
services or new external persistence:

1. **Daemon-as-porter (durability mechanism).** The lane stops being responsible
   for making its own output durable. At `artifact.publish` the body travels
   daemon-to-store over the RPC channel, and the **daemon** — already running git
   in-process as the operator user — performs the last-mile commit via git
   plumbing (`hash-object -w` → `commit-tree` → CAS `update-ref`). Plumbing needs
   no checked-out branch (kills the detached-HEAD refusal, #281), ignores
   `.gitignore` (kills the ignored-path trap, #278), and never reads the
   lane-unreadable per-job worktree filesystem (kills the ACL trap, #272).

2. **Body-reconstructability completion gate + RUN_LEDGER (audit invariant).**
   A new `verifyRequiredArtifactReconstructable` re-reads and hash-verifies every
   required body at the run-completion boundary, dispatching on placement. It is
   **orthogonal** to the verdict-attestation check, so RFC 0118's legitimate
   "neutral verdict from an unattested session on a non-required review" path is
   untouched. Finalization extends the existing write-once `run_completion_record`
   into a self-contained, content-addressed **RUN_LEDGER** so a retrospective can
   reconstruct gates/verdicts/SHAs/checksums offline, from the ledger hash alone.

3. **Same-attempt reseal / RMA (recovery layer).** A new `recovery.reseal`
   re-runs the idempotent durability probe and, on pass, completes the **same
   attempt** via the existing `requeueJobSameAttempt` primitive — no attempt
   inflation, no duplicate provenance (#271, #273) — with a durable receipt and an
   override-class verdict basis so a resealed gate can never read back as
   `lanes_attested`. Plus the adjacent recovery-legibility fixes (#270, #274).

4. **Shift-left path validation (cheap guard).** Declared `git_publication`
   artifact paths are validated against the target repo's ignore rules at
   prepare/start, failing or redirecting *before* a lane spends work landing into
   an unrecoverable path (#278 prevention, complementing the porter's runtime fix).

The keystone reframe, surfaced by the divergent design pass: **the lane should
never be on the durability path at all.** Once the daemon owns the last-mile
commit, "detached HEAD," "`.gitignore`," and "lane ACL on `.git`" stop being
durability hazards because the per-job worktree is no longer the channel.

## Problem

### The exact gap

`verifyRequiredArtifacts` (mutations.go:684), invoked from
`verifyRunCompletionProvenance` (run_completion_gate.go:63), proves an artifact
**row** is present at the job's current attempt — an attempt-scoped `SELECT`
keyed by job/logical-name/kind/repo-path. It never re-reads the body. RFC 0118's
gate, RFC 0123's placement routing, and RFC 0117's ref anchoring each assume the
body landed; none of them re-verifies it at the completion boundary.

Issue #275 is the smoking gun: `artifact.list_for_run` lists the synthesis
artifact, `artifact.show` returns its metadata including `size_bytes: 29483`, but
`artifact.get_content` returns `not_found: artifact body file does not exist on
disk`. The row and size survived; the body did not. The finalizer counts that
gate as present.

### The five failure modes (all from Hippo run `run_44ada924…`)

| # | Failure | Root mechanism |
| --- | --- | --- |
| #275 | `get_content` "body file does not exist on disk" though row + `size_bytes` exist | gate checks row, not body reconstructability |
| #281 | `git.commit_apply` refuses detached-HEAD job worktrees (`HEAD == run-branch ref` but no branch checked out) | durable path requires a checked-out branch in the lane's worktree |
| #278 | declared path `…/build/HANDOFF.md` is gitignored (`.gitignore: build/`); `work.complete` blocks **after** publish succeeded | durability uses worktree `git add`/HEAD semantics that honor ignore rules |
| #272 | lane OS user `striatum-lane` cannot enter `.striatum/worktrees/…` (0700, operator-owned) to commit | durability path requires lane filesystem access to the worktree |
| #271 / #273 | operator commits the body manually, but the only recovery verb (`run retry-job`) bumps the job to attempt 4 past `max_attempts: 1`, creating duplicate provenance | no same-attempt completion verb for a remediated durability blocker |

Plus the recovery-legibility gaps: #270 (`recovery resume` help advertises a
positional `run-id` but the daemon requires `blocker_id`; params.go:173 maps the
shared `recovery` group's first positional to `run_id`) and #274
(`recovery auto-finalize` returns `eligible_count: 0` with no explanation because
`autoFinalizeCandidateRows` filters blocked/lease-released jobs out before
counting, and never inspects the job worktree).

### Why RFC 0118 did not catch it

RFC 0118 deliberately gates on **verdict** attestation plus artifact-**row**
presence. The body lives in blob storage or a git anchor; the gate trusts that
publication made it durable. The Hippo run shows publication and durability can
disagree: a `git_publication`-placed body can be published (row + `size_bytes`
recorded) while its body never reaches a durable git object — because the only
durable path ran through a worktree that was detached, ignored, or unreadable.
RFC 0118's own §P1-5/§P1-6 open questions flagged "should a durable
`worktree_durability` blocker row exist" and "is the completion record the right
home for body durability"; this RFC answers both.

## Goals

- A required gate artifact body is **reconstructable** (blob readback hash-matches,
  and for durable-provenance kinds, a git anchor hash-matches) before its run can
  reach a clean terminal state. A row + `size_bytes` without a reconstructable body
  fails the run closed, into the existing escalation path — never `completed`.
- The durability path **works from detached-HEAD, gitignored, and lane-unreadable
  worktrees** with no operator git surgery, because the daemon owns the commit.
- A remediated durability blocker completes the **same attempt**, with no attempt
  inflation and no duplicate artifact/verdict rows.
- Finalization emits a **committed, self-contained RUN_LEDGER** (content-addressed)
  that lets a retrospective reconstruct every gate, verdict, SHA, and checksum
  **offline**, without live daemon archaeology.
- **No regression** of RFC 0118's legitimate unattested-neutral admission on
  non-required reviews. Body durability is an orthogonal axis, not a stricter
  attestation rule.

## Non-goals

- **Remote `git push`.** Making a completed branch reach a GitHub remote (#277) is
  a separate provisioning concern; this RFC makes the body durable in the local
  daemon-owned object store and run branch, which is the provenance guarantee.
  #277 is tracked in the campaign's lane-provisioning workstream, not here.
- **Cross-repo lane write provisioning** (#280) and **scratch ACL prep** (#279)
  are lane-provisioning hardening, tracked separately.
- **Stale-verdict lifecycle** (#282/#283) is verdict-accounting legibility (the
  retrospective's revision-convergence finding); it rides RFC 0118's surface and
  is a companion workstream, referenced here but specified outside this RFC.
- **No new artifact shape** is introduced — the RFC 0106 shape freeze holds. The
  RUN_LEDGER reuses the existing `run_completion_record` / pointer-manifest
  machinery, not a new graduated shape.
- **No blob-only durability.** Treating the blob store as the sole durable tier and
  git as an optional async mirror is explicitly rejected (see Anti-bets): it would
  violate the product principle that repository files are durable provenance,
  reconstructable without the live daemon.

## Design

### Mechanism 1 — Daemon-as-porter (the durability path)

Today a repo-write lane writes its artifact into its own worktree and commits it
there; `publishArtifact` reads that file via the active-worktree path, and
`ensurePerJobPublishedArtifactsDurable` (artifact_durability.go:29) proves
durability by `git show HEAD:<repo_path>` against the worktree HEAD, then
`anchorActiveWorktreeForJob` (worktree.go:851) advances `refs/heads/<run_branch>`
or pins `refs/striatum/<run>/<job>/<attempt>`. The daemon already runs these git
commands in-process as the operator user — **not** under the lane's `RunAsUser` —
so it already holds the access the lane lacks.

**Change.** Add a daemon-side commit-by-plumbing helper next to
`anchorWorktreeCommitStack` (worktree.go:942):

```
commitArtifactBodyToRunRef(ctx, repoRoot, runBranch, repoPath string, body []byte)
    (commitSHA string, err error)
```

It runs entirely against the bare `repoRoot` object DB, no worktree checkout:

1. `git hash-object -w --stdin` writes the body as a blob object (ignores
   `.gitignore` — ignore rules are a worktree/index concept, irrelevant to
   `hash-object`). → fixes #278.
2. `git read-tree <run-tip>` + `git update-index --add --cacheinfo
   100644,<blob>,<repoPath>` (or `mktree`) splices the body onto the run tip's
   tree; `git write-tree` materializes it.
3. `git commit-tree <tree> -p <run-tip>` parents a commit on the run tip — needs
   no checked-out branch. → fixes #281.
4. CAS `git update-ref refs/heads/<run_branch> <new> <old>` (the compare-and-swap
   form `anchorWorktreeCommitStack` already uses) advances the ref, with a bounded
   retry on tip-moved.

The body never travels through the per-job worktree filesystem, so the lane OS
user's inability to enter `.striatum/worktrees/…` is irrelevant to durability. →
fixes #272.

**Body transport.** `artifact.publish` accepts the body via a new
`body_base64` envelope param (daemon-to-store over the authenticated MCP/RPC
channel). The worktree-file read path remains the compatibility default for
existing callers; `body_base64` is preferred and required for the porter commit
path. Hashing, front-matter validation, and `expectedAuthorLine` checks
(artifact.go) run on the envelope body exactly as they run on the worktree file
today — no validation is weakened.

**Concurrency (the load-bearing risk).** Two sibling repo-write jobs that both
synthesize a commit on the same run tip will race the `update-ref`; last-writer
wins on the ref and the loser's body silently drops (its row is still recorded,
so the new completion gate would then correctly fail that run — but we want
correctness, not a wedge). Two safe options, in preference order:

- **(A) Per-job pin, fold at closeout.** Each job's porter commit lands on its own
  `refs/striatum/<run>/<job>/<attempt>` pin (no contention — exactly RFC 0117's
  pin model), and the existing reachable-pin sweep folds pins into the run branch
  in a single serialized closeout step. Turns the concurrency problem into an
  already-solved merge; cost is the run branch not reflecting in-flight jobs.
- **(B) CAS-with-retry under the run lock.** Move the splice under the per-run
  advisory lock (`lockRunForJob`, currently taken at `work.complete` but not at
  `artifact.publish`) or give it its own CAS-retry loop.

This RFC recommends **(A)** as the default: it reuses shipped machinery, avoids
widening the publish-time lock, and keeps each job's provenance independently
addressable until the run deliberately folds it.

### Mechanism 2 — Body-reconstructability completion gate + RUN_LEDGER

Add `verifyRequiredArtifactReconstructable(ctx, runner, repositoryID, jobID)` in
mutations.go, mirroring `verifyRequiredArtifacts`' attempt-scoped
`expected_artifacts` loop but, after selecting `blob_key`, `blob_sha256`,
`content_sha256`, `size_bytes`, `repo_path`, resolving placement via
`artifactcontracts.ResolvePlacement` and dispatching a verified readback:

| Placement | Reconstructability check |
| --- | --- |
| `blob_exhaust` | blob readback (`packageBlobClient.GetBytes(bucket, blob_key, blob_sha256)` — the same sha-verifying readback `reads.getContentFromBlob` already uses). Fast blob readback **alone** is sufficient — RFC 0123 D190 already declared these non-durable-repo-provenance. |
| `git_publication` | git-anchor probe (`readGitBlobSHA256` against the durable `run_branch`/job pin) hash-matched to `content_sha256` — **and**, if blob-mirrored, a blob readback too. git is the body of record, not an async mirror. |
| `git_pointer_manifest` | git-anchor probe **plus** resolve each pointer hash. |

Wire it as a second call after `verifyRequiredArtifacts` at
run_completion_gate.go:63, so a non-reconstructable body appends to the existing
`failing[]` set and routes through `escalateProvenanceGateFailure` (gate
failure → `needs_operator`, not `completed`) with a distinct failure key
`required_artifact_unreconstructable`. This is **orthogonal** to the verdict
posture path — RFC 0118's unattested-neutral admission on a non-required review
runs only after artifact verification passes and is not touched.

**The reconciliation rule (resolves the blob-vs-git trap).** A fast blob readback
alone is a trap for `git_publication`-class artifacts: it would let a run pass on
a daemon-resident blob whose git anchor is missing, violating "repository files
are durable provenance, reconstructable without the live daemon." So for
`git_publication`/`git_pointer_manifest` the gate requires the git anchor to
hash-match; blob is supplementary. **Degrade ladder** for runs whose integration
model anchors *after* terminal state (the body is durably blob-resident and
`content_sha256`-correct but the run branch is not yet ff-integrated): drive the
strictness off the workflow's declared placement + RFC 0117 anchor state — if the
contract declares `git_publication` and the anchor exists, require it; if the
anchor is legitimately pending pre-integration, record a **ledger warning**
rather than false-blocking (mirroring how `doctor`, not the completion gate, owns
anchor posture today). This avoids regressing legitimate post-completion
integration while still failing the Hippo case (where the body never anchored at
all).

**RUN_LEDGER.** Extend the existing write-once `run_completion_record`
(`freezeRunCompletionRecord:190`, already content-addressed and anchored in the
`run.completed` event) with a per-gate `artifacts[]` block:

```
{ job_id, workflow_job_id, artifact_id, logical_name, placement,
  content_sha256, blob_sha256, git_anchor_ref, git_anchor_commit,
  verdict_id, lane_attestation_at_record, readback_verified: true }
```

The ledger's own sha256 is recorded in the same transaction that flips the run to
its terminal state, so offline reconstruction needs only the ledger hash to pull
the full corpus from the content-addressed store + run branch. Optionally also
emit the assembled ledger as a publishable `git_pointer_manifest` artifact so the
manifest is itself git-anchored and reconstructable from the ledger hash alone.

### Mechanism 3 — Same-attempt reseal / RMA recovery

The durability failure surfaces as a `work.complete`/auto-finalize **refusal**
(an `rpc.Error` from `ensurePerJobPublishedArtifactsDurable`) and as a
worktree-GC **skip** (`reason: "published_artifact_not_durable"`); neither writes
a `blockers` row. So the recovery verb keys on `(run_id, job_id[, attempt])`, not
a `blocker_id`.

Add `HandleRecoveryReseal` (new `go/pkg/mutations/recovery_reseal.go`,
`CapabilityRecovery`, registered alongside `recovery.invalidate_job` in
mutations.go:133):

1. Load the job + its active `job_worktrees` row.
2. Re-run the idempotent read-only `publishedArtifactDurabilityProblems`
   (artifact_durability.go:62).
3. If `len(problems) == 0` (the operator committed the body, **or** a porter
   last-mile commit landed it), route to `requeueJobSameAttempt`
   (recovery.go:1775) — the existing same-attempt primitive that
   `resumeWriteScopeBlocker` already uses — which requeues **without** bumping
   `attempt`. The fresh claim's `work.complete` then passes
   `verifyRequiredArtifacts` + the new reconstructability check and enqueues
   downstream normally. This sidesteps `HandleRunRetryJob`'s attempt bump
   (run.go) entirely. → fixes #271, #273.
4. Durable receipt: a `recovery.resealed` event; for verdict-capable jobs, the
   RFC 0118 frozen provenance stamp is written with basis
   `daemon_resealed_after_durable_body` (override-class), so a resealed gate can
   **never** read back as `lanes_attested` in the RUN_LEDGER.

Optional `--commit-body` mode turns reseal into a true RMA reconciler: if the body
exists on disk in the per-job worktree with the recorded `content_sha256` but is
uncommitted, the daemon-porter (`commitArtifactBodyToRunRef`) lands it before
re-probing.

**Adjacent recovery-legibility fixes.**
- **#270.** Split the shared `recovery` positional group (params.go:173) so
  `recovery resume`'s first positional maps to `blocker_id` (matching
  `HandleRecoveryResume:212`), and fix the generated help. Add a route guardrail
  test asserting `positionalNames("recovery_resume")[0] == "blocker_id"`.
- **#274.** Make `autoFinalizeCandidateRows` (or a sibling diagnostic pass) also
  surface blocked/lease-released jobs as explicit skip records that run the
  durability probe against the job worktree, so `eligible_count: 0` is accompanied
  by a per-job `published_artifact_not_durable` explanation pointing the operator
  at `recovery.reseal`.
- **Optional P1-6 answer.** Mint a first-class durable `worktree_durability`
  blocker row at the refusal sites, so the condition is queryable, `recovery.reseal`
  can optionally accept a `blocker_id`, and the RUN_LEDGER can cite a resealed gate.

### Mechanism 4 — Shift-left declared-path validation

At `run prepare`/`run start`, validate each declared `git_publication` artifact
path against the target repo's ignore rules. A gitignored declared path is
rejected (or annotated for porter force-add) **before** any lane work, so the
operator fixes the workflow fixture at the only moment that costs nothing.
Complements Mechanism 1: the porter makes ignored paths durable at runtime; this
guard stops the workflow from declaring them in the first place. → #278 prevention.

## Phasing

| Phase | Scope | Closes |
| --- | --- | --- |
| **P0-1** | `commitArtifactBodyToRunRef` plumbing helper + concurrency model (A) + unit tests for concurrent splice | (enables #272/#278/#281) |
| **P0-2** | `artifact.publish` `body_base64` envelope + porter placement branch in `publishArtifact`; durability check verifies against the run ref | #272, #281, #278 |
| **P0-3** | `verifyRequiredArtifactReconstructable` + wire into `verifyRunCompletionProvenance`; placement degrade ladder; regression fence (delete body under a present row ⇒ gate fails; unattested-neutral non-required review still completes) | #275 |
| **P1-1** | RUN_LEDGER `artifacts[]` extension to `run_completion_record` + optional pointer-manifest emission | (retrospective rec 2/4) |
| **P1-2** | `recovery.reseal` + `requeueJobSameAttempt` wiring + pgtest (attempt unchanged, single artifact row, downstream enqueued) | #271, #273 |
| **P1-3** | #270 positional fix + guardrail test; #274 blocked-job skip legibility | #270, #274 |
| **P2-1** | Shift-left declared-path validation at prepare/start | #278 (prevention) |
| **P2-2** | Retire the lane git identity for porter-committed runs (lane becomes a pure body producer; shrinks RFC 0103 sandbox) | (architectural end-state) |

## Anti-bets (what this RFC refuses)

- **Blob-only durability with git as an optional async mirror.** Rejected: it
  makes provenance depend on the live daemon's blob store and breaks "reconstructable
  without live daemon archaeology." git anchoring stays mandatory for
  durable-provenance kinds.
- **Apoptosis / void-all-attempts + fresh recovery run.** Rejected: destroying
  attempt records loses the provenance we are protecting and contradicts the
  same-attempt goal. Reseal preserves attempt identity.
- **Streaming bodies from lane stdout/PTY into the store.** Rejected: PTY/terminal
  output is explicitly non-authoritative (product boundary + RFC 0119 redaction).
  The `body_base64` MCP envelope is the only sanctioned channel.
- **tmpfs staging buffer.** Rejected as a durable tier: not durable across daemon
  restart. Bodies stage to the content-addressed blob store.
- **A new graduated artifact shape for the ledger.** Rejected: RFC 0106 freeze
  holds; reuse `run_completion_record` / pointer-manifest.

## Test obligations (hard, before any P0 lands)

1. **#275 regression fence (pgtest):** complete a run, delete the blob body /
   git-anchor out from under a present required-artifact row, assert
   `maybeCompleteRun` routes to `needs_operator` / `provenance_gate_failed` with
   failure `required_artifact_unreconstructable` — not `run.completed`.
2. **RFC 0118 no-regression (pgtest):** a `posture=neutral` unattested verdict on a
   non-required review still completes the run cleanly with body durability
   satisfied.
3. **Porter concurrency (unit):** two concurrent `commitArtifactBodyToRunRef`
   splices on one tip both survive (pin model A) or CAS-retry converges (model B);
   neither silently drops a body.
4. **Detached/ignored/unreadable (pgtest or harness):** a `git_publication` body in
   a gitignored path, from a detached job worktree, with the lane OS user unable to
   read `.striatum`, becomes durable with no operator shell access.
5. **Reseal same-attempt (pgtest):** `jobs.attempt` unchanged before/after; exactly
   one `artifacts` row per logical name at that attempt; downstream enqueued.
6. **RUN_LEDGER offline reconstruction:** from the ledger hash alone, every required
   gate's body, verdict, SHA, and attestation stamp resolve from the
   content-addressed store + run branch with the daemon stopped.

## Issue disposition

Closes via implementation: #270, #271, #272, #273, #274, #275, #278, #281.
Companion workstreams (referenced, specified elsewhere): #277 (push provisioning),
#279 (scratch ACLs), #280 (cross-repo write provisioning), #276 (lane env
hermeticity), #282 / #283 (stale-verdict lifecycle legibility).

## Open questions

1. Concurrency model **(A)** pin-fold vs **(B)** CAS-under-lock — recommend (A);
   confirm against the multi-run isolation invariants (RFC 0108).
2. Should the RUN_LEDGER be emitted as a first-class `git_pointer_manifest`
   artifact unconditionally, or only when the workflow declares durable-audit?
3. Do we mint a durable `worktree_durability` blocker row (P1-6 answer) now, or
   keep the durability failure as a transient refusal keyed by job/attempt?
4. Does retiring the lane git identity (P2-2) belong in this RFC's scope or a
   follow-on once the porter is proven?
