---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/040/design/codex/DESIGN.md", "docs/dogfood/040/design/claude_code/DESIGN.md", "docs/dogfood/040/design/gemini/DESIGN.md"]
---

author: designer-codex-gpt-5.5-001

# RFC 0040 Design Synthesis

## Accepted Implementation Scope

Acceptance criterion 1, every dogfood-lifecycle RPC method has an MCP chat-tool entry: `implement_ergonomics_claude` owns `src/striatum/web/chat_tools.py`, `tests/test_chat_tools.py`, and `tests/test_web_chat.py`. Add thin entries for `run_prepare`, `run_start`, `register_session`, `supervise_start`, `claim_next`, `ack`, `publish_artifact`, `verdict`, `complete`, `run_summary`, `evidence_export`, and `supervise_stop`. Visibility is derived from daemon RPC registry capability metadata and every call routes through the same audited daemon/RPC invocation path when daemon mode is configured.

Acceptance criterion 2, `dogfood.publish_on_behalf`: `implement_systems_codex` owns `src/striatum/dogfood/operator_tools.py`, registry entries, route wiring, and `tests/test_dogfood_publish_on_behalf.py`. It composes ack, publish, optional verdict, and complete in one transaction and writes one composite audit row.

Acceptance criterion 3, `dogfood.surgical_recovery`: `implement_systems_codex` owns `src/striatum/dogfood/operator_tools.py`, `src/striatum/daemon_rpc/registry.py`, `src/striatum/daemon_rpc/capability.py`, any daemon PG capability constraints/migrations, route wiring, and `tests/test_dogfood_surgical_recovery.py`. It restores one stale repo-write job only after strict preconditions pass.

Acceptance criterion 4, supervised-progress heartbeat: `implement_systems_codex` owns `src/striatum/daemon_supervisor/progress_watcher.py`, daemon supervisor integration, `tests/test_supervised_progress_watcher.py`, and relevant RFC 0035 harness integration. The watcher refreshes leases only through the normal heartbeat transition.

Acceptance criterion 5, generator emits new harness fragments: `implement_ergonomics_claude` owns `src/striatum/workflow_generator/`, `src/striatum/workflow_templates/`, and `tests/test_workflow_generator.py`.

Acceptance criterion 6, backport existing workflows: `implement_ergonomics_claude` owns `striatum workflow upgrade <path>` in `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py`, `src/striatum/cli/workflow.py`, a new `src/striatum/workflow_generator/upgrade.py`, and `tests/test_workflow_upgrade.py`.

Acceptance criteria 7 through 10, docs, capability, RFC 0035 tests, and no regression: docs are owned by `implement_ergonomics_claude`; capability and systems tests by `implement_systems_codex`; both implementers report focused test output in their handoffs, and `implement_ergonomics_claude` writes the combined `docs/dogfood/040/BUILD_HANDOFF.md`.

## Deferred Scope

Operator-side automation beyond the two composite tools is deferred. RFC 0040 removes ID-copying friction but does not auto-drive a full dogfood; that remains an operator/session choice over audited tools.

Self-healing supervised wrappers are deferred. The one-shot no-questions fragments reduce the recurring failure mode; wrapper retries or adaptive prompts need their own RFC after watcher/composite telemetry exists.

The Go-daemon backport is deferred to RFC 0039 implementation. This plan keeps the Python daemon surface language-agnostic so the Go daemon can mirror it later.

Hosted-mode MCP remains out of scope under D083 and RFC 0030/0032. All tools are local, owner-only, token-gated, and metadata-audited.

## MCP Chat-Tool Surface

Ship per-RPC lifecycle tools plus the two composite tools. Do not replace the per-RPC tools with only composites; operators still need structured access to the ordinary lifecycle.

| Chat tool | RPC method | Capability |
|---|---|---|
| `run_prepare` | `run.prepare` | `write` |
| `run_start` | `run.start` | `write` |
| `register_session` | `session.register` | `write` |
| `supervise_start` | `supervise.start` | `write` |
| `claim_next` | `claim_next` | `claim` |
| `ack` | `ack` | `write` |
| `publish_artifact` | `publish_artifact` | `write` |
| `verdict` | `verdict` | `review` |
| `complete` | `complete` | `write` |
| `run_summary` | `run.summary` | `read` |
| `evidence_export` | `evidence.export` | `read` |
| `supervise_stop` | `supervise.stop` | `write` |
| `dogfood.publish_on_behalf` | `dogfood.publish_on_behalf` | `write`, or `review` when verdict supplied |
| `dogfood.surgical_recovery` | `dogfood.surgical_recovery` | `surgical_recovery` |

If the registry cannot express dynamic capability for `publish_on_behalf`, register the V1 method as `review`; that is conservative and may be relaxed later. Every allowed or denied tool call appends the existing daemon audit/request-log row with `transport: "mcp"` or the equivalent chat transport value. The chat layer must preserve structured denials instead of converting them to generic strings.

## Composite Tool: `dogfood.publish_on_behalf`

Input schema: `session_id`, `artifact_path`, `artifact_kind`, `logical_name`, optional `verdict`, optional `findings_artifact_id`, optional `verdict_rationale`, and required non-empty capped `reason`.

Output schema: `operation`, `session_id`, `job_id`, `lease_id`, `message_id`, `artifact_id`, optional `verdict_id`, `composition_steps`, and `reason`.

Lookup flow: find exactly one active lease for `session_id` on a non-terminal job; find the queue message for that lease/job; if claimed but unacked, ack it; if already acked, continue; otherwise deny. Then publish the declared artifact, record the verdict when supplied, and complete the job when the existing review transition did not already do so. Validate path and `(kind, logical_name)` against the work packet/expected artifact contract before publishing.

Audit metadata: a single row with `operation: "publish_on_behalf"`, `operator_reason`, ids for session/job/lease/message/artifact/verdict, and `composition_steps` entries for `ack`, `publish_artifact`, optional `verdict`, and `complete`. Do not store artifact contents or model output.

Denial vocabulary: `no_active_lease` when no exactly-one active lease exists; `lease_busy` when the message/lease/job is not in the recoverable claimed or already-acked shape; existing validation errors for path, kind, logical name, and verdict failures.

## Composite Tool: `dogfood.surgical_recovery`

Input schema: `job_id`, required non-empty capped `reason`, `extend_lease_seconds` default `900`, `confirm_write: true`, and the UI/operator confirmation marker used by the chat mutation gate.

Output schema: `operation`, `job_id`, `session_id`, `lease_id`, `message_id`, optional `supervisor_id`, `new_expires_at`, `validated_artifact_paths`, `composition_steps`, and `reason`.

Validation flow: load job, run, session, last lease, queue message, expected artifacts, and supervisor pointer/row in one transaction. Refuse terminal jobs, non-stale jobs, missing expected artifact files, artifact paths outside write scope, live attached supervisors, concurrent supervisors, pid identity mismatch when attestation would be restored, and empty reasons. "Supervisor not alive" is required; if a process is still alive, ordinary heartbeat or operator stop/lost handling should happen first.

Atomic transaction: reactivate the expired lease with `expires_at = now + extend_lease_seconds`, restore the queue message to the post-ack claimed shape, restore the job to `running` with the current lease, and move the matching supervisor/pointer from `lost` to `attached` only when pid identity is verified. All steps commit together or roll back together.

Audit metadata: one row with `operation: "surgical_recovery"`, `operator_reason`, before/after lease/message/job/supervisor state, `validated_artifact_paths`, and `composition_steps` for `lease_reactivate`, `queue_message_restore`, `job_state_restore`, and optional `supervisor_reattach`.

Denial vocabulary: `capability_missing` for missing token authority; `surgical_recovery_validation_failed` for any precondition failure, with structured details naming the specific failed check.

## New `surgical_recovery` Capability

Add `surgical_recovery` to the RFC 0030 closed capability vocabulary and registry. It is admin-only in product posture and should be issued as a short-lived token; 15 minutes is the recommended maximum. It is distinct from the existing `recovery` capability because it bypasses the normal repo-write stale-lease refusal. The method is repository-scoped to one job/repo, but the capability is privileged and should be granted only deliberately by an admin operator.

## Operator-Confirmation Semantics

`dogfood.publish_on_behalf` does not require `confirm_write` or a UI gesture. Its required `reason` is the operator confirmation, and its authority is equivalent to the underlying ack/publish/verdict/complete sequence.

`dogfood.surgical_recovery` requires `confirm_write: true` and a separate UI/operator gesture. It bypasses ordinary recovery policy and changes stale lease/supervisor state, so a double-confirm gate is appropriate.

Thin lifecycle tools do not require `confirm_write`; they inherit the CLI/RPC semantics and token capability boundary. `workflow upgrade` is a CLI command with dry-run semantics rather than a chat UI gesture.

## Audit-Chain Shape for Composite Operations

Use a single audit row with `composition_steps` metadata for each composite operation. This matches the operator's mental model, preserves atomicity, and still gives reviewers a structured sub-step trace. Do not emit decomposed audit rows linked by `composition_id` in V1.

## Supervised-Progress Watcher

Implement one daemon-owned watcher task per attached supervisor. Poll every 30 seconds. If the newest relevant `.log` file under the supervisor scratch path has `mtime` within the last 60 seconds, refresh the session's active lease through the normal heartbeat transition. If no progress is observed for 600 seconds, emit a metadata-only warning and let normal lease expiry/recovery policy proceed.

Cleanup semantics: start on supervisor `attached`; cancel on `stopped`, `detached`, or `lost`; after daemon restart, recreate only after pid/start-time reattach succeeds. Missing log paths are non-fatal. The watcher reads file metadata only, never log contents, preserving transcript-off. Concurrency with `surgical_recovery` is resolved by using the same transactional job/lease checks; if the job is no longer in the heartbeatable state, the watcher skips.

## Per-Model Harness-Profile Fragments

Store fragments in the RFC 0034 generator path, preferably as Python constants in `src/striatum/workflow_generator/harness_profiles.py` with generator/catalog integration rather than long prompt strings buried only in JSON.

Codex instruction:

```text
Use native delegation aggressively for parallelizable work. The parent session remains accountable for Striatum artifacts, write scope, verification, and completion. This is a one-shot supervised invocation: you cannot ask the operator a follow-up question. If a step is ambiguous, choose the most-conservative default that matches the synthesis and proceed; the operator publishes the result on your behalf if your CLI access is denied. For long-running test runs, prefer focused pytest invocations before the wider `make test` to avoid lease expiry beyond ~30 minutes.
```

Claude Code instruction:

```text
Use sub-agents aggressively for bounded independent research, review, and code/test/doc authorship. This is a one-shot supervised invocation: you cannot ask the operator a follow-up question. If permission to call `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf. Do not let native delegation obscure the parent session's artifact and verdict accountability.
```

Gemini instruction:

```text
Use local sub-agents only for bounded independent work. This is a one-shot supervised invocation: write the artifact in this single call; do not surface strategy and exit without producing the file. When writing finding artifacts, ALL FIVE front-matter fields are required (none are optional): schema_version, artifact_kind, verdict_intent, severity, tags. Use verdict_intent (not verdict); severity from {low,medium,high,critical} (not none); tags as a JSON array; the `author:` byline is a plain markdown line AFTER the front-matter block, NOT a key inside it. Handoff artifacts (DESIGN.md / BUILD_HANDOFF.md / REVIEW.md for `accept`) do not need front matter; just the plain `author:` byline. Do not rely on remote agent-to-agent delegation.
```

## `striatum workflow upgrade` Verb

Ship `striatum workflow upgrade <path> [--dry-run] [--force] [--json]`. Default mode is dry-run/refuse-to-write; if the existing instruction is already current, report no-op. When writing is explicitly requested, refuse on conflict by default if an instruction is customized beyond a known older fragment shape. `--force` replaces the instruction with the canonical current text for the matching `tool_family`.

The upgrader modifies only `harness_profiles.*.native_delegation.instruction`. It must not rewrite jobs, lanes, edges, cycles, write scopes, roles, artifact paths, or unrelated formatting beyond deterministic JSON output. It refuses on a workflow with an active/running prepared snapshot unless `--force` is supplied and the operator accepts that only the file changes, not the already-snapshotted run.

## Implementer Split

`implement_systems_codex` owns daemon-side machinery: `src/striatum/dogfood/`, `src/striatum/daemon_supervisor/`, `src/striatum/daemon_rpc/`, `src/striatum/daemon_pg/`, recovery-adjacent systems hooks, and systems tests. Its staging order is capability addition, supervised-progress watcher, composite tools, then tests.

`implement_ergonomics_claude` owns chat-tool registry entries, web chat affordances if needed, harness-profile fragment updates, workflow upgrade, docs, and combined build handoff. Its staging order is chat-tool entries, harness fragments, workflow upgrade verb, then docs.

The workflow's write scopes are disjoint for implementation code except for conceptual integration through registry/tool names. Systems must not edit docs or chat files; ergonomics must not edit daemon systems paths.

## 3-Way Build Review

After both implementation jobs complete, run three fresh repo-level threat-model reviews in parallel: `review_build_codex`, `review_build_claude`, and `review_build_gemini`. Codex focuses on composite atomicity, watcher concurrency, and capability gating. Claude focuses on chat-tool visibility, workflow upgrade safety, and documentation honesty. Gemini focuses on adversarial cases: live-supervisor surgical recovery, concurrent composite calls, missing log files, active workflow upgrade, and token leakage across composite boundaries.

## Test Strategy

Systems unit tests cover `publish_on_behalf`: unacked claimed message, already-acked message, missing active lease, busy lease/message, missing artifact, review verdict path, and concurrent second call refusal.

Systems unit tests cover `surgical_recovery`: happy path, terminal job refusal, non-stale job refusal, missing expected artifact, attached/live supervisor refusal, concurrent supervisor refusal, pid identity mismatch, missing reason, capped or bounded lease extension, and structured audit metadata.

Progress watcher tests use fake clock/temp logs for mtime growth, missing log directories, idle threshold, dead process detection, and no transcript-content reads. Integration tests verify a real lease `expires_at` moves forward when log mtime advances.

Daemon/MCP tests assert `surgical_recovery` appears in `CAPABILITIES`, `daemon.describe`, and `tools/list` only for matching tokens; denied calls append audit rows; thin lifecycle tools map to the expected method/capability.

Ergonomics tests cover chat tool schemas and mutation hiding, generator profile text for codex/claude/gemini, workflow upgrade dry-run/no-op/conflict/force/running-workflow refusal, and docs link coverage including `docs/HARNESS_FRICTION_PATTERNS.md`.

Use focused pytest first, then `make lint`, `make typecheck`, the widest feasible `make test`, and `make smoke` if lease time permits.

## Documentation Deltas

Add `docs/HARNESS_FRICTION_PATTERNS.md` covering strategy-then-exit, question-then-exit, front-matter omissions, publish-on-behalf, and lease-expiry-under-active-load.

Update `docs/MCP.md` with dogfood-lifecycle tool names, capabilities, composite examples, confirmation semantics, and denial guidance.

Update `docs/HOW_TO_HUMAN.md` with the operator flow for MCP-driven dogfood operation and short-lived token issuance.

Update `docs/HOW_TO_AGENT.md` to say operator AI sessions should prefer structured MCP tools when a token/tool surface is available, while ordinary claimed jobs still obey work-packet commands.

Update `docs/UBIQUITOUS_LANGUAGE.md` with dogfood-lifecycle tool, publish-on-behalf, surgical recovery, and supervised-progress heartbeat.

Update `CHANGELOG.md`, `docs/CLI_REFERENCE.md`, `docs/SPEC.md` where the implementation changes user-visible behavior. Update `docs/DECISION_LOG.md` only if the build records RFC 0040 as accepted.

## Staging Plan

Systems-codex: add `surgical_recovery` capability and registry/DB constraints; implement and test `SupervisedProgressWatcher`; implement and test `dogfood.publish_on_behalf`; implement and test `dogfood.surgical_recovery`; wire audited daemon routing; run focused systems tests.

Ergonomics-claude: add lifecycle/composite chat-tool descriptors; add harness-profile constants and generator/catalog integration; add workflow upgrade parser/dispatch/implementation; update docs and changelog; write the combined build handoff; run focused ergonomics tests.

## Human-Decision Questions

No blocking human decision remains for implementers. The only implementation-time judgment left is mechanical: if the daemon registry cannot express dynamic capability for `publish_on_behalf`, use the conservative `review` capability for V1 and document the follow-up to split or relax it later.
