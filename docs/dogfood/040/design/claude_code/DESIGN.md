# RFC 0040 Implementation Design — Trust Boundaries and Operator Footguns

author: designer-claude-opus-001

## Scope and Stance

This design assumes RFC 0040's seven proposed subsystems land together: the
MCP exposure of the dogfood-lifecycle RPC verbs (§1), the two composite
tools `dogfood.publish_on_behalf` (§2) and `dogfood.surgical_recovery` (§3),
the daemon-side supervised-progress watcher (§4), the per-model
"no-questions" + "front-matter completeness" harness-profile fragments
(§5–§6), the documentation pass (§7), and the explicit no-change list (§8).
It does NOT redesign RFC 0030 (RPC), RFC 0032 V2 (MCP capability gating),
RFC 0034 V1 (workflow generator), or RFC 0036 V1 (chat-tool gestures).

The remit from `docs/dogfood/040/prompts/design_claude_code.md` narrows the
implementation discussion to four trust-boundary concerns. Everything in
this document is structured around those four concerns plus the
operator-mistake footguns they create. Where I make a tradeoff the RFC
left as an open question, I land on a single recommendation, name the
losing option, and explain why a future maintainer should be able to
flip the choice cheaply if production friction reveals I was wrong.

The four trust-boundary concerns:

1. Capability authorization for the new `surgical_recovery` vocabulary
   entry.
2. Audit-chain semantics for composite-tool operations (single audit row
   with `composition_steps` metadata vs decomposed sequence linked by a
   shared `composition_id`).
3. Operator-confirmation semantics for the new composite tools (do they
   inherit RFC 0036's `confirm_write: true` + UI-gesture pattern, or is
   their input shape sufficient confirmation).
4. Supervised-progress watcher interaction with lease ownership when the
   operator is also calling `dogfood.surgical_recovery` against the same
   lease, when the supervised wrapper crashes mid-log-growth, and when
   multiple supervisors run concurrently with separate scratch dirs.

A fifth concern — the daemon-side watcher lifecycle (start on
`supervise.start`, stop on `supervise.stop`, restart on daemon restart) —
is covered alongside concern 4 because the lifecycle invariants are what
make the lease-ownership invariants hold.

I treat the `workflow upgrade` CLI verb (RFC 0040 §5) as a trust-boundary
concern even though the prompt did not list it explicitly, because it is
the only repository-write surface this RFC introduces and it shares the
"composite operation, no preview" footgun shape with `surgical_recovery`.

## Background Invariants

These are NOT being changed. The design relies on them; I name them so
implementers can verify before touching surrounding code:

- **Capability vocabulary is closed.** `src/striatum/daemon_rpc/registry.py`
  exports `CAPABILITIES = frozenset({"read", "write", "review", "claim",
  "apply", "admin", "recovery"})`. Every RPC method declares its required
  capability at registry-import time. RFC 0040 adds exactly one entry —
  `surgical_recovery` — and registers it as `Literal` member of the
  `Capability` alias.
- **Capability tokens are repository-scoped or daemon-global.** The
  authorize path in `src/striatum/daemon_rpc/capability.py` accepts a
  `repository_id` and either matches a row in
  `striatumd.client_capabilities` with that repo id OR a row with
  `repository_id IS NULL` (daemon-global). The
  `capability_scope_mismatch` denial fires when the client has the
  capability for a DIFFERENT repo. This is the existing 7-capability
  semantics and RFC 0040 reuses it as-is.
- **Audit rows are append-only and capability-decorated.** Every RPC
  call writes a `daemon_audit` row keyed by `request_id`, capturing
  `client_id`, `token_id`, `repository_id`, `capability`, `decision`,
  `denial_reason`, plus method-specific metadata. Composite tools write
  one row per composite call (see concern 2).
- **The mutation gate is the only "double-confirm" surface today.**
  `src/striatum/web/chat_tools.py` enforces it for
  `generate_workflow_write` via the three-AND condition `allow_mutations
  AND confirm_write AND operator_confirmed`. The two new composite tools
  reuse this exact gate shape where it applies (see concern 3).
- **Leases are owned by sessions, refreshed by lease holder, expired
  lazily.** Per `docs/HOW_TO_AGENT.md` and `src/striatum/cli/recovery.py`,
  the lease is the session's claim on a job, the session refreshes it
  via `striatum heartbeat`, and the daemon expires it lazily — `expire_leases`
  fires at the start of every recovery call. RFC 0040's
  supervised-progress watcher refreshes leases ON BEHALF OF the session
  whose supervised wrapper is making forward progress; the watcher does
  not own the lease, the session does.
- **Supervisor PIDs are attested.** `lane_attestation.pid` is recorded
  per supervisor at `supervise.start` time. The watcher uses this PID
  for liveness probing (see concern 4, sub-issue B).

If any of these invariants is being changed by other in-flight work,
flag it before merging this RFC.

## Concern 1 — Capability authorization for `surgical_recovery`

### Recommendation

Add `surgical_recovery` to the closed `Capability` vocabulary as a
**daemon-global single-key admin capability**, never repository-scoped,
issued only via short-lived tokens (`expires_in ≤ 15m`), denied for any
repository_id mismatch by failing CLOSED rather than auto-scoping.

### Vocabulary change

`src/striatum/daemon_rpc/registry.py`:

```python
Capability = Literal[
    "read", "write", "review", "claim", "apply",
    "admin", "recovery", "surgical_recovery"
]
CAPABILITIES: frozenset[str] = frozenset({
    "read", "write", "review", "claim", "apply",
    "admin", "recovery", "surgical_recovery",
})
```

The composite tool registers as:

```python
MethodEntry(
    "dogfood.surgical_recovery",
    "surgical_recovery",
    repository_scope=True,
    repository_scope_mode="single_repo",
    audit_class="state_transition",
)
```

Note: the **method** is repository-scoped (it always operates on a
specific job in a specific repo), but the **capability** is
daemon-global. The two are independent axes:

- `repository_scope=True` means the call must carry a `repository_id`
  parameter and the daemon binds the operation to that repo.
- The capability being daemon-global means tokens carry it with
  `repository_id IS NULL` in `striatumd.client_capabilities`, so the
  authorize path matches via the existing "daemon-global wins" branch.

This combination has precedent: `daemon.token.create` is repository-
neutral; `apply.reviewed_patch` is repo-scoped with the `apply`
capability that is in practice issued daemon-globally for the operator.
RFC 0040 makes the daemon-global scoping explicit for
`surgical_recovery` by REJECTING repository-scoped issuance.

### Token issuance constraints

`daemon.token.create` already accepts `--capability` and `--expires-in`.
For `surgical_recovery` we add two issuance-time constraints, enforced
in `src/striatum/daemon/token_create.py` (or wherever
`daemon.token.create` validates input):

1. **Forbid repository scoping at issuance.** If the operator invokes
   `daemon.token.create --capability surgical_recovery --repo <id>`, the
   daemon refuses with `RpcError("schema_invalid",
   "surgical_recovery is a daemon-global capability; do not pass
   --repo")`. The token must be issued with `repository_id IS NULL` in
   `client_capabilities`.
2. **Cap the TTL at 15 minutes.** The CLI documents 15m as the
   recommended TTL; the daemon enforces a hard ceiling. Any
   `--expires-in` value > 15m is clamped to 15m and the response
   includes `ttl_clamped: true` in the `data` field. A daemon-side
   constant `SURGICAL_RECOVERY_MAX_TTL_SECONDS = 900` lives in
   `src/striatum/daemon_rpc/capability.py` near `_expired`.

The TTL ceiling is a footgun guard, not a security boundary: an
attacker who has obtained the admin token can mint a fresh
surgical_recovery token at any time. The 15-minute cap exists so the
operator's surgical_recovery token does not sit in a shell history for
weeks waiting to be invoked accidentally.

### Authorize-path changes

`src/striatum/daemon_rpc/capability.py::authorize` is structured around
`required` (the capability name on the method entry) and `repository_id`
(the parameter from the request). Add one branch:

```python
if required == "surgical_recovery":
    # Capability is daemon-global by design. If the capability row
    # in client_capabilities is repo-scoped, that's a misconfiguration:
    # refuse with surgical_recovery_validation_failed, not capability_missing.
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT repository_id FROM striatumd.client_capabilities
            WHERE client_id = %s AND capability = 'surgical_recovery'
              AND revoked_at IS NULL
            LIMIT 1
            """,
            (client_id,),
        )
        cap_row = cur.fetchone()
    if cap_row is None:
        return RpcAuthContext(..., decision="denied",
                              denial_reason="capability_missing")
    if cap_row["repository_id"] is not None:
        return RpcAuthContext(..., decision="denied",
                              denial_reason="capability_scope_mismatch")
    # ... fall through to expiry check
```

This denial path is checked BEFORE the generic repository-scope match
because surgical_recovery's invariant is "no repo scoping at all."

### Denial vocabulary

The audit row's `denial_reason` field accepts a small closed set per
RFC 0030. RFC 0040 adds one new value:
`surgical_recovery_validation_failed`. The existing values
`capability_missing`, `token_expired`, `token_revoked`,
`token_malformed`, `token_invalid`, `capability_scope_mismatch`,
`capability_expired` continue to mean what they meant before.

`surgical_recovery_validation_failed` is the new denial reason for the
case "auth passes, but the surgical-recovery preconditions don't":

- Expected artifacts from the job are not on disk at their declared
  paths.
- Another supervisor is currently attached for the same lane in the
  same run.
- The job is in a state where surgical recovery would corrupt the
  audit chain (e.g., already `completed`, or in `applied` after a
  sealed apply).

Each precondition failure produces the same `denial_reason` but the
audit row's metadata (`details` JSON column) names the specific failure
so the operator can fix the inspection note and retry.

### Audit-row shape for surgical_recovery

Every `dogfood.surgical_recovery` call appends one row, regardless of
decision. The row's `audit_class` is `state_transition`; the `details`
JSON contains:

```json
{
  "operation": "surgical_recovery",
  "job_id": "...",
  "lease_id_before": "...",
  "lease_id_after": "...",
  "supervisor_state_before": "lost",
  "supervisor_state_after": "attached",
  "queue_message_state_before": "claimed",
  "queue_message_state_after": "claimed",
  "extend_lease_seconds": 900,
  "operator_reason": "BUILD_HANDOFF.md present, tests passed, ready to publish",
  "preconditions_checked": [
    {"check": "expected_artifacts_on_disk", "passed": true,
     "evidence": ["docs/dogfood/040/build/BUILD_HANDOFF.md"]},
    {"check": "no_concurrent_supervisor", "passed": true,
     "evidence": {"supervisor_id": "sup_...",
                  "supervisor_state": "lost"}},
    {"check": "job_state_recoverable", "passed": true,
     "evidence": {"state": "stale_lease"}}
  ]
}
```

For denied calls, `lease_id_after` etc. are null and `decision` is
`denied` with the specific `denial_reason`. The `preconditions_checked`
array is still populated where preconditions ran before denial fired,
so the operator can see how far the precondition pipeline got.

### Footgun guards

The capability is admin-only. The footguns the operator can still hit:

- **Reusing a stale token across runs.** Mitigation: 15m TTL ceiling.
- **Running surgical_recovery against a job that doesn't need it.**
  Mitigation: preconditions in §3 below; the daemon refuses if the
  supervisor is `attached` (not `lost`), the lease is not expired, or
  the queue message is not in a claimed-but-incomplete state.
- **Confusing surgical_recovery with the existing `recovery` capability.**
  Mitigation: they have different capability names. The `recovery`
  capability is for `recovery.stale_leases` / `recovery.requeue_stale`
  etc., which refuse repo-write jobs by policy. `surgical_recovery` is
  the override for that policy and is gated separately.

## Concern 2 — Audit-chain semantics for composite tools

### Recommendation

Option (a): one audit row per composite call with
`composition_steps` as JSON metadata.

### Why this choice

The audit chain serves two readers: the operator inspecting "what
happened" and tooling reconstructing system state from the chain. Both
are better served by the composed shape:

1. **The operator's mental model is "one publish-on-behalf op."** The
   composite tool is the level of abstraction the operator chose. If
   the chain decomposes the operation into four rows
   (`ack`/`publish`/`verdict`/`complete`), the operator has to re-glue
   them mentally on every inspection. With the composed shape, the
   chain answers "what did the operator do" at the same level the
   operator was reasoning at.
2. **A single audit row is atomic with the state transition it
   records.** With the decomposed shape, if the daemon process crashes
   between step 2 and step 3, the audit chain is half-written and the
   reader cannot tell whether the operation completed. With the
   composed shape, either the row is there (operation atomic) or it
   is not (operation rolled back). This matches existing
   `apply.reviewed_patch` semantics, which writes ONE audit row for a
   multi-file patch.
3. **Decomposed inspection is still possible.** The
   `composition_steps` array lists each sub-operation with its
   `step_kind`, `target_id`, `timestamp`, and outcome. Tooling that
   wants the per-step view can flatten the JSON; the chain's row-level
   shape is unchanged.

### Why NOT option (b)

Option (b) — decomposed sequence linked by `composition_id` — has one
real advantage: the decomposed rows are queryable directly. A
hypothetical "how many ack ops happened today" query would count
ack-rows under both shapes; under (b) the count includes composite
ones for free, under (a) the query must JOIN against the
`composition_steps` JSON.

That advantage does not outweigh the atomicity loss. Where queryability
matters, we can add a materialized view that flattens
`composition_steps` to per-step rows; the underlying chain stays
composed.

### Audit-row shape for `publish_on_behalf`

```json
{
  "operation": "publish_on_behalf",
  "session_id": "...",
  "job_id": "...",
  "lease_id": "...",
  "artifact_path": "docs/dogfood/040/design/claude_code/DESIGN.md",
  "artifact_kind": "handoff",
  "logical_name": "claude_code_design",
  "operator_reason": "claude --print denied ack on supervised wrapper",
  "composition_steps": [
    {"step": "ack", "message_id": "msg_...",
     "ts": "2026-05-12T14:55:12Z", "outcome": "ok"},
    {"step": "publish_artifact", "artifact_id": "art_...",
     "ts": "2026-05-12T14:55:12Z", "outcome": "ok"},
    {"step": "verdict", "verdict_id": "vd_...",
     "verdict": "approve_with_followups", "ts": "...",
     "outcome": "ok"},
    {"step": "complete", "job_state_after": "completed",
     "ts": "2026-05-12T14:55:13Z", "outcome": "ok"}
  ]
}
```

The `verdict` step is only present when the underlying job is a review
job; for non-review jobs the composition_steps array is three entries
(ack, publish_artifact, complete).

### Audit-row shape for `surgical_recovery`

See concern 1's shape. The `composition_steps` array names each atomic
state mutation: `lease_reactivate`, `supervisor_reattach`,
`queue_message_revert`, `job_state_revert`. The atomic SQL transaction
either commits all four (single audit row with all four steps) or
commits none.

### Audit-row shape for the unwrapped MCP tools

The thin-shell MCP tools (`mcp.dogfood.ack`, `mcp.dogfood.publish_artifact`,
etc.) reuse the existing audit-row shape for their underlying RPC
method. They do NOT carry a `composition_steps` field; they ARE the
single step they record. The transport (MCP vs CLI vs HTTP JSON-RPC)
appears in the audit row as the `transport` field but does not change
the row's structural shape.

This keeps the chain's row schema simple: composite operations have
`composition_steps`, non-composite operations don't.

### Partial-failure semantics

What happens if step 3 of `publish_on_behalf` fails (e.g., the verdict
validation rejects the rationale)? Two options:

(i) **All-or-nothing transaction.** The composite runs inside a single
DB transaction; partial failure rolls back to the pre-composite state.
The audit row records `outcome: "rolled_back"` with the failing step's
error in `failure_details`.

(ii) **Best-effort with break.** Step 1 commits, step 2 commits, step
3 fails, the daemon stops, the audit row records steps 1–2 as ok and
step 3 as failed.

Recommendation: **(i) all-or-nothing.** Reasoning: the composite tool's
contract to the operator is "do this whole thing or don't." Partial
state ("ack happened but publish_artifact didn't") is exactly the
SQLite-surgery footgun this RFC removes.

Implementation note: `publish_on_behalf` runs the four underlying state
transitions inside one SQLite transaction. The existing `transaction()`
helper from `src/striatum/db.py` supports this pattern. Rollback on
partial failure means the operator can retry the composite call
without compensating for half-applied state.

### Cross-row composite identification

For tooling that wants to find composite operations across the chain,
the row's `details.operation` field is the discriminator. Today the
chain already uses `operation` for the RPC method name. Composite
operations use the new values `publish_on_behalf` and
`surgical_recovery`; the unwrapped MCP tools use the underlying method
name unchanged.

## Concern 3 — Operator-confirmation semantics for composite tools

### Recommendation

| Tool | `confirm_write` arg | UI gesture | Why |
|---|---|---|---|
| `dogfood.publish_on_behalf` | NO | NO | `reason` IS the confirmation |
| `dogfood.surgical_recovery` | YES | YES | bypasses recovery policy |
| Thin-shell MCP tools (ack/publish/verdict/complete/heartbeat/release) | NO | NO | same as CLI |
| Thin-shell MCP tools (run.prepare/run.start/register-session/supervise.start) | NO | NO | same as CLI |
| Thin-shell `dogfood.supervise.stop` | NO | NO | same as CLI |
| `workflow upgrade` (CLI verb) | YES (`--apply`) | NO | refuse-on-conflict, dry-run default |

### Why `publish_on_behalf` skips confirm_write

The semantics are:

- The operator's chat session decided to invoke the composite tool.
  The model produced the `reason` string. The operator read the
  reason. Adding `confirm_write: true` adds nothing the reason field
  doesn't already encode.
- The composite tool's failure mode is "publishes an artifact the
  operator intended to publish." If the operator's session called it
  by mistake, the published artifact still went to the right place on
  disk (the operator's session controls the artifact path argument). 
  The audit chain records the operator-on-behalf publish as such.
- Requiring a UI gesture would force the operator to confirm every
  ack-denied case, of which there are 3–7 per dogfood. The friction
  removal is the point of the RFC.

The trust boundary that `publish_on_behalf` does NOT cross:

- It cannot publish to an arbitrary path. The artifact_path must be
  inside the job's `write_scope.allowed_paths`. The daemon enforces
  this before step 2 of the composite.
- It cannot satisfy expected_artifacts the job did not declare. The
  daemon validates `(artifact_kind, logical_name)` against the job's
  `expected_artifacts` list.
- It cannot affect a job the session does not own. The daemon
  validates session ownership before step 1.

These three boundaries are exactly the same boundaries the
non-composite `publish-artifact` enforces. The composite is a
convenience, not a privilege escalation.

### Why `surgical_recovery` requires both gates

The semantics are:

- Surgical recovery BYPASSES the recovery-policy refusal that
  `recovery.requeue_stale` enforces for repo-write jobs. Bypassing
  policy is exactly the kind of operation that should require a
  deliberate gesture, not a single tool call.
- The operation modifies daemon state in ways no other operator-
  facing tool does (reactivating an expired lease, restoring a
  supervisor from `lost` to `attached`). A double-confirm gate
  prevents the operator's session from invoking it as part of an
  exploratory tool-call sequence.
- The blast radius is bounded (one job in one run) but the failure
  mode if invoked against the wrong job is "corrupted audit chain"
  — silent, hard to diagnose later, and exactly the kind of footgun
  the audit chain is supposed to make visible.

The double-confirm is the same as `generate_workflow_write`'s gate:

```python
def _tool_dogfood_surgical_recovery(
    repo, job_id, reason, extend_lease_seconds,
    *, confirm_write, allow_mutations, operator_confirmed,
):
    if not allow_mutations:
        return "[error] mutations_disabled: ..."
    if not confirm_write:
        return "[error] confirm_write_missing: ..."
    if not operator_confirmed:
        return "[error] operator_gesture_missing: ..."
    # ... proceed with the composite
```

### Why thin-shell MCP tools skip the gate

The thin-shell MCP tools wrap RPC verbs the operator already invokes
via CLI without a confirmation gate. Adding the gate at the MCP layer
without adding it at the CLI layer would create an asymmetry: the same
operation requires a UI gesture from MCP but not from a typed CLI
command. The capability gate already enforces the trust boundary;
adding a second gate on top would be friction without a clear
threat-model justification.

The thin-shell MCP tools DO require `--allow-mutations` (the same
service-level posture flag that gates `generate_workflow_write`).
Without `--allow-mutations`, the entire mutation surface is hidden.

### Why `workflow upgrade` uses `--apply`, not UI gesture

`workflow upgrade` is a CLI verb, not a chat tool. The CLI's existing
double-confirm shape is the `--dry-run` (default) / `--apply` pair, as
used by `striatum workflow validate --fix`. Following that convention
keeps the verb consistent with adjacent CLI surfaces.

Operator-facing behavior:

```
$ striatum workflow upgrade docs/dogfood/035/workflow.json
[dry-run] would update harness_profiles.codex.native_delegation.instruction
[dry-run] would update harness_profiles.claude_code.native_delegation.instruction
[dry-run] would update harness_profiles.gemini.native_delegation.instruction
[dry-run] re-run with --apply to write changes
```

The verb refuses to write when the local file has uncommitted changes
that affect the same fields it would modify, citing the offending
field path. Operators with active edits in the affected fields must
commit or stash before retrying.

## Concern 4 — Supervised-progress watcher lifecycle and lease ownership

### Recommendation

Per-supervisor watcher tasks (one task per supervisor); the watcher
acquires a job-level lock before refreshing the lease; the lock is
also acquired by `dogfood.surgical_recovery` so the two cannot race;
the watcher probes the supervisor's PID with `os.kill(pid, 0)` and
stops refreshing when the process is dead; the watcher uses
mtime-based polling (NOT a sidecar progress signal) for V1.

### Watcher lifecycle

The watcher is daemon-internal. It has no MCP surface, no CLI surface,
no operator-visible state outside of audit rows and warnings in the
daemon log. Implementation lives in
`src/striatum/daemon_supervisor/progress_watcher.py`.

Lifecycle events:

1. **Start.** When `supervise.start` returns successfully, the daemon
   calls `progress_watcher.attach(supervisor_id, pid, log_path,
   session_id)`. This spawns a daemon-internal task (asyncio task or
   thread, see below) that polls mtime every 30s.
2. **Stop.** When `supervise.stop` is called, the daemon calls
   `progress_watcher.detach(supervisor_id)`. The task is cancelled
   cooperatively; the watcher's next mtime check observes the
   cancellation flag and exits cleanly.
3. **Crash recovery.** On daemon restart, the daemon enumerates
   `supervisors` rows with `state IN ('attached', 'lost')` and calls
   `progress_watcher.attach(...)` for each. The watcher's first check
   probes liveness; if the PID is gone, the supervisor's state is
   updated to `lost` and the watcher exits.

The watcher does NOT call back into the RPC layer; it calls
`heartbeat()` directly against the daemon's connection pool with a
synthetic auth context (`client_id = "_daemon_supervised_progress"`,
no token, decision="allowed_by_daemon_internal"). The synthetic
context appears in the audit row's `client_id` column so operators
can distinguish daemon-internal heartbeats from session-driven ones.

### Why daemon-internal auth and not a token

A token-bound auth path would mean the daemon mints itself a write
capability at startup and uses it for every internal heartbeat. That
would either:

- Be a long-lived token (security regression — a leaked token from the
  daemon's memory would have indefinite write access), or
- Be a short-lived token rotated by the daemon (complexity without
  benefit — the daemon already has process-level trust to its own DB).

The synthetic auth context bypasses the capability check entirely.
This is the same pattern the daemon uses for sealed-apply receipts and
for cross-repo reconciliation: internal operations skip the capability
gate but DO write audit rows so they remain inspectable.

### Mtime polling vs sidecar signal

The RFC's open question §4 names two options:

- **mtime-based:** the watcher stats the supervised wrapper's log file
  and refreshes the lease if mtime is within the last 60s.
- **sidecar signal:** the supervised wrapper emits a heartbeat line to
  a sidecar file (e.g., `<scratch>/codex-logs/heartbeat.fifo`) that
  the watcher reads.

Recommendation: **mtime for V1.**

- Works against current wrappers without modification.
- Conservative: a wrapper that emits log lines is making progress; a
  wrapper that does not emit log lines is either done or stuck. The
  watcher does not refresh dead supervisors (concern 4b), so the false-
  refresh rate is low.
- Pluggable to sidecar in V1.5: the watcher's "growth detection"
  function is the only thing that changes; the lease-refresh path is
  unchanged.

The mtime path's false-negative case: a supervised wrapper that runs
for >`idle_threshold_seconds` (default 600s) without writing to its
log. Today this means the watcher does NOT refresh; the lease expires;
the operator hand-recovers via `dogfood.surgical_recovery`. That is
strictly better than today's "the lease expires and the operator
hand-edits SQLite" path because surgical_recovery is now a single
auditable tool call.

### Concurrency invariant: per-job lock

Concern 4 sub-issue A: the operator calls `dogfood.surgical_recovery`
on a job at t=N; the watcher refreshes the lease at t=N+1; both
observe stale state and write conflicting updates.

Solution: a **per-job advisory lock** held during the entire
surgical_recovery transaction AND queried (non-blocking) by the
watcher before every refresh.

```python
def watcher_tick(supervisor_id, pid, log_path, session_id):
    if not _process_alive(pid):
        return _mark_supervisor_lost(supervisor_id)
    if not _log_grew_recently(log_path):
        return
    lease = _active_lease_for_session(session_id)
    if lease is None:
        return  # session has no current lease
    if _job_lock_held(lease.job_id):
        # surgical_recovery is mid-flight; skip this tick.
        _log.debug("watcher: job lock held; skipping refresh",
                   job_id=lease.job_id)
        return
    _refresh_lease(lease.lease_id, ttl=DEFAULT_WATCHER_TTL)
```

The lock is implemented as a row in a new
`striatumd.job_advisory_locks` table:

```sql
CREATE TABLE striatumd.job_advisory_locks (
  job_id TEXT PRIMARY KEY,
  holder TEXT NOT NULL,           -- 'surgical_recovery' or 'watcher'
  acquired_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL
);
```

Locks are acquired with `INSERT ... ON CONFLICT DO NOTHING` and held
inside a transaction. They auto-expire (cleanup query at the start of
every recovery operation, mirroring the existing `expire_leases`
pattern). The watcher uses a non-blocking lock-check (the SELECT
above); surgical_recovery holds the lock for the duration of the
4-step composite transaction.

The watcher does NOT acquire the lock itself: it would be a per-tick
write on the hot path. The asymmetric pattern (writer holds, reader
checks) is acceptable here because the operations are NOT mutually
racing on the same column — they are racing on overlapping but
distinct state (watcher refreshes lease, surgical_recovery reactivates
lease + reattaches supervisor + reverts queue state). The lock ensures
the WRITES happen serially; the READS can proceed without it.

### Liveness probe

Concern 4 sub-issue B: the supervised wrapper crashes, the log file is
abandoned; the watcher must not keep refreshing forever.

The watcher probes the supervisor's PID before each refresh:

```python
def _process_alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        # Process exists but is owned by another uid (shouldn't happen
        # in single-user single-daemon deployment; treat as alive to
        # avoid false negatives).
        return True
```

If the probe says the process is dead, the watcher:

1. Updates the supervisor row from `attached` to `lost`.
2. Records a daemon-internal audit row with
   `operation = "supervisor_died_observed_by_watcher"`.
3. Detaches itself (cancels future ticks).

The daemon does NOT automatically reclaim the lease in this case; the
lease expires lazily through the normal path. The operator's
recovery action remains `dogfood.surgical_recovery` if the job
produced artifacts before the crash, or `recovery.requeue_stale` if
not.

### Multiple supervisors

Concern 4 sub-issue C: multiple supervisors run concurrently with
separate scratch dirs.

Per-supervisor watcher tasks make this trivial: each watcher closes
over its own `(supervisor_id, pid, log_path, session_id)` tuple. The
shared resource is the `job_advisory_locks` table, which is keyed by
`job_id` (the per-job granularity matches the per-job composite-tool
granularity).

The watcher pool's shape is one task per `supervisors` row in state
`attached`. The pool is keyed by `supervisor_id`; attach/detach are
idempotent. On daemon restart the pool is rebuilt from
`supervisors WHERE state = 'attached'`.

### Asyncio task or thread?

Recommendation: **asyncio task**, on the daemon's main event loop, with
a 30s `asyncio.sleep` between ticks.

Why: the daemon already runs an asyncio loop for the RPC server and
for sealed-apply background work. Adding watcher tasks to the same
loop reuses the existing concurrency model. Threads would require
threadsafe access to the connection pool; the daemon's pool is async.

Failure modes the asyncio choice introduces:

- A blocking `os.stat` call inside the watcher would stall the loop.
  Mitigation: `loop.run_in_executor(None, os.stat, log_path)` wraps
  the syscall.
- A blocking lease-refresh query would stall the loop. Mitigation:
  use the async DB client (psycopg async / aiosqlite), not the sync
  one, for the watcher's queries.

The daemon's existing async patterns already handle these; the
watcher inherits them.

## Concern 5 — `workflow upgrade` as a footgun surface

### Recommendation

`workflow upgrade` is a NEW CLI verb (the only new CLI verb this RFC
adds) and it writes to repository files. Treat it like
`generate_workflow_write` from the CLI side: refuse-on-conflict,
dry-run by default, scoped narrowly to harness-profile fragments.

### Surface shape

```
$ striatum workflow upgrade <path>                    # dry-run (default)
$ striatum workflow upgrade <path> --apply             # write changes
$ striatum workflow upgrade <path> --apply --check     # CI mode: exit 1 if diff
```

The verb opens `<path>` (a workflow.json file), runs the RFC 0034 V1
generator's fragment-merger against the file's harness_profiles, and
prints a diff. With `--apply`, it writes the file in place after
verifying:

1. The file is tracked by git.
2. The fields the merger would modify match the on-disk values (no
   uncommitted operator edits to those specific fields).
3. The file's `schema_version` is supported by the merger.

Refusal modes:

- **Untracked file.** `[error] file not tracked by git; commit or
  stash before running workflow upgrade`.
- **Concurrent edits to target fields.** `[error] field
  harness_profiles.codex.native_delegation.instruction has uncommitted
  changes; commit or revert before running workflow upgrade`.
- **Schema mismatch.** `[error] workflow schema_version X.Y not
  supported by upgrade tool; upgrade tool supports X.Z and above`.

### Footgun guards

The footguns `workflow upgrade` removes:

- Operators hand-editing harness profiles across N workflow.json files
  with copy-paste errors (the actual friction from dogfood-037..039).

The footguns `workflow upgrade` could introduce:

- Silently overwriting operator-customized fragments. Mitigation: the
  refuse-on-conflict check at field granularity.
- Drift between catalog defaults and existing workflows. Mitigation:
  the verb is the operator's tool to ENFORCE convergence; without it,
  drift accumulates.
- CI loops re-running `workflow upgrade --apply` and creating noisy
  commits. Mitigation: `--check` mode for CI; the verb is operator-
  invoked by default.

### Capability requirement

`workflow upgrade` is a CLI verb that mutates repository files, not
daemon state. It does not require a capability token; it does require
the operator's filesystem permissions on the workflow.json file. This
matches the existing `workflow validate --fix` precedent.

If a future iteration of this RFC exposes `workflow upgrade` over MCP,
it would require the `write` capability (the same gate as
`generate_workflow_write`) plus the same dry-run / refuse-on-conflict
semantics.

## Implementation Ordering and Test Coverage

The RFC's implementation plan (§§Step 1–6) is correct in shape. This
design recommends the following ordering refinements:

### Step 1 (MCP exposure of dogfood-lifecycle verbs)

Add the eleven thin-shell entries to
`src/striatum/web/chat_tools.py::_TOOLS`. Each entry's
`parameters.required` matches the underlying RPC method's required
params; each entry's capability gate is set via the existing
capability-aware `tool_schemas` / `execute_tool` plumbing. Test the
full matrix:

- **Capability matrix.** For each of the 11 tools, with each of the 7
  capabilities + 1 (the new surgical_recovery), verify `allowed`
  decision iff capability matches.
- **Audit-row append.** For each call, verify exactly one row in
  `daemon_audit` with the correct `decision` and (on denied) the
  correct `denial_reason`.
- **Reuse of existing RPC paths.** No new RPC methods; each MCP tool
  delegates to the existing path. Test by mocking the RPC entry
  point and verifying call shapes.

### Step 2 (`dogfood.publish_on_behalf`)

Add `_tool_dogfood_publish_on_behalf` to chat_tools.py (or extract to
`src/striatum/dogfood/operator_tools.py` if chat_tools.py grows past
~1000 lines). The composite path:

```python
def _tool_dogfood_publish_on_behalf(
    repo, session_id, artifact_path, artifact_kind, logical_name,
    verdict, findings_artifact_id, verdict_rationale, reason,
):
    with transaction(conn):
        lease = _lookup_active_lease_for_session(conn, session_id)
        if lease is None:
            return _error("no_active_lease", session_id=session_id)
        message = _lookup_claimed_message_for_job(conn, lease.job_id)
        if message is None:
            return _error("no_claimed_message", job_id=lease.job_id)
        # Step 1: ack
        _ack(conn, session_id, message.message_id, lease.lease_id)
        # Step 2: publish_artifact
        artifact_id = _publish_artifact(
            conn, session_id, lease.job_id, lease.lease_id,
            artifact_kind, logical_name, artifact_path,
        )
        # Step 3 (review jobs only): verdict
        if verdict is not None:
            _verdict(conn, session_id, lease.job_id, lease.lease_id,
                     verdict, findings_artifact_id, verdict_rationale)
        # Step 4: complete
        _complete(conn, session_id, lease.job_id, lease.lease_id,
                  summary="published_on_behalf: " + reason)
        _append_audit_row(
            conn, operation="publish_on_behalf",
            details={... composition_steps ...},
        )
    return _success({...})
```

Tests:

- **Happy path: handoff artifact.** Session in `claimed` state, ack
  denied externally, operator calls `publish_on_behalf`; verify all 3
  underlying mutations applied + single audit row.
- **Happy path: review artifact.** Same as above plus verdict step.
- **Failure rollback.** Inject a failure at the verdict step; verify
  ack and publish_artifact rolled back, audit row records rollback.
- **Capability denial.** Token lacks `write`; verify denial audit row
  + no state mutation.
- **Session ownership mismatch.** Session does not own the job;
  verify denial.
- **Path scope violation.** artifact_path outside write_scope; verify
  denial.

### Step 3 (`dogfood.surgical_recovery`)

Add `_tool_dogfood_surgical_recovery` with the same shape as
`publish_on_behalf`. Add the `surgical_recovery` capability to the
registry. Add the `job_advisory_locks` table migration.

Tests:

- **Happy path.** Job in `stale_lease`, expected artifacts on disk,
  supervisor in `lost`; surgical_recovery succeeds; verify lease
  reactivated, supervisor reattached, queue message reverted, single
  audit row.
- **Precondition: artifacts missing.** Same as above but artifacts
  not on disk; verify denial with
  `surgical_recovery_validation_failed`.
- **Precondition: concurrent supervisor.** Supervisor in `attached`
  not `lost`; verify denial.
- **Precondition: bad job state.** Job already `completed`; verify
  denial.
- **Capability denial: missing.** Token lacks `surgical_recovery`;
  verify `capability_missing`.
- **Capability denial: repo-scoped.** Token has `surgical_recovery`
  but row has non-null repository_id; verify
  `capability_scope_mismatch`.
- **TTL clamp at issuance.** `daemon.token.create --capability
  surgical_recovery --expires-in 24h`; verify response includes
  `ttl_clamped: true` and the token expires in 15m.
- **Repo-scoping refusal at issuance.** Same with `--repo X`; verify
  RpcError schema_invalid.
- **Race with watcher.** Mock watcher refresh during
  surgical_recovery; verify watcher skips refresh (lock held); verify
  surgical_recovery commits cleanly.
- **Confirm gates.** Missing confirm_write / operator_confirmed /
  allow_mutations each produce the right error.

### Step 4 (supervised-progress watcher)

Add `src/striatum/daemon_supervisor/progress_watcher.py`. Add the
watcher pool to the daemon's startup sequence. Add the synthetic
auth-context plumbing.

Tests:

- **Refresh on growth.** Write to log file; verify watcher refreshes
  lease within one tick.
- **No refresh on no growth.** Quiet log file; verify lease expires
  normally.
- **Crash detection.** Kill the supervisor PID; verify watcher marks
  supervisor lost and exits.
- **Daemon restart.** Stop daemon mid-run; restart; verify watchers
  re-spawn for `attached` supervisors and skip `lost` ones.
- **Lock contention.** Start surgical_recovery; verify watcher skips
  refresh during the recovery transaction.
- **Multi-supervisor isolation.** Two supervisors, two logs; verify
  each watcher refreshes only its own lease.
- **Audit-row append.** Verify each refresh writes one
  `daemon_audit` row with `client_id =
  "_daemon_supervised_progress"`.

### Step 5 (harness profile fragments + `workflow upgrade`)

Add the corrected fragments to the RFC 0034 V1 catalog. Add the
`workflow upgrade` CLI verb.

Tests:

- **Generator emits fragments by default.** `striatum workflow init`
  produces workflow.json with the no-questions + front-matter callout
  in the right profiles.
- **Upgrade verb dry-run.** Run against an old workflow.json; verify
  no file modification + diff output.
- **Upgrade verb apply.** Same with --apply; verify file written.
- **Upgrade verb refuse-on-conflict.** Modify the target field
  manually, leave uncommitted; run upgrade; verify refusal.
- **Upgrade verb idempotence.** Run upgrade twice; second run is a
  no-op.
- **Backport against dogfood-035..039.** Each existing workflow
  upgrades cleanly; manual verification of resulting JSON.

### Step 6 (documentation + RFC 0035 multi-repo test harness)

Update `docs/MCP.md`, `docs/HOW_TO_HUMAN.md`, `docs/HOW_TO_AGENT.md`,
and create `docs/HARNESS_FRICTION_PATTERNS.md`. Extend the RFC 0035
test harness with end-to-end coverage of the new composite tools and
the watcher.

The friction-patterns doc should NOT name specific Engram-related
content; it should describe the three patterns (036 strategy-then-
exit, 037 ask-question-and-exit, 038 lease-expiry-under-active-load)
with the fixes that landed. Cross-link to the relevant OPERATOR_REPORT
entries.

## Risk Register

| Risk | Severity | Mitigation |
|---|---|---|
| Surgical_recovery token leak | High | 15m TTL ceiling; no repo scoping; admin-only issuance; revocation via `daemon.token.revoke` |
| Composite tool partial commit | Medium | All-or-nothing transaction; rollback audit-row outcome |
| Watcher refreshes dead supervisor's lease | Medium | PID liveness probe; mark supervisor `lost` on detection |
| Watcher races surgical_recovery | Medium | `job_advisory_locks` table; watcher skips refresh when lock held |
| Watcher false-negative (log quiet during work) | Low | Operator falls back to surgical_recovery; threshold configurable |
| `workflow upgrade` overwrites operator edits | Medium | Field-granular refuse-on-conflict against git tracked state |
| Operator confuses `recovery` and `surgical_recovery` capabilities | Low | Distinct names; admin-only issuance for surgical_recovery; documentation in MCP.md |
| Audit-row composition shape change breaks downstream tooling | Low | The composed shape is additive; existing rows for non-composite ops unchanged |

## Open Questions Left for Implementer

I am closing the four RFC open questions per the design recommendation
above, but two items would benefit from operator-side validation
during build:

1. **The 15m TTL ceiling for surgical_recovery tokens** is a guess
   calibrated to "long enough to inspect the job, short enough that
   stale shell history isn't a footgun." Build-stage validation:
   verify that a single surgical_recovery operation completes in well
   under 15m end-to-end (token mint, operator inspection, tool call,
   audit-row commit). If the median real run is > 5m, consider
   extending the cap to 30m; if it's < 2m, consider 10m.
2. **The mtime-vs-sidecar choice for the watcher's growth signal** is
   landed on mtime per this design. If the build-stage tests reveal
   that the supervised wrappers (codex / claude / gemini) have long
   stretches of "thinking time" with no log writes (>10 minutes,
   the default idle threshold), the watcher will not refresh and the
   lease will expire mid-task. The friction pattern would then look
   identical to today's pattern, and the RFC's main fix would be
   ineffective. If that pattern emerges in build-stage testing, fall
   back to the sidecar-signal approach: have each supervised wrapper
   write a "still alive" line to its log every N seconds even when
   idle. This is a wrapper-side change, not a watcher-side one.

These are NOT blockers for landing the RFC; they are calibration
items that the build phase will surface.

## Out-of-Scope Reminders

Per the design prompt and the RFC:

- Hosted-mode multi-tenant MCP (D083). The capability gate is
  single-user owner-only; multi-tenant is a future RFC.
- Operator-side automation of `dogfood.publish_on_behalf` (no
  auto-detect of ack-denied; the operator's session calls it
  explicitly).
- Self-healing supervised wrappers (this RFC adds the watcher to the
  daemon side; wrappers stay one-shot per RFC 0030 / 0036).
- Backporting to the Go daemon (RFC 0039 §6 picks up the same surface
  independently when the Go core lands).

## Summary for Implementers

The smallest viable build slice is:

1. Add the `surgical_recovery` capability to the registry +
   `daemon.token.create` issuance rules.
2. Add the eleven thin-shell MCP tools to chat_tools.py.
3. Add `dogfood.publish_on_behalf` and `dogfood.surgical_recovery` as
   composite tools with their respective confirmation gates.
4. Add the `job_advisory_locks` table + the watcher pool.
5. Add the corrected harness-profile fragments + `workflow upgrade`
   CLI verb.
6. Land the audit-row shape changes + the documentation pass.

Each slice is independently testable; slices 1–3 deliver the
operator-side friction reduction, slice 4 delivers the supervised-side
fix, slice 5 closes the harness-profile drift, slice 6 makes the
whole thing inspectable.

The four trust boundaries — capability authorization (concern 1),
audit-chain semantics (concern 2), operator confirmation (concern 3),
and watcher-vs-lease concurrency (concern 4) — are the load-bearing
invariants. Every other implementation choice rolls back to one of
those four.
