# Draft — Doctor integrity legibility (P0)

Make `striatum doctor`'s artifact/worktree integrity checks **legible**: a red
`ok=false` must mean an **actionable, genuine durability gap** — not accumulated
historical / preserved-on-`main` noise. Today doctor reports ~276 problems; the
vast majority are artifacts whose content is durably on the default branch, or
which predate blob storage, plus canceled-run worktree debris. That permanent
red state is exactly why operators stopped trusting doctor and silently worked
around the few real gaps (see `AGENTS.md` → "Do not paste over a broken
runner"). This slice restores doctor's signal so that guardrail is enforceable.

Read first: `go/pkg/reads/doctor.go` (assembly — note `ok = len(problems)==0`
and the separate `warnings` channel, ~lines 187 and 212),
`go/pkg/reads/worktree_refs.go`, `go/pkg/reads/doctor_artifact_anchor.go`.

## Root cause (don't re-derive)

Both checks resolve durability **only** from the run branch + `refs/striatum/<run>/*`
pins (`durableWorktreeProbeRefs`) — **never the repository default branch** — and
they ignore run terminality and pre-blob-storage legacy. So normal post-merge
run-branch deletion (which `AGENTS.md` *encourages* — "Do not strand pushed
branches") and canceled runs generate permanent, un-actionable `problems`.
Evidence: flagged artifacts' files (e.g.
`docs/operator/artifacts/rfc-0075-and-mcp-cutover/build/HANDOFF.md`) are present
on `origin/main`; the 150 `artifact_blob_metadata_missing` are blob-placement
artifacts with an empty `blob_key` (they predate RFC 0125 blob storage).

## The P0 slice (and ONLY this)

Reclassify un-actionable integrity findings from `problems` (which reds `ok`) to
`warnings` (which do not), keeping only **genuine loss** as a problem. Three rules:

1. **Default-branch preservation.** Add the repository **default branch** as a
   durable-preservation source:
   - Worktree check: a completed-job worktree HEAD reachable from the default
     branch is preserved → emit a `warning`
     (`worktree_unanchored_on_default_branch`) instead of a
     `worktree_head_unreachable` problem. (The operator *should* still create the
     pin, so keep it visible — just not `ok`-reddening.)
   - Artifact check: an artifact whose `content_sha256` matches its `repo_path`
     content at the default-branch tip is preserved → not a problem.
2. **Terminal-run reclassification.** Worktree/artifact findings whose run is in a
   terminal state (`canceled`) are debris/archived, not active gaps → `warnings`,
   not `problems`. (Clears canceled-run worktree debris that the cleanup verbs
   cannot physically remove — `worktree release --force` refuses an undurable
   published artifact, and the worktree dir is owned by the `striatum-lane`
   sandbox user.)
3. **Legacy pre-blob-storage.** A blob-placement artifact with an empty `blob_key`
   → `warning` `artifact_legacy_unverifiable`, NOT an
   `artifact_blob_metadata_missing` problem — **unless** its content is also
   absent from the default branch / git anchor (then it is genuine loss → problem).

After this, `problems` ⇒ `ok=false` only on genuinely-unpreserved content.

## Gotchas

- **No schema change.** This is read-only diagnostic code (`go/pkg/reads/`): no
  migration, no owner-table DDL. (If you think you need one, you have over-scoped.)
- **Do NOT hardcode `"main"`.** The run row's `branch_name` is the RUN branch; you
  need the repo's *default* branch. Derive it robustly (e.g.
  `git symbolic-ref refs/remotes/origin/HEAD`, or repo config), and **degrade
  safely** if it cannot be resolved — fall back to the current behavior, never
  crash or hang the check.
- Reachability/content checks shell out to `git` (see `readGitAncestor`,
  `readGitFileBytes`, `readGitCommit`). Keep them `ctx`-cancellable and tolerant of
  a missing ref (treat as "not preserved here" and continue).
- Wire a `warnings` return from both check functions (extend their signatures) and
  append in `doctor.go` next to the existing `warnings = append(...)`. Do **not**
  change what `ok` *means* beyond moving reclassified items off `problems`.
- Keep `problem_records` / block fields coherent for verbose mode and do not break
  existing JSON consumers or `go/pkg/reads/redaction.go` (which lists `problems` /
  `doctor`). A parallel warning record is fine if it stays additive.
- **Preserve genuine-loss detection.** Content that is on NO durable ref AND NOT on
  the default branch MUST stay a `problem`. The fix must not blind doctor.

## Deliverable

- Implement in `go/pkg/reads/{worktree_refs.go,doctor_artifact_anchor.go,doctor.go}`.
- Tests in `go/pkg/reads/*_test.go`: (a) preserved-on-default-branch → warning;
  (b) terminal-run debris → warning; (c) legacy empty-`blob_key` → legacy warning;
  (d) **load-bearing safety**: an artifact/worktree whose content is on no ref and
  not on the default branch still reds `ok`. PG-gated tests use
  `STRIATUM_PG_TEST_URL` (scoped `-run <names> -p 1 -timeout 900s`).
- Add a decision-log entry in `docs/decisions/decision-log.md` (next D-number):
  "doctor integrity legibility — preserved-on-default-branch / terminal-run /
  legacy artifacts are warnings, not `ok=false` problems," explicitly tied to the
  `AGENTS.md` "Do not paste over a broken runner" guardrail (a red doctor must be
  actionable to be a usable stop-condition).
- Write `docs/campaigns/doctor-integrity-legibility/artifacts/DRAFT.md`: design
  note (edit sites, default-branch resolution approach, the warning taxonomy, what
  stays a problem) and **honest** `make -C go build` + targeted-test results (note
  which PG-gated checks you ran or skipped and why).
- Write the code onto the run branch (repo-write scope; `publish_source_changes`
  is enabled, so `work.complete` commits your in-scope edits). Do **NOT** merge to
  `main`.
