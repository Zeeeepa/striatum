---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/030/design/codex/DESIGN.md", "docs/dogfood/030/design/claude_code/DESIGN.md"]
---

# RFC 0026 and RFC 0027 Implementation Synthesis

author: designer-codex-gpt-5.5-004
date: 2026-05-11
status: draft

## Accepted Implementation Scope

RFC 0026 and RFC 0027 should ship as one provenance program with two different guarantee levels. RFC 0026 is the immediate honesty layer: Striatum must stop rendering an operator-asserted lane as a model byline unless a live runner-owned supervisor binding attests that lane session. RFC 0027 is the longer source-byte provenance layer: patch identity, review digest binding, apply checks, receipts, and, later, hard local containment.

Accepted implementation scope:

- Implement lane attestation as a derived session property from `process_supervisors`: a session is attested only when it has an active `starting` or `attached` supervisor row and the recorded pid is alive.
- Change author derivation so unattested sessions expect and render `author: operator`, or `author: operator [self-declared: <label>]` when an explicit self-label was registered.
- Add nullable `sessions.operator_label TEXT`, `register-session --operator-label`, conservative single-line label validation, and JSON/status visibility for the label.
- Add job-level `require_attested_lane` for review jobs in V1, enforced before artifact or verdict side effects in `publish-artifact`, `verdict`, and `submit-review`.
- Add workflow-level `provenance_mode` with closed values `advisory`, `attested_bylines`, and `sealed_patch`; absent mode defaults to `advisory`.
- Surface provenance mode and lane attestation in work packets, `register-session`, `status`, `why`, `dashboard`, run summary, evidence export, doctor, and web views.
- Add advisory RFC 0027 primitives: `patch` artifact kind, patch metadata table, patch capture, and hash-bound verdict review-target metadata.
- Add apply eligibility and receipt verification scaffolding only with explicit authority labels such as `advisory` or `apply_gate_only` until hard containment exists.

The invariant is explicit: byline honesty is not source-byte provenance; a live supervisor is not proof of model-token authorship; patch digests are not sealed provenance; signatures are not tamper-resistant while the operator can write the source tree, signing material, runner code, or SQLite state.

## Deferred Scope

Deferred scope:

- Model-token authorship proof. D028's no-broad-transcript-capture posture means Striatum cannot prove that artifact bytes came from a model token stream.
- Defense against local root, a modified Striatum binary, malicious dependencies, or direct `.striatum/state.sqlite3` tamper by an operator with full local authority.
- Issue #3 retraction: compromised run state, historical artifact/verdict retraction, and retraction-aware read surfaces.
- Hosted signing, transparency logs, Sigstore, TEEs, provider token attestations, watermarking, or cross-machine trust.
- macOS and Windows sealed containment until a concrete hard write-denial design exists and is tested.
- Web mutation buttons for patch capture, apply, key management, or receipt creation.
- A new `verification` job type. Initial apply eligibility should reuse existing build/review gates and add a type only if later workflow vocabulary proves it necessary.
- A legacy byline bypass. This synthesis chooses immediate honesty over preserving the low-friction false-byline path RFC 0026 exists to close.

## Reconciled Design Choices

The Codex design is stronger on low-risk migrations, publish-time byline checks, and keeping early RFC 0027 work advisory. The Claude Code design is stronger on mode vocabulary, negative guarantees, and adversarial containment tests. The implementation plan should combine those strengths.

First, job-level `require_attested_lane` should land before lane-level inheritance. Review jobs mint verdict authority, so their semantics are clear. Build-job producer gates belong with patch capture because they are about accepting source deltas, not merely rendering review bylines.

Second, verdict digest binding should use a side table rather than nullable columns on `verdicts`. Old verdicts then naturally have no reviewed object and cannot satisfy sealed apply eligibility, without rebuilding a central table.

Third, `sealed_patch` must remain unsupported until Striatum can demonstrate operator write denial. Patch capture, digest-bound reviews, apply checks, and receipts are valuable before that, but every surface must label them advisory or apply-gate-only.

Fourth, publish-time truth should win over claim-time snapshots. Work packets may show the attestation state observed at claim time, but `publish-artifact` and verdict recording must recompute attestation at the mutation boundary. If a supervisor dies after claim, publish downgrades to `author: operator` or refuses when the job requires attestation.

## Phased Plan

Phase 1: RFC 0026 byline honesty.

Add `LaneAttestation`, `session_lane_attestation(conn, session_id=...)`, attestation-aware `artifact_author_identity`, publish-time `expected_author_line`, `operator_label`, and review-job `require_attested_lane`. Update `register-session`, work packets, status, evidence export, run summary, doctor, and web read views.

Milestone: an unattested Codex review session can publish only with `author: operator`; a lane-typed author line fails through existing author-line validation; a live attached supervisor restores the lane-typed byline; `require_attested_lane` refuses before side effects.

Phase 2: provenance mode surfacing.

Add `provenance_mode` validation and shared guarantee descriptions. Existing workflows default to `advisory`. `sealed_patch` may validate structurally, but `run start` must refuse it unless an authority probe reports real containment support.

Milestone: every introspection and export surface names the mode, and unsupported `sealed_patch` runs stop before work starts.

Phase 3: advisory patch objects and hash-bound reviews.

Add `patch` to allowed artifact kinds, add `patch_artifacts`, add `verdict_review_targets`, implement `patch capture`, and add review-target flags to `verdict` and `submit-review`. Capture validates base tree, non-empty diff unless explicitly allowed, write scope, forbidden paths, scratch/worktree ownership, and producer attestation when policy requires it.

Milestone: a verdict over patch digest A cannot satisfy eligibility for patch digest B, and evidence export includes patch metadata while still saying authority is advisory.

Phase 4: apply eligibility and receipts without sealed claims.

Add `striatum apply reviewed-patch`, `striatum provenance status`, `striatum provenance verify`, and receipt records. These checks are useful before containment, but output must say `authority: advisory` or `authority: apply_gate_only` when source writes and keys are not isolated.

Milestone: apply refuses each missing precondition with a precise error; receipt verification detects patch, tree, and receipt drift; no surface describes the result as sealed.

Phase 5: sealed containment and signed local commit.

After a human chooses the first containment mechanism, allow `sealed_patch` runs to start only on supported platforms/configurations. Then isolate protected paths, lane scratch, and signing material. Enable the narrow sealed-mode local signed-commit exception only after `docs/DECISION_LOG.md` and `docs/SPEC.md` are updated. Striatum still must never push, merge, rebase, or rewrite history.

Milestone: a supported sealed fixture proves operator write denial, lane scratch authoring, hash-bound review, gated apply, receipt verification, and local signed commit creation without remote publication.

## Schema And Migration Changes

Phase 1:

- Add nullable `sessions.operator_label TEXT`.
- Update the baseline schema and migration registry.
- Do not change `process_supervisors`; attestation is derived.

Phase 2:

- Add `provenance_mode` to workflow validation with default `advisory`.
- Use the workflow snapshot as source of truth. Add `runs.provenance_mode TEXT NOT NULL DEFAULT 'advisory'` only if status/query paths need denormalization.
- Add `protected_paths` and `operator_writable_paths` validation for `sealed_patch`: repo-relative only, no absolute paths, no `..`, no `.striatum/`, and no overlap.

Phase 3:

- Add `patch` to `ALLOWED_ARTIFACT_KINDS`.
- Add `patch_artifacts(artifact_id, run_id, producer_job_id, producer_session_id, producer_supervisor_id, base_tree, result_tree, patch_sha256, paths_json, blob_hashes_json, hunk_hashes_json, write_scope_validated, captured_at)`.
- Add `verdict_review_targets(verdict_id, reviewed_artifact_id, reviewed_digest, reviewed_base_tree, reviewed_result_tree)`.

Phase 4 and 5:

- Add receipt support as either a `receipt` artifact kind or an `apply_receipts` table after the receipt format is fixed.
- Add containment authority metadata only when a mechanism is chosen: platform, mechanism, protected root, scratch roots, key id, support status, probe time, and failure reason.

All migrations stay forward-only under the existing `PRAGMA user_version` system. Older databases migrate automatically; newer databases continue to refuse older runners with exit code 9.

## CLI And API Changes

Modified commands:

- `register-session --operator-label <label>` returns `lane_attestation` and `operator_label`; unattested JSON remains valid and any hint goes to stderr.
- `claim-next` includes a `lane_attestation` block in work packets.
- `publish-artifact` recomputes the expected author line at publish time.
- `verdict` and `submit-review` enforce `require_attested_lane` before recording artifacts or verdicts.
- `status`, `why`, `dashboard`, `run summary`, `evidence export`, `doctor`, and web views show provenance mode and attestation state.

New commands:

- `striatum patch capture --session-id <s> --job-id <j> --lease-id <l> ...`
- `striatum provenance status --run-id <run>`
- `striatum provenance verify --run-id <run>` and later `--receipt-file <path>`
- `striatum apply reviewed-patch --run-id <run> --artifact-id <patch_artifact>`
- `striatum keys init|rotate|export-public` only when receipt signing lands
- Optional sealed-mode read helpers: `striatum source read` and `striatum source grep`

The local API and MCP wrapper should inherit these through the existing parser/dispatcher. The service mutation gate must classify patch capture, apply, key management, and receipt creation as mutating commands.

## Artifact, Verdict, Evidence, Status, Doctor, Web, Docs, And Fixture Changes

Artifacts:

- Markdown artifacts with author lines use publish-time expected author derivation.
- `artifacts.author_line` remains the actual file truth; historical rows are not rewritten.
- Patch metadata is stored in SQLite and may have a human-readable companion file. Markdown front matter is not the primary machine record for patches.

Verdicts:

- Verdict displays must not reconstruct model bylines from unattested `lane_id` values.
- If record-time attestation is snapshotted, it is audit-only and must not render as authorship proof.
- Hash-bound verdicts must name the exact reviewed patch artifact and digest before they can satisfy apply eligibility.

Evidence:

- Evidence export includes provenance mode, attestation state, actual artifact bylines, patch digests, review-target bindings, and receipts.
- Each mode includes clear "does not prove" text, especially that `attested_bylines` still permits direct source edits and `sealed_patch` still does not prove model-token authorship.

Status and doctor:

- `status --json` exposes run `provenance_mode`, per-session attestation, and patch/apply readiness.
- `doctor` warns for unattested sessions, unsupported sealed mode, writable protected paths, writable active scratch, signing keys inside operator authority, and stale/lost supervisor rows.

Web and docs:

- Web UI starts read-only: mode chips, attestation labels, patch and receipt detail pages, and doctor warnings.
- Update `SPEC.md`, `UBIQUITOUS_LANGUAGE.md`, `DECISION_LOG.md`, RFC 0026, RFC 0027, `CLI_REFERENCE.md`, `HOW_TO_AGENT.md`, `HOW_TO_HUMAN.md`, README, and `CHANGELOG.md` as each implementation phase lands.
- Add a decision-log row before enabling sealed-mode local signed commits. The carve-out is local signed commit only; no push, merge, rebase, or history rewriting.

Fixtures:

- Update examples and dogfood workflows that assert exact bylines. Tests should either start a real supervisor, expect `author: operator`, or set `require_attested_lane` and assert refusal.
- Add one advisory-mode fixture, one attested-byline fixture, and one sealed-mode fixture that is skipped or refused on unsupported platforms.

## Compatibility And Upgrade Risks

The largest break is intentional: manually driven unattested sessions can no longer publish lane-typed bylines. Existing tests and fixtures that register sessions without `supervise start` and write `author: <role>-<model>-<ordinal>` must change.

`artifact_author_identity` has broad reach. The implementation must audit work-packet assembly, artifact publishing, verdict rendering, evidence export, run summary, web views, and tests so no stale path still treats `sessions.lane_id` as attested model identity.

Operator labels can leak sensitive text if unbounded. Keep them short, ASCII, single-line, and self-declared. They must never affect policy.

`sealed_patch` has high overclaim risk. Patch capture, digest-bound reviews, apply checks, and receipts remain advisory until the operator cannot write protected source, active lane scratch, or signing key material.

Cross-platform risk should be handled with explicit unsupported errors. Silent downgrade from `sealed_patch` to `advisory` is a correctness bug.

## Test Matrix

Lane attestation:

- Unsupervised session registers with `lane_attestation: unattested`.
- Unattested artifact with `author: operator` publishes.
- Unattested artifact with lane-typed author fails with exit code 6.
- Live supervisor changes expected byline to lane-typed.
- Dead supervisor downgrades to operator or refuses when required.
- `operator_label` validates and renders as self-declared.
- `require_attested_lane` refuses `publish-artifact`, `verdict`, and `submit-review` before side effects.
- Evidence export and run summary never display unattested verdicts as model bylines.

Workflow validation:

- Unknown `provenance_mode` fails.
- Absent mode defaults to advisory.
- `sealed_patch` requires valid protected/operator-writable paths.
- Path overlap, absolute paths, traversal, and `.striatum/` protection fail.
- `require_attested_lane` accepts boolean on review jobs and rejects invalid types and unsupported job types.

Patch and review binding:

- Patch capture refuses empty diffs by default.
- Patch capture refuses out-of-scope and forbidden paths.
- Patch capture records deterministic digest metadata.
- Re-capture after mutation creates a new digest.
- Verdict over digest A does not satisfy eligibility for digest B.
- Verdict without reviewed digest records in advisory mode but cannot satisfy sealed apply.

Apply and receipt:

- Apply refuses unreviewed patch, wrong digest, wrong base tree, open blocker, failed dependency, missing required review, missing required attestation, and unsupported authority.
- Receipt verification fails after patch substitution, protected-tree drift, receipt mutation, and SQLite metadata tamper when signature material is outside operator authority.
- Evidence export includes enough patch and receipt metadata to verify from a fresh checkout or exported bundle.

Containment:

- `sealed_patch` run refuses when protected paths are writable by the operator.
- Operator shell cannot write protected source paths during an active sealed run.
- Operator shell cannot write active lane scratch.
- Lane process can write its allocated scratch.
- Apply gate writes protected source through Striatum.
- Unsupported platforms fail loudly rather than silently running advisory semantics.

## Staging Plan To Avoid Overclaiming

The release sequence should make guarantee level visible before advanced provenance features land:

1. Ship byline honesty and attestation gates.
2. Ship `provenance_mode` surfacing and exact mode descriptions.
3. Ship advisory patch capture and hash-bound reviews.
4. Ship apply eligibility and receipts as `advisory` or `apply_gate_only`.
5. Ship the first containment mechanism and only then allow `sealed_patch` runs to start.
6. Ship sealed-mode signed local commits after the decision-log and SPEC carve-out is accepted.

Each phase needs a negative test proving it does not overclaim the next phase. Patch capture tests should prove `sealed_patch` still refuses without containment; receipt tests should prove signatures inside operator authority are labeled advisory.

## Human-Decision Questions

1. Should the owner accept the no-legacy-bypass path for RFC 0026? This synthesis recommends yes.
2. Should `provenance_mode` be stored only in workflow snapshots or denormalized onto `runs`? This synthesis recommends snapshot as source of truth, with denormalization only for query simplicity.
3. Should verdict events snapshot `attested_at_record_time` for audit? This synthesis recommends yes, provided renderers do not present it as authorship proof.
4. Which containment mechanism should be first: `bwrap`, separate Unix users, POSIX ACLs, macOS sandboxing, or a local apply service? This must be decided before Phase 5.
5. Is Linux-only sealed support acceptable for the first real sealed mode? This synthesis recommends yes if CI can prove hard write denial; macOS and Windows should fail explicitly until supported.
6. Where should receipts live: ordinary artifact rows with durable JSON files, Git notes, commit trailers, or multiple forms? This synthesis recommends ordinary artifact rows/files first.
7. What local signed-commit identity and key management should Striatum use? This must be decided before changing the current no-automatic-commit boundary.
8. Should sealed apply require a human `decision record`, or are accepting review verdicts sufficient? This synthesis recommends not gating V1 apply on decision artifacts unless the owner wants apply to become an explicit human acceptance act.
