# Draft — Doctor integrity legibility P1 (reach `doctor ok=True`)

P0 (D204, landed) reclassified the un-actionable doctor integrity findings to
`warnings`, taking `striatum doctor` from ~278 `problems` to **42**. All 42 are
old historical-loss artifacts (`docs/operator/artifacts`, `docs/dogfood/058`,
`docs/issues/2x`, `ace-graduation`, `rfc-0110-pg-auth` cross-exams, etc.). This
P1 slice takes doctor from `ok=false` (42 actionable-but-historical problems) to
**`ok=true`** by adding the two preservation signals P0 deliberately left out,
plus a curated baseline for genuinely-lost-but-immaterial content. The goal: a
*green* doctor means "trust it," and ANY future real integrity gap flips it red
visibly — which is the whole point of the `AGENTS.md` "Do not paste over a broken
runner" guardrail.

Read first: `go/pkg/reads/doctor_artifact_anchor.go`,
`go/pkg/reads/worktree_refs.go`, `go/pkg/reads/doctor.go` (assembly — `ok =
len(problems)==0`, separate `warnings` channel), and the existing tests
`go/pkg/reads/doctor_integrity_legibility_test.go`. The P0 helpers you are
extending: `artifactContentMatchesRef` / `artifactContentPreserved` /
`resolveDefaultRefCached` / `readGitDefaultBranchRef` (doctor_artifact_anchor.go
+ worktree_refs.go).

## The operator-verified split of the 42 (don't re-derive — this is the spec)

Each of the 42 problem artifacts falls into exactly one class, by what doctor can
*prove* about its recorded `content_sha256` vs the repository default branch:

| Class | Count | What is true | This slice does |
|---|---|---|---|
| **Recoverable** | 14 | `content_sha256` matches a revision of `repo_path` in default-branch **history** (merged, then the path was deleted/edited later) | **Rule A** → fully clean |
| **Superseded** | 12 | `repo_path` is **live on the default-branch tip** but with *different* content (the lane draft was revised before merge; the recorded sha is an intermediate draft) | **Rule B** → `artifact_superseded_on_default_branch` warning |
| **Genuine loss** | 16 | `repo_path` is **absent from the default branch entirely** (never merged / fully removed) and content is on no durable ref | **Rule C** → `artifact_acknowledged_loss` warning *iff* curated; else stays a `problem` |

After this slice: 14 clean + 12 superseded-warning + 16 acknowledged-warning =
all 42 off `problems` → `ok=true`, with the warning channel still telling the
full story.

## Rule A — default-branch *history* awareness (clears 14)

Today `artifactContentMatchesRef` (doctor_artifact_anchor.go) checks the blob
sha256 at the **tip** of a ref only. Extend preservation to the default branch's
**history**: the content is durably preserved if `content_sha256` equals the
sha256 of `repo_path`'s blob at **any** reachable revision of the default branch.

- Add a helper, e.g. `artifactContentInDefaultRefHistory(ctx, repoRoot,
  defaultRef, repoPath, expectedSHA) bool`:
  - `git -C <repoRoot> log <defaultRef> --format=%H -- <repoPath>` → the commits
    that touched that path (cheap; bounded to that path's revisions).
  - For each commit, read the blob at `repoPath` (reuse `readGitBlobSHA256` /
    `readGitFileBytes`) and compare sha256 to `expectedSHA`; return true on first
    match.
  - **Bound it**: cap the number of revisions scanned (e.g. 200) so pathological
    histories can't blow up a doctor pass; `ctx`-cancellable; **safe-degrade to
    `false`** on any git error.
  - **Memoize** within the pass: a `map[string]bool` keyed by
    `repoRoot|defaultRef|repoPath|expectedSHA`, threaded like `defaultRefByRoot`
    is. (Different artifacts touch different paths, so the cache mostly prevents
    repeat work on hash-identical rows.)
- Wire it in:
  - `checkArtifactAnchor`: after the existing **tip** match (the `defaultRef != ""
    && artifactContentMatchesRef(...)` block, ~line 227) and **before** the
    terminal-debris / fileFound / missing-file verdicts, add: if
    `artifactContentInDefaultRefHistory(...)` → return clean (preserved in
    history). This must run before the `fileFound` → `hash_mismatch` branch (the 3
    current hash-mismatch artifacts are recoverable via history).
  - `artifactContentPreserved` (used by the empty-`blob_key` legacy path in
    `checkBlobExhaustArtifact`): also consult history, so legacy blob artifacts
    whose content is in history become `artifact_legacy_unverifiable` warnings (as
    P0 intended) rather than `artifact_blob_metadata_missing` problems.

## Rule B — superseded-on-default-branch (clears 12)

When an artifact is NOT preserved (no exact content match at tip or in history)
but its `repo_path` **still exists on the default-branch tip** (any content), the
deliverable landed at that path — only the recorded *draft* sha is unverifiable.
That is not an active durability gap; emit a **warning**, not a problem.

- Add a shared warning code `artifact_superseded_on_default_branch` (const beside
  `artifactLegacyUnverifiable` / `artifactDebrisTerminalRun`).
- Add a helper `pathExistsOnRef(ctx, repoRoot, ref, repoPath) bool` (e.g.
  `git cat-file -e <commit>:<repoPath>`, `ctx`-cancellable, safe-degrade false).
- Wire into both genuine-loss decision points, AFTER Rule A and the existing
  terminal-debris check, BEFORE the genuine-loss problem:
  - `checkArtifactAnchor`: before the final `missing_file` / `hash_mismatch`
    problem.
  - `checkBlobExhaustArtifact`: in the empty-`blob_key`, not-preserved,
    not-terminal-debris branch, before `artifactBlobProblem(...)`.
- Use `artifactWarning(artifactSupersededOnDefaultBranch, row, detail,
  defaultRef)` with a message like "deliverable is present at <repo_path> on the
  default branch in a revised form; the recorded draft content_sha256 is not
  verifiable but this is not a durability gap." Extend `artifactWarning`'s switch
  with the new code's message.

## Rule C — acknowledged_loss baseline (clears the 16 genuine losses *iff* curated)

For artifacts that are genuinely lost (path absent from the default branch, no
content anywhere) the daemon cannot prove preservation — but the operator can
record that a specific, reviewed loss is **acknowledged** (immaterial superseded
dogfood content). Add a curated baseline the doctor consults at the genuine-loss
decision point; a matching, sha-bound entry downgrades the problem to a
`artifact_acknowledged_loss` warning. **Anything not in the baseline still reds
`ok`** — this is the load-bearing safety property.

- New file `go/pkg/reads/doctor_acknowledged_loss.go`:
  - Reads a tracked JSON file at `<repo_root>/docs/operator/doctor-acknowledged-loss.json`.
    Schema `striatum.doctor.acknowledged_loss.v1`:
    ```json
    {
      "schema": "striatum.doctor.acknowledged_loss.v1",
      "entries": [
        {
          "artifact_id": "art_...",
          "repo_path": "docs/issues/22/build/HANDOFF.md",
          "content_sha256": "<hex>",
          "reason": "Superseded dogfood draft; never merged to main. Immaterial historical loss.",
          "acknowledged_by": "operator-<role>-<model>-<ordinal>",
          "acknowledged_at": "2026-06-16"
        }
      ]
    }
    ```
  - Loads once per `repo_root` (memoized like `defaultRefByRoot`), returning a
    lookup keyed by `artifact_id`. **Safe-degrade**: a missing or unparseable file
    → empty set (no downgrades), recorded as a block field (e.g.
    `acknowledged_loss_status: "absent"|"loaded"|"parse_error"`), never an error
    that aborts the check.
  - Honor an entry ONLY if `entry.content_sha256 == row.content_sha256` (binds the
    acknowledgment to the exact lost content, so a stale/wrong entry can never mask
    a *different* future problem at the same artifact id). On id-match but
    sha-mismatch: do NOT downgrade (stays a problem).
- Add warning code `artifact_acknowledged_loss`. Wire into both genuine-loss
  decision points, AFTER Rule B, immediately BEFORE the genuine-loss problem: if
  the ack set honors this row → `artifactWarning(artifactAcknowledgedLoss, row,
  reason, "")` (carry `reason` + `acknowledged_by` into the record context); else
  the existing problem.
- **Do NOT author the real baseline file in this dogfood.** The operator curates
  and commits `docs/operator/doctor-acknowledged-loss.json` (the 16 reviewed
  entries) as a separate provenance step after merge+deploy, having verified the
  real residual against the deployed code. Your job is the *reader + schema +
  fixture + tests*. You MAY add a `go/pkg/reads/testdata/` fixture for tests, but
  leave the live baseline file to the operator (the reader must safe-degrade when
  it is absent).

## Check ordering (both `checkArtifactAnchor` and the blob genuine-loss branch)

1. Preserved at default-branch **tip** (exact content) → clean. *(existing)*
2. **Rule A**: preserved in default-branch **history** (exact content) → clean.
3. Terminal-debris run (`canceled`/`failed`) → `artifact_debris_terminal_run`
   warning. *(existing)*
4. **Rule B**: `repo_path` exists on default-branch tip → `artifact_superseded_on_default_branch`
   warning.
5. **Rule C**: artifact_id in the sha-bound ack baseline → `artifact_acknowledged_loss`
   warning.
6. else → genuine-loss `problem` (`artifact_anchor_hash_mismatch` /
   `artifact_anchor_missing_file` / `artifact_blob_metadata_missing`). *(existing)*

## Gotchas

- **No schema change, no migration, no new RPC** (read-only diagnostic in
  `go/pkg/reads/`). If you think you need any of those, you have over-scoped.
- **Do NOT hardcode `"main"`** — reuse `resolveDefaultRefCached` /
  `readGitDefaultBranchRef`.
- Every new git shell-out must be `ctx`-cancellable and treat a missing
  ref/path/blob as "not preserved/not present here", never crash or hang.
- Keep `problem_records` / `warning_records` / block fields coherent for verbose
  mode and don't break `go/pkg/reads/redaction.go` or other JSON consumers — keep
  new fields additive.
- **Preserve genuine-loss detection.** An artifact whose content is on no ref, not
  in default-branch history, whose path is absent from the default branch, and
  which is NOT in the ack baseline MUST stay a `problem`. The fix must not blind
  doctor.

## Deliverable

- Code: `go/pkg/reads/doctor_artifact_anchor.go`,
  `go/pkg/reads/doctor_acknowledged_loss.go` (new),
  `go/pkg/reads/doctor.go` if assembly needs it. (Rule B/C apply to the artifact
  checks; the worktree check is unchanged unless you find a parallel gap.)
- Tests in `go/pkg/reads/*_test.go` (use `requireGit` + the `readsGit*` helpers
  and the fake runners already in `doctor_integrity_legibility_test.go`):
  1. **Rule A**: an artifact whose `content_sha256` matches a *historical* (not
     tip) revision of its path on the default branch → clean (no problem, no
     warning).
  2. **Rule B**: path present on default tip with *different* content → exactly one
     `artifact_superseded_on_default_branch` warning, no problem.
  3. **Rule C accept**: a genuine-loss artifact (path absent from default branch)
     whose id+sha are in a fixture baseline → one `artifact_acknowledged_loss`
     warning, no problem.
  4. **Rule C safety (load-bearing)**: the SAME genuine-loss artifact NOT in the
     baseline → still a `problem` (reds `ok`). And id-match-but-sha-mismatch →
     still a `problem`.
  5. Baseline reader safe-degrades: absent file → empty set, no panic, no
     downgrades.
  Run them with `STRIATUM_PG_TEST_URL=postgres://halbritt@/postgres?host=/var/run/postgresql`,
  scoped `-run <Names> -p 1 -timeout 900s`, and run `-v` so you can confirm the
  named tests actually executed (no no-match false-pass).
- Decision-log entry in `docs/decisions/decision-log.md` (next D-number, D205):
  "doctor integrity legibility P1 — default-branch-history preservation,
  superseded-on-default-branch warning, and a curated acknowledged_loss baseline
  take doctor to `ok=true`," referencing D204 and the `AGENTS.md` "Do not paste
  over a broken runner" guardrail; record that the live 16-entry baseline is
  operator-curated provenance committed separately.
- `docs/campaigns/doctor-integrity-legibility-p1/artifacts/DRAFT.md`: design note
  (edit sites, the three rules, ack-file schema + safe-degrade, check ordering)
  and **honest** `make -C go build` + targeted-test results (name which tests ran
  / were skipped and why).
- Write the code onto the run branch (repo-write scope; `publish_source_changes`
  is enabled, so `work.complete` commits your in-scope edits). Do **NOT** merge to
  `main`.
