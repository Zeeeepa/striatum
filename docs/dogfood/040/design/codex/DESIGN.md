author: designer-codex-gpt-5.5-001

# RFC 0040 Implementation Design

## Design Stance

RFC 0040 should land as a narrow extension of the surfaces that already exist:
the daemon RPC method registry remains the authority for capability names and
route metadata, MCP/chat tools remain adapters over registered methods, and
repo-local `.striatum/state.sqlite3` remains the live workflow state for a
target repository. The implementation should not create a dogfood-specific
control plane. It should add two audited operator conveniences for recurring
dogfood recovery shapes, plus one daemon-side lease-refresh loop for supervised
sessions that are visibly making progress.

The implementer split should be strict. The systems implementer owns daemon
RPC, composite state transitions, the supervised-progress watcher, capability
registration, and tests for state-machine correctness. The ergonomics
implementer owns web chat-tool exposure, harness-profile text, `workflow
upgrade`, and documentation.

## Existing Surface To Reuse

`src/striatum/daemon_rpc/registry.py` already declares most lifecycle methods:
`run.prepare`, `run.start`, `session.register`, `claim_next`, `ack`,
`publish_artifact`, `complete`, `verdict`, `evidence.export`,
`supervise.start`, `supervise.stop`, and the related recovery methods. It also
owns `CAPABILITIES`, currently `read`, `write`, `review`, `claim`, `apply`,
`admin`, and `recovery`.

`src/striatum/daemon_rpc/server.py` already maps registered RPC methods to CLI
argv prefixes through `CLI_ROUTES` and `striatum.api.invoke`. The thin
lifecycle tools should call this route instead of duplicating CLI logic. Two
small naming alignments are needed: add `run.summary` to the registry and
`CLI_ROUTES`, and decide whether chat-facing tool names use friendly snake case
while RPC methods keep dotted names. The design below uses friendly chat names
that carry a `method` field internally.

`src/striatum/web/chat_tools.py` is the current closed-set local chat-tool
registry. It already hides mutating tools when `allow_mutations` is false and
enforces the operator confirmation path for `generate_workflow_write`. RFC 0040
should extend this module or split the new entries into
`src/striatum/web/chat_tools_dogfood.py` and import them into the same
closed-set `_TOOLS` list. Either way, `execute_tool` remains the single
dispatch entry point.

`src/striatum/mcp.py` is the older repo-local stdio wrapper over
`striatum.api.invoke`. It already exposes many lifecycle tools directly. Do
not make it authoritative for RFC 0040. The daemon MCP section of the same
module is the important MCP path: it filters `tools/list` through
`METHOD_REGISTRY` and re-authorizes `tools/call`, but the current code should be
checked because the call path may audit authorization without actually routing
the method through `DaemonRpcRouter`. RFC 0040 should close that gap for all
dogfood lifecycle methods so "MCP tool" means "authorized daemon RPC call", not
"tool name appeared in a list".

## Dogfood-Lifecycle Chat Tools

Add a shared dogfood tool descriptor table in
`src/striatum/web/chat_tools_dogfood.py`:

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

The daemon MCP implementation should expose these by registry method name and
dispatch through `DaemonRpcRouter` after authorization. The web chat tool
implementation may expose friendlier snake-case names, but it should accept
`repository_id` and `capability_token` when daemon mode is configured, then
submit the same daemon RPC envelope. If the current web chat surface is still
running in direct local service mode, it may fall back to `striatum.api.invoke`
only for methods that already have direct CLI equivalents, but the capability
requirement and mutation visibility should still be derived from
`METHOD_REGISTRY`. That keeps `tools/list` honest even before every operator
environment is fully daemon-routed.

Each tool schema should be minimal and CLI-shaped. For example,
`publish_artifact` takes `session_id`, `job_id`, `lease_id`, `kind`,
`logical_name`, and `path`; `verdict` takes `session_id`, `job_id`,
`lease_id`, `verdict`, optional `findings_artifact_id`, and optional
`rationale`; `run_prepare` takes `workflow_path` but translates it to the RPC
parameter the server expects. Denied calls must use the existing audit/request
log append path in the daemon router; the chat layer must not swallow
capability failures into a generic string when the daemon returns structured
denial data.

## Composite Tool: `dogfood.publish_on_behalf`

Add `src/striatum/dogfood/operator_tools.py` with a pure Python function:

```python
def publish_on_behalf(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    session_id: str,
    artifact_path: str,
    artifact_kind: str,
    logical_name: str,
    verdict: str | None,
    findings_artifact_id: str | None,
    verdict_rationale: str | None,
    reason: str,
) -> JsonObject:
    ...
```

The function should run inside one `BEGIN IMMEDIATE` transaction and use
existing domain functions where possible: `ack_work`, `publish_artifact`,
`record_verdict`/`verdict`, and `complete_job`/`complete`. It should not
assemble SQL updates except for the lookups that discover the active lease and
claimed message. The lookup rules are:

1. Find exactly one active lease for `session_id` whose job is not terminal.
2. Find the queue message for that lease/job. If it is already acked, skip
   `ack`; if it is claimed but unacked, call `ack_work`; otherwise refuse.
3. Publish the artifact with the supplied kind, logical name, and path.
4. If `verdict` is supplied, record the verdict using the published artifact id
   unless `findings_artifact_id` is explicitly supplied.
5. Complete the job unless the verdict route already transitions the review job
   according to the existing review command behavior.

The return payload should include `operation`, `composition_steps`,
`session_id`, `job_id`, `lease_id`, `message_id`, `artifact_id`, optional
`verdict_id`, and `reason`. The audit metadata should record the operator
reason and the list of composed commands, but not artifact contents or model
output.

Register the daemon RPC method as `dogfood.publish_on_behalf`. Capability
handling should be conservative: require `review` when a verdict is supplied,
otherwise require `write`. If the current registry cannot express dynamic
capability requirements, register two methods internally
(`dogfood.publish_on_behalf.write` and `dogfood.publish_on_behalf.review`) or
register the method as `review` and accept the narrower first slice. Do not
make it `admin`; this is a normal operator-on-behalf action, not state surgery.

## Composite Tool: `dogfood.surgical_recovery`

Add the second function to `src/striatum/dogfood/operator_tools.py`:

```python
def surgical_recovery(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    job_id: str,
    reason: str,
    extend_lease_seconds: int = 900,
) -> JsonObject:
    ...
```

This function is intentionally state-machine surgery and must be gated by a
new capability named `surgical_recovery`. This is a product-level exception to
RFC 0032's seven-capability vocabulary and should be treated as such in the
implementation notes. The capability should be added to `Capability` and
`CAPABILITIES` in `src/striatum/daemon_rpc/registry.py` and validated through
`src/striatum/daemon_rpc/capability.py` without special-case auth logic. It
also requires daemon DB migrations because the Postgres SQL baseline and later
capability constraints hardcode the closed vocabulary under
`src/striatum/daemon_pg/sql/`. Register
`MethodEntry("dogfood.surgical_recovery", "surgical_recovery", True)` in the
daemon method registry and route it in `src/striatum/daemon_rpc/server.py`.

Validation should be stricter than the historical manual SQL:

1. Load the job, current/last lease, queue message, session, run, expected
   artifacts, and supervisor row/pointer in one transaction.
2. Refuse terminal jobs and jobs that are not in a stale claimed/running shape.
3. Refuse if there is an attached concurrent supervisor different from the
   stale supervisor being recovered.
4. Verify every required expected artifact path for the job exists on disk
   inside the repo and under the job write scope. This is an operator-inspection
   proxy, not content validation.
5. If the artifact byline requires attestation, require a matching lost
   supervisor row/pointer with the same session and run. Reattach only that row.
6. Move the lease to `active` with `expires_at = now + extend_lease_seconds`.
7. Move the queue message to the post-ack state expected by the existing
   state machine and set `current_lease_id`.
8. Move the job to `running` with `current_lease_id`.
9. Move the supervisor or repo-local pointer from `lost` back to `attached`
   only when pid identity is still valid. If pid identity cannot be verified,
   return a refusal that tells the operator to publish as `author: operator`
   or restart the work normally.

The response should make the mutation auditable: `operation:
"surgical_recovery"`, `job_id`, `session_id`, `lease_id`, `message_id`,
`supervisor_id`, `new_expires_at`, `validated_artifact_paths`, and `reason`.
The daemon audit row should carry the reason as metadata. The reason is
operator-authored evidence and must be required, non-empty, and capped.

## Supervised-Progress Watcher

Create `src/striatum/daemon_supervisor/progress_watcher.py`. Keep it separate
from repo-local `src/striatum/supervisor.py` so the daemon-owned path can grow
without muddying V1 direct supervision.

The module should expose a small testable object:

```python
@dataclass(frozen=True)
class ProgressWatcherConfig:
    poll_seconds: float = 30.0
    active_window_seconds: float = 60.0
    idle_threshold_seconds: float = 600.0
    heartbeat_extend_seconds: int = 1800

class SupervisedProgressWatcher:
    def tick(self, supervisor: SupervisorRecord) -> ProgressWatcherResult:
        ...
```

The watcher should inspect log files under the daemon supervisor
`scratch_path`, preferring explicit packet log paths when available and falling
back to `**/*.log` under the supervisor scratch directory. If the newest mtime
is within `active_window_seconds`, call the existing heartbeat mutation for the
session's active lease. If no log exists or mtime is older than
`idle_threshold_seconds`, do not mutate the lease; emit a metadata-only warning
event and let ordinary lease expiry happen.

The heartbeat call must use the same state transition as
`striatum heartbeat`, not a direct SQL timestamp update. That preserves lease
validation and event emission. Tests can inject a fake clock and a fake
heartbeat callback; integration coverage should then verify the real DB
`expires_at` moves forward when a log file grows.

Daemon integration points:

- Start one watcher task when a daemon-owned supervisor transitions to
  `attached`.
- Cancel it when the supervisor transitions to `stopped`, `detached`, or
  `lost`.
- On daemon restart, recreate watchers only after pid/start-time reattach
  succeeds.
- Never read agent stdout/stderr contents. The watcher checks file metadata
  only; transcript-off remains intact.

## Harness-Profile Fragments

Move the recurring instructions into a central source of truth, not scattered
string literals. Add `src/striatum/workflow_generator/harness_profiles.py`
with constants:

- `NO_QUESTIONS_FRAGMENT`: one-shot invocation, do not ask follow-up questions,
  choose the most conservative default, write the artifact, operator publishes
  on behalf if CLI access is denied.
- `CODEX_LONG_TEST_FRAGMENT`: prefer focused pytest before wider `make test`
  for long-running test work.
- `GEMINI_FINDING_FRONT_MATTER_FRAGMENT`: all five fields required:
  `schema_version`, `artifact_kind`, `verdict_intent`, `severity`, `tags`;
  author byline after the front matter; handoff artifacts do not need finding
  front matter.

`src/striatum/workflow_generator/core.py` should call helper functions from
that module when generating default harness profiles. Today the generator
requires `options.harness_profiles` for `harness_profiled`; RFC 0040 can either
add catalog defaults for common tool families or normalize operator-supplied
profiles by appending the fragments. Prefer catalog defaults for dogfood
templates and normalization only when a profile declares a known `tool_family`.

Update `src/striatum/workflow_templates/catalog.json` only for metadata that
helps the generator choose default profile families. Do not bury long prompt
text in JSON if Python constants can own it more safely.

## `striatum workflow upgrade <path>`

Add the parser shape in `src/striatum/cli/parser.py` under the existing
`workflow` subparser:

```text
striatum workflow upgrade <path> [--dry-run] [--force] [--json]
```

Add dispatch in `src/striatum/cli/dispatch.py` and implementation in a new
module `src/striatum/workflow_generator/upgrade.py`. The upgrader should:

1. Parse the target JSON preserving deterministic formatting on write.
2. Visit `harness_profiles.*.native_delegation.instruction`.
3. Detect the profile `tool_family`.
4. Build the desired instruction by appending the appropriate fragment only
   when it is missing.
5. Refuse by default if the existing instruction differs from a known older
   default in a way that would require overwriting operator text.
6. With `--force`, replace the instruction with the canonical current text for
   that family.
7. With `--dry-run`, write nothing and return changed paths plus before/after
   snippets or a JSON patch-like list.

The upgrader must not modify jobs, edges, cycles, lanes, write scopes, roles,
or artifact paths. Its output should make that narrowness explicit.

## Documentation

The ergonomics implementer should update:

- `docs/MCP.md`: "Dogfood-Lifecycle Tools", capability table, thin-tool
  examples, composite-tool examples, and denial guidance.
- `docs/HOW_TO_HUMAN.md`: operator walkthrough for dogfood via tools:
  issue short-lived token, call `tools/list`, prepare/start/register/claim,
  publish on behalf, stop supervisors, export summary/evidence.
- `docs/HOW_TO_AGENT.md`: when a capability token and MCP tools are available,
  prefer structured tools over hand-composed shell commands for dogfood
  operations; still obey work-packet commands inside ordinary agent jobs.
- `docs/HARNESS_FRICTION_PATTERNS.md`: record strategy-then-exit,
  question-then-exit, front-matter omissions, and lease-expiry-under-active-load
  with links to dogfood reports.
- `docs/UBIQUITOUS_LANGUAGE.md`: define publish-on-behalf,
  surgical-recovery, supervised-progress heartbeat, and dogfood-lifecycle tool.
- `CHANGELOG.md`: added entry for RFC 0040.

If implementing teams decide the RFC itself is accepted during the build, also
update `docs/DECISION_LOG.md`; otherwise leave product-decision edits for the
operator/synthesis lane.

## Test Plan

Systems tests:

- Unit-test `publish_on_behalf` for unacked claimed message, already-acked
  message, missing active lease, missing artifact, review verdict path, and
  concurrent second call refusal.
- Unit-test `surgical_recovery` for happy path, terminal job refusal,
  missing expected artifact refusal, attached-concurrent-supervisor refusal,
  pid identity mismatch, missing reason, and capped `extend_lease_seconds`.
- Assert audit/request-log metadata includes operation name and reason but not
  artifact bytes.
- Add daemon registry tests that `surgical_recovery` appears in `CAPABILITIES`,
  `daemon.describe`, `tools/list` only for matching tokens, and denied calls
  append audit rows.
- Test the progress watcher with fake clock and temp log files; integration
  test with a repo-local run should show heartbeat extension when log mtime
  advances.

Ergonomics tests:

- Chat tool schema tests for every lifecycle tool and the two composite tools,
  including hidden mutating tools when mutations are disabled.
- Dispatch tests proving thin tools call the expected RPC method and preserve
  structured denials.
- Generator tests for codex, claude_code, and gemini profile text.
- `workflow upgrade` tests for dry run, no-op already-current workflow, default
  refusal on conflicting customized instruction, and `--force` replacement.
- Documentation link tests should include the new friction-patterns doc.

Focused test commands for implementers should start with the new unit tests
rather than immediately running the full suite, because RFC 0040 is explicitly
trying to avoid lease expiry during long unfocused test runs. The final handoff
should still report `make lint`, `make typecheck`, focused tests, and the
widest feasible `make test`/`make smoke` result.

## Implementation Order

1. Add `surgical_recovery` to the capability vocabulary and registry, then add
   route stubs that deny cleanly until implementation is wired.
2. Implement `src/striatum/dogfood/operator_tools.py` and systems tests.
3. Add the progress watcher as a testable daemon-supervisor helper, then wire it
   into daemon-owned supervisor transitions.
4. Add chat-tool descriptors and dispatch for lifecycle and composite tools,
   deriving visibility from registry capabilities.
5. Add harness-profile constants and generator integration.
6. Add `workflow upgrade`.
7. Update docs and run the focused verification matrix.

The main risk is over-broad operator authority. Keep `publish_on_behalf` as a
normal write/review composition, keep `surgical_recovery` behind its own
short-lived admin-issued capability, and keep every path audited through the
daemon machinery rather than direct SQLite edits.
