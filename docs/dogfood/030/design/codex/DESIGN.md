# RFC 0026 and RFC 0027 Implementation Design

author: designer-codex-gpt-5.5-001
date: 2026-05-11
status: draft

## Summary

RFC 0026 should ship first as byline honesty. It can be implemented inside the existing session, supervisor, artifact, verdict, evidence, and workflow-validation boundaries without changing Striatum's local-first authority model. The runner should stop treating an operator-asserted `lane_id` as proof that the lane process authored an artifact. A lane-typed author line is valid only when the session has a live runner-owned supervisor binding at the moment the runner validates the artifact or verdict. Otherwise the expected author line is an operator byline.

RFC 0027 should be staged carefully. The early pieces, such as `provenance_mode`, patch artifacts, hash-bound verdict metadata, apply eligibility checks, and unsigned receipts, are useful in advisory mode. They must not be described as sealed provenance until Striatum can establish a real local authority boundary where the operator cannot write protected source paths, cannot mutate active lane scratch workspaces, and cannot alter receipt signing material.

## Domain Model

Add these terms to `docs/UBIQUITOUS_LANGUAGE.md` before implementation code lands:

- `lane attestation`: a derived value object for a session at a point in time. It includes `status`, optional `supervisor_id`, optional `pid`, supervisor `state`, and a liveness result.
- `attested lane byline`: the current lane-typed form, for example `author: reviewer-codex-gpt-5.5-001`, valid only for a lane-attested session.
- `operator byline`: `author: operator` or `author: operator [self-declared: <label>]`, explicitly not a model-lane claim.
- `provenance mode`: workflow/run policy with values `advisory`, `attested_bylines`, and later `sealed_patch`.
- `patch artifact`: immutable captured source delta with base tree, result tree, touched paths, digest metadata, producer job/session/supervisor, and write-scope validation result.
- `hash-bound verdict`: a verdict whose review authority names an exact reviewed artifact id and digest.
- `candidate tree`, `apply gate`, and `provenance receipt`: RFC 0027 concepts that become authoritative only when sealed mode has a containment boundary.

The key distinction for docs and code is: byline honesty is not source-byte provenance. Process/supervisor liveness is not model-token provenance. Signed receipts are not meaningful against operator tamper unless signing material and protected source writes are outside the operator's writable authority.

## RFC 0026 Design

Implement lane attestation as a read helper, not a new persisted state machine. Add `LaneAttestation` and `session_lane_attestation(conn, session_id=...)` in `src/striatum/identity.py` or a small neighboring module if importing process liveness would create cycles. It should query `process_supervisors` for the session's active row in `starting` or `attached`, confirm the pid is alive with the same liveness helper used by supervision/doctor, and return `unattested` when no live binding exists. `detached` should not count as attested for new artifact/verdict authority because there is no live lane process to bind at that moment.

Extend `artifact_author_identity(...)` with explicit inputs for `attestation` and `operator_label`. For attested sessions, keep today's normalized lane byline. For unattested sessions, return `author: operator`, optionally suffixed with `[self-declared: <label>]`. The existing return shape can grow fields such as `byline_kind`, `lane_attestation`, and `operator_label` while preserving current keys for evidence export callers.

Wire the helper through both places that derive expected bylines:

- `src/striatum/db.py` work-packet assembly currently calls `artifact_author_identity` when adding `expected_artifacts[].author_line`. This should compute the attestation snapshot at claim time so the packet is honest about what the runner expects right now.
- `src/striatum/artifacts.py::expected_author_line` must recompute attestation at publish time before `validate_optional_markdown_author_line`. Publish-time truth wins. If a supervisor died after claim, the expected line becomes `author: operator` unless the job requires attestation and therefore refuses.

Add `sessions.operator_label TEXT` with a forward-only migration. `register-session --operator-label <text>` should trim and validate the label as printable single-line text with a small length cap, for example 80 characters. Labels must not contain `]`, newlines, or an `author:` prefix, and they must never be normalized into a lane-like byline. Existing databases migrate with `NULL`.

Add `require_attested_lane` to workflow validation. It should be accepted on review jobs and, if the implementers choose symmetry, at lane level as a default. My recommendation is to implement job-level first and lane-level only as inherited sugar:

- `jobs[i].require_attested_lane` must be boolean when present.
- Non-review use should be rejected in RFC 0026 V1 unless a later patch-capture phase has a producer-side gate to enforce it for build jobs too.
- A lane-level boolean, if added, should be copied into prepared job metadata only for jobs whose own value is absent.

At mutation time, `publish-artifact`, `submit-review`, and low-level `verdict` recording should call a shared guard before side effects. If `require_attested_lane` is true and the session is not attested, refuse with the existing invalid-transition path and include the recovery hint `striatum supervise start --session-id <id>`. `submit-review` needs the preflight because it otherwise publishes before recording the verdict.

## RFC 0026 Surfaces

`register-session` should return `lane_attestation` and `operator_label` in JSON. It will usually return `unattested` because supervision starts after registration; that is acceptable as long as the response makes the downgrade visible.

`claim-next` work packets should include a session-level block:

```json
{
  "lane_attestation": {
    "status": "attested",
    "supervisor_id": "sup_...",
    "state": "attached"
  }
}
```

For privacy and portability, avoid exposing pid in work packets unless existing supervisor surfaces already do. `supervise status`, `status --json`, run summary, evidence export, and the web session/job views should render the same status. Events may continue to record `lane_id` because it is an operator assertion, but event payloads and evidence renderers must not turn an unattested `lane_id` into a model byline.

The existing `artifacts.author_line` column remains the durable truth for published Markdown. Evidence export should prefer actual artifact byline when rendering artifacts, and should render verdict authors through the new attestation-aware identity helper. Historical rows are not rewritten.

## RFC 0027 Schema and Validation

Add top-level workflow fields:

```json
{
  "provenance_mode": "advisory",
  "protected_paths": ["src/", "tests/"],
  "operator_writable_paths": ["docs/rfcs/", "docs/dogfood/"]
}
```

`provenance_mode` defaults to `advisory` to preserve every current workflow. `attested_bylines` enables RFC 0026 behavior explicitly, but RFC 0026's byline downgrade should be the global default once accepted because it prevents false evidence. `sealed_patch` must validate structurally but refuse run start until the authority boundary is supported and verified.

Validate path fields as repo-relative, no absolute paths, no `..`, no `.striatum/`, and no overlap between protected and operator-writable paths. Also validate that a job with `repo_write: true` in `sealed_patch` has a worktree/scratch allocation strategy and cannot publish source modifications directly as final protected bytes.

Add artifact kind `patch` to `ALLOWED_ARTIFACT_KINDS`. Do not use Markdown front matter as the primary machine record for patch metadata. Store metadata in SQLite and optionally write a human-readable companion artifact.

Proposed tables for the advisory patch phases:

- `patch_artifacts(artifact_id PRIMARY KEY REFERENCES artifacts, run_id, producer_job_id, producer_session_id, producer_supervisor_id, base_tree, result_tree, patch_sha256, paths_json, blob_hashes_json, hunk_hashes_json, write_scope_validated, captured_at)`.
- `verdict_review_targets(verdict_id REFERENCES verdicts, reviewed_artifact_id REFERENCES artifacts, reviewed_digest, reviewed_base_tree, reviewed_result_tree)`.
- `apply_receipts(receipt_id, run_id, receipt_sha256, receipt_json, signature, public_key_id, applied_at, commit_hash)`.

When containment lands, add either a `protected_workspaces` table or a more general `provenance_authorities` table that records the authority mechanism, protected root, scratch roots, key id, support level, verification time, and whether the current platform can enforce sealed writes.

## RFC 0027 CLI and API

Ship new surfaces in stages:

- `striatum provenance status --run-id <id>` reports mode, authority status, protected paths, patch count, hash-bound review coverage, apply eligibility, and warnings.
- `striatum patch capture --session-id <id> --job-id <id> --lease-id <id> --output <path>` captures a patch from the active job worktree/scratch area. It refuses empty patches by default, out-of-scope paths, forbidden paths, missing base tree, and unattested producers when policy requires attestation.
- `striatum submit-review` and `striatum verdict` gain optional `--reviewed-artifact-id`, `--reviewed-digest`, `--reviewed-base-tree`, and `--reviewed-result-tree` arguments. In advisory mode they record metadata. In sealed apply eligibility, they are mandatory.
- `striatum apply reviewed-patch --run-id <id> --artifact-id <id>` evaluates preconditions and writes the protected tree only after containment exists.
- `striatum receipt show|verify` reads and verifies apply receipts. Verification should be available before sealed enforcement, but output must say `authority: advisory` when keys or protected writes were not isolated.

The local API and MCP wrapper should route these through the existing parser/dispatcher. The service mutation gate must treat `patch capture`, `apply reviewed-patch`, key rotation, and receipt creation as mutating verbs. Web UI should initially expose read-only provenance status and patch/receipt detail pages; mutation buttons can wait until the CLI semantics settle.

## Migration Strategy

Use forward-only migrations after current `user_version` 11.

The RFC 0026 migration is small: add nullable `sessions.operator_label`. It is safe for existing databases and requires no backfill. The baseline schema should be updated alongside the migration.

The provenance-mode migration should add nullable or defaulted run columns only if the implementation needs query speed. The workflow snapshot already stores the source of truth, so the simplest compatible path is `runs.provenance_mode TEXT NOT NULL DEFAULT 'advisory'` populated from the workflow during `run prepare` for new runs and defaulted for old runs. Existing runs become advisory.

Patch and receipt migrations add new tables without touching existing rows. If verdict review-target metadata is split into a side table, old verdicts simply have no hash-bound target and cannot satisfy sealed apply. Avoid rebuilding `verdicts` unless there is a strong query need; the side table is cleaner and keeps migration risk low.

Receipt signing and protected authority metadata should land only when a concrete containment implementation is selected. Until then, receipts should carry `authority_level: advisory` or `authority_level: apply_gate_only` so the verifier cannot overstate guarantees.

Databases with a higher `user_version` already refuse with exit code 9; keep that behavior. Older databases should migrate automatically during normal connect, matching SPEC.

## Compatibility Risks

The largest visible change from RFC 0026 is that examples and dogfood workflows that are driven manually, without `supervise start`, will publish `author: operator` instead of lane-typed bylines. That is correct but will break tests that assert exact `expected_artifacts[].author_line` in packets. Update those tests to either start a real supervisor, assert operator bylines for unattested sessions, or set `require_attested_lane` and expect refusal.

The RFC 003 and 029 workflow fixtures show common patterns: process lanes, transcript constraints, review jobs, and harness profiles, but not guaranteed supervision in every test. Do not globally set `require_attested_lane` in old fixtures without also updating the test harness to create supervisors.

`artifact_author_identity` is used by work packets, evidence export, run summaries, and possibly web views. Change it in one place, but audit every caller so an unattested verdict is not rendered as an attested lane byline through a stale call path.

Operator labels can become a new leakage vector if unbounded. Treat them as self-declared, short, single-line, and never trusted for policy decisions.

`sealed_patch` risks confusing users if early patch capture is available before containment. Every status, receipt, summary, and evidence export surface must distinguish `advisory patch provenance` from `sealed patch provenance`.

## Test Plan

Add `tests/test_lane_attestation.py` for RFC 0026:

- unattested `register-session --lane codex` returns `lane_attestation: unattested`;
- unattested publish accepts `author: operator` and rejects `author: reviewer-codex-gpt-5.5-001` with exit code 6;
- `--operator-label` renders `author: operator [self-declared: ...]`;
- a live supervisor restores the lane-typed expected byline;
- a dead supervisor downgrades or refuses according to `require_attested_lane`;
- `submit-review` with `require_attested_lane: true` refuses before publishing the finding artifact;
- evidence export and run summary render unattested verdict authors without model identity;
- event payload tests ensure no derived byline field leaks a model identity for unattested sessions.

Add workflow validator tests:

- `require_attested_lane` accepts boolean on review jobs;
- non-boolean values fail with a field path;
- non-review declaration fails unless intentionally supported;
- lane-level default behavior, if implemented, is deterministic.

Add provenance-mode and patch tests in stages:

- absent `provenance_mode` defaults to advisory;
- unknown mode fails validation;
- protected/operator-writable path overlap fails;
- `sealed_patch` start refuses when authority verification reports unsupported;
- patch capture refuses forbidden paths, out-of-scope paths, empty patches by default, missing scratch allocation, and wrong base tree;
- hash-bound verdict over digest A cannot satisfy apply for digest B;
- advisory verdict without reviewed digest records successfully but is ignored by sealed apply eligibility;
- receipt verification fails after receipt JSON, SQLite patch metadata, patch artifact, or protected tree drift;
- service mutation gate rejects new mutating provenance commands without `--allow-mutations`.

The adversarial tests matter most for RFC 0027. Include tests where the operator edits protected paths directly, modifies lane scratch, rewrites a patch file after capture, changes a verdict target digest in SQLite, and tries to apply on a different base tree. Until hard containment exists, those tests should assert refusal to claim `sealed_patch`, not pretend enforcement is present.

## Staged Delivery

1. Land RFC 0026 vocabulary, `operator_label` migration, attestation helper, byline derivation, publish/verdict guards, and status/evidence/run-summary visibility. This is independently useful and should close the observed low-friction byline forgery.
2. Add `provenance_mode` with honest surfacing everywhere and default `advisory`. This can land without patch capture and should teach users the guarantee vocabulary.
3. Add patch artifact kind, `patch capture`, patch metadata tables, and hash-bound verdict recording in advisory mode. Market this as exact review-object provenance, not sealed provenance.
4. Add apply eligibility checks and unsigned or locally signed advisory receipts. The apply command may verify all workflow preconditions but must report `authority: advisory` if it runs under the same writable operator account.
5. Choose and implement the first real containment mechanism. Only at this point should `sealed_patch` runs start successfully. Platform support can be Linux-first if macOS/Windows return explicit unsupported errors.
6. Add receipt signing with key material outside the operator writable boundary, then the narrow local signed-commit exception. Update `docs/DECISION_LOG.md` and `docs/SPEC.md` before enabling local commits, while preserving the no push/merge/rebase rule.

This sequence lets the project ship concrete evidence improvements early while keeping the product honest about what remains advisory.
