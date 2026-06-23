# RFC 0165 Holder Proposal: Spawn-Time Claude Credential Hydration
author: holder-author-001

## Claim

Claude credential freshness must become a synchronous launch invariant, not a
background hygiene task. Before any real Claude lane process starts, Striatum
should hydrate the lane OS user's Claude OAuth credential from the operator's
current credential, verify that the lane file matches the current source
generation, persist a redacted custody receipt, and refuse launch when freshness
cannot be proven.

This proposal is deliberately narrower than a provider-auth broker. It fixes GH
#583 by making the current local-file Claude CLI model safe enough for supervised
lanes. It does not move Striatum capability tokens, does not give lanes daemon or
admin credentials, and does not ask a lane to refresh the operator's OAuth login.

## Current Source Anchors

- `go/pkg/mutations/supervision_control.go::HandleSuperviseStart` already calls
  `loadSupervisionStartConfig`, then `runSuperviseProviderAuthGate`, before it
  creates `.striatum/scratch`, mints the session-bound Striatum token, inserts
  `process_supervisors` rows, records `supervisor.starting`, or calls
  `launchSupervisedProcess`.
- `go/pkg/mutations/supervision_provider_auth.go::runSuperviseProviderAuthGate`
  is currently Codex-only. Claude is unsupported in `auto`; `required` returns
  `lane_provider_preflight_unsupported`.
- `go/pkg/laneproviderauth/resolver.go::ResolveCredential` already has a Claude
  resolver for `$CLAUDE_CONFIG_DIR/.credentials.json`, falling back to
  `$HOME/.claude/.credentials.json`. It fails closed when no runtime credential
  source can be proven.
- `go/pkg/laneproviderauth/expiry.go::ParseExpiry` already parses Claude OAuth
  `claudeAiOauth.expiresAt` and top-level `expiresAt`.
- `go/pkg/laneproviderauth/sampler.go::LaneFileReader` already demonstrates the
  bounded `sudo -n -u <lane> -- env -i ... cat` shape for reading lane-owned
  credential files without leaking inherited daemon environment.
- `go/pkg/mutations/supervision_env.go::providerAuthPreflightEnv` sanitizes
  provider env before checks. Its current allowlist includes `CODEX_HOME` but not
  `CLAUDE_CONFIG_DIR`; a Claude implementation must fix that or explicitly fail
  closed when a trusted Claude config dir is required.
- `go/pkg/sessionliveness/liveness.go` classifies lanes that never bind MCP as
  `agent_mcp_discovery_stall`; `go/pkg/mutations/recovery_decision_tree.go`
  turns that generic stall into same-attempt requeue, transfer, or
  `recovery_exhausted` through `job_recovery_state`.

The future implementation run needs source write scope outside this design
workflow's artifact-only lanes, at minimum:

- `go/pkg/laneproviderauth/`
- `go/pkg/mutations/supervision_provider_auth.go`
- `go/pkg/mutations/supervision_control.go`
- `go/pkg/mutations/supervision_env.go`
- `go/pkg/mutations/recovery_decision_tree.go`
- `go/pkg/reads/doctor_lane_provider_auth.go`
- `go/pkg/rpc/error_catalog.go`
- `docs/reference/command-authority-matrix.md`
- the PostgreSQL migration directory and the generated/guarded daemon contract
  docs if new RPC params or tables are added

## Implementation Spec

### 1. Add a Claude Hydrator in `laneproviderauth`

Add a provider-specific module, for example
`go/pkg/laneproviderauth/claude_hydrator.go`, with fakeable filesystem,
clock, user lookup, and runner seams:

```text
HydrateClaudeCredential(ctx, params) -> HydrationReceipt
```

Inputs:

- `Provider`: closed enum `claude`
- `Kind`: closed enum `oauth`
- `RepositoryID`, `RunID`, `SessionID`, `LaneID`
- `RunAsUser`: the configured lane OS user after same-user collapse
- `OperatorEnv`: trusted daemon/operator environment, not workflow-authored
- `LaneLaunchEnv`: the env the Claude lane will resolve credentials from
- `MinFreshness`: fixed V1 default `30m`

The hydrator does four things in order:

1. Resolve the operator source credential from trusted operator state:
   `$CLAUDE_CONFIG_DIR/.credentials.json` if `CLAUDE_CONFIG_DIR` is present in
   the daemon/operator environment, else the daemon user's
   `$HOME/.claude/.credentials.json`. If neither is trustworthy, refuse with
   `provider_credential_resolver_mismatch`.
2. Resolve the lane destination credential from the lane identity, not from an
   arbitrary workflow path. V1 supports:
   - same-user collapse: `RunAsUser == ""`; destination is the same resolved
     operator credential and hydration is a verify-only no-op;
   - distinct lane user default: lane OS user's home plus
     `.claude/.credentials.json`;
   - optional daemon/operator-configured lane Claude config dir, if added later.
     A workflow `command_env.CLAUDE_CONFIG_DIR` is accepted only when it matches
     this trusted destination. Any other workflow-authored Claude config dir
     fails closed with `provider_credential_resolver_mismatch` because copying
     OAuth material into an arbitrary path would be a privilege bridge.
3. Observe the source generation, copy source bytes to the lane destination by
   temp file plus rename, and verify destination generation. If the source
   generation changes between pre-copy and post-copy observation, retry once.
   If it changes again, refuse with
   `provider_credential_source_unstable`.
4. Verify destination owner, mode, parseability, expiry, and generation match.
   Distinct lane-user destination must be lane-owned `0600`. Same-user
   verify-only mode verifies the operator-owned source path but does not chown
   or chmod it.

Credential generation is a non-secret value:

- provider, kind, source/destination selector;
- HMAC-SHA256 over file bytes using the daemon authority secret, never a raw
  unsalted hash;
- `expires_at` parsed by `ParseExpiry`;
- file size, mode, uid/gid or owner name, mtime;
- observed time.

For Claude OAuth, `ParseExpiry(...).HasExpiry == false` is not acceptable for
launch. It is an unverifiable OAuth credential and must fail closed. Launch also
fails when `expires_at <= now + 30m`.

### 2. Path and Ownership Rules

The hydrator must treat path resolution as a security boundary.

Source rules:

- Source path comes only from trusted daemon/operator env and the operator home.
- A full operator home path must not be persisted to events, metrics, doctor,
  dashboard, repo artifacts, GitHub comments, or error details.
- Source must be a regular file. Symlink source paths are refused unless a
  future maintainer decision explicitly permits and bounds them.

Destination rules:

- Destination path comes from the lane OS identity and a trusted config selector,
  not from workflow-authored arbitrary paths.
- Every destination parent component must stay under the trusted lane credential
  directory after symlink evaluation.
- The destination parent may be created only when the trusted selector is the
  lane home default or a daemon-configured lane config dir. Create directory
  mode `0700`, lane-owned.
- The destination file is written through a temp file in the destination
  directory, chmodded `0600`, chowned to the lane uid/gid, fsynced where
  practical, and atomically renamed.
- Existing destination symlinks, non-regular files, wrong owner, or wrong mode
  are refusal conditions unless the hydrator overwrote them through the safe temp
  file path and the final file verifies as regular lane-owned `0600`.

### 3. Launch Integration

Integrate Claude hydration at the existing `supervise.start` authority point.
The order should become:

1. `HandleSuperviseStart` resolves `session_id`, `provider_auth_gate`, and
   `supervisionStartConfig`.
2. If `config.adapterName() == "claude"` and agent-loop mode is self-driving,
   run `runSuperviseClaudeCredentialGate` before scratch creation and before
   session-bound Striatum token minting.
3. Existing Codex provider-auth preflight remains in
   `runSuperviseProviderAuthGate`.
4. Only after Claude hydration succeeds may the handler create scratch, prepare
   ACLs, mint/inject the Striatum session-bound token, insert supervisor rows,
   and launch the PTY/helper or pipe process.

`provider_auth_gate=off` must not bypass Claude hydration. That flag is an
emergency rollback for provider CLI smoke/probe behavior from RFC 0121. Claude
hydration moves rotating OAuth material into the lane's credential home and is
the correctness boundary for #583.

If an emergency bypass is required, add a separate explicit launch parameter,
for example `provider_credential_hydration=off`, defaulting to `auto`. It must:

- be documented as unsafe;
- emit a redacted `provider_credential.hydration_bypassed` event;
- mark the provider-auth dependency `disabled`;
- be forwarded by `run drive` only when explicitly requested;
- never be implied by `provider_auth_gate=off`.

### 4. Durable State and Receipts

Use a hybrid: queryable PostgreSQL tables plus redacted timeline events.

Add a current-state table keyed by repository, provider, kind, lane user, and
destination selector:

```text
striatumd.provider_auth_dependencies
  repository_id
  provider
  kind
  lane_user
  destination_selector
  state                         -- ready|hydrating|reseed_required|unverifiable|disabled
  source_selector
  source_generation_id
  destination_generation_id
  expires_at
  min_freshness_seconds
  last_receipt_id
  last_failure_class
  last_failure_reason
  updated_at
```

Add append-only custody receipts with bounded retention, for example last 100
receipts per dependency key or 30 days, whichever keeps more diagnostic value:

```text
striatumd.provider_credential_custody_receipts
  receipt_id
  repository_id
  run_id
  session_id
  lane_id
  lane_user
  provider
  kind
  source_selector
  destination_selector
  source_generation_id
  destination_generation_id
  source_observed_at_before
  source_observed_at_after
  hydration_started_at
  hydration_completed_at
  expires_at
  min_freshness_seconds
  destination_owner_ok
  destination_mode_ok
  destination_parse_ok
  verifier_result              -- passed|source_missing|source_unstable|source_unparseable|destination_write_failed|destination_unparseable|expiry_too_near|owner_mode_invalid|resolver_mismatch
```

Emit compact events for run history:

- `provider_credential.hydrated`
- `provider_credential.hydration_refused`
- `provider_auth.reseed_required`
- `provider_auth.reseed_cleared`

Event payloads carry ids, enum selectors, provider, kind, lane user, expiry,
failure class, and receipt id. They do not carry credential bytes, token values,
full private paths, stdout/stderr, or provider output.

### 5. Error Codes and Operator Remediation

Add stable daemon errors:

- `provider_credential_hydration_failed`
- `provider_credential_generation_stale`
- `provider_credential_source_unstable`
- `provider_credential_expiry_too_near`
- `provider_credential_owner_mode_invalid`
- `provider_credential_resolver_mismatch`
- `provider_auth_reseed_required`

The operator-facing remediation for source-missing, source-unparseable, or
expiry-too-near cases should be exact and private-safe:

```text
Claude provider credential is not fresh enough for lane launch. Re-authorize or
refresh Claude as the operator user, then retry the run. No lane process was
started.
```

Doctor/dashboard may add:

```text
provider=claude lane_user=striatum-lane state=reseed_required
reason=provider_credential_expiry_too_near expires_at=<timestamp>
action=refresh_operator_claude_login_then_retry
```

They must not show `~halbritt`, `~striatum-lane`, raw JSON, OAuth access tokens,
refresh tokens, id tokens, provider stdout/stderr, daemon runtime token paths, or
capability tokens.

### 6. Recovery Circuit Breaker

Recovery must prefer provider-auth evidence over generic MCP-discovery retry.
When a Claude lane hits `agent_mcp_discovery_stall`, recovery should check the
latest Claude provider-auth dependency and receipts for that lane user before it
spends `requeue_count` or `transfer_count`.

Behavior:

- If a fresh source generation is available and hydration can now succeed,
  hydration should clear `reseed_required` and recovery may requeue the job once.
- If the latest source is missing, unparseable, too close to expiry, unstable, or
  resolver-mismatched, recovery sets dependency state to `reseed_required` or
  `unverifiable`, emits one redacted event, and does not increment the generic
  requeue/transfer counters for this provider-auth cause.
- If a job already has an open provider-auth blocker for the same dependency
  generation, subsequent sweeps are idempotent no-ops.
- When a newer source generation hydrates successfully, only jobs blocked
  against the stale generation are eligible to requeue. The receipt's generation
  ids are what prevent a stale blocker from clearing against the same bad source.

This is the key #583 invariant: stale Claude OAuth must become provider-auth
readiness debt, not another `agent_mcp_discovery_stall` that burns recovery
budget and escalates as `recovery_exhausted`.

### 7. Redaction Contract

Persist allowed:

- closed enums: provider, kind, state, selector, failure class;
- lane user and lane id;
- run/session/job ids;
- receipt id;
- HMAC generation ids computed with the daemon authority secret;
- expiry timestamp, observed timestamp, size, mode, owner/mode booleans;
- safe remediation strings.

Forbidden everywhere outside transient process memory:

- raw OAuth credential bytes;
- Claude access tokens, refresh tokens, id tokens, account ids if present;
- full private operator or lane credential paths;
- provider stdout/stderr, transcript text, model output;
- daemon bootstrap admin token, runtime `client-token`, session-bound
  Striatum tokens, capability tokens, DSNs;
- raw unsalted hashes of credential bytes.

The implementation should include a table-driven redaction test that serializes
every receipt/event/error/doctor/dashboard payload and searches for fixture
secrets and private path substrings.

## TDD Build Order

1. Add failing unit tests around a fake Claude hydrator filesystem:
   happy path, stale lane generation overwritten, source rotation during copy,
   wrong owner/mode, destination symlink escape, unparseable JSON, missing
   expiry, expiry inside 30m, and redaction.
2. Add migration and data access tests for
   `provider_auth_dependencies` and custody receipts. Verify upsert,
   idempotent same-generation refusal, retention, and event payload shape.
3. Implement Claude source/destination resolution and generation observation in
   `laneproviderauth`, reusing `ResolveCredential`, `ParseExpiry`, and
   `LaneFileReader` patterns where they fit. Add `CLAUDE_CONFIG_DIR` to the
   safe env path only when it is a trusted selector.
4. Wire `HandleSuperviseStart` so a stale or unverifiable Claude credential
   refuses before supervisor rows, scratch, lane-token minting, helper/tmux, or
   real Claude process launch.
5. Extend `run drive`, auto-spawn, doctor, dashboard, and error catalog so the
   refusal is visible and does not loop as an ordinary provider CLI or MCP
   discovery failure.
6. Teach recovery to consult provider-auth dependency state before generic
   `agent_mcp_discovery_stall` requeue/transfer, and add the
   `reseed_required` idempotency tests.
7. Update `docs/reference/command-authority-matrix.md`,
   `docs/reference/spec.md`, CLI/RPC docs, and decision-log entries required by
   the new daemon state and error codes.

## Required Tests

- `TestClaudeHydrationHappyPathBeforeLaunch`: stale lane credential is replaced
  and verified before launch; then supervisor rows/process launch may occur.
- `TestClaudeHydrationRefusesBeforeSupervisorRows`: expired, missing,
  unparseable, or expiry-too-near source returns a provider credential error and
  no scratch, token mint, supervisor rows, helper, tmux, or Claude process exist.
- `TestClaudeHydrationSourceRotationRace`: source generation A before copy and B
  after copy retries once; a second drift refuses with
  `provider_credential_source_unstable`.
- `TestClaudeHydrationDestinationOwnerMode`: final file must be regular,
  lane-owned, and `0600`; wrong owner/mode/non-regular destination refuses.
- `TestClaudeHydrationSymlinkEscape`: symlinked destination parent or final path
  escaping the trusted lane credential dir refuses.
- `TestClaudeHydrationResolverRejectsWorkflowPath`: workflow-authored
  `CLAUDE_CONFIG_DIR` pointing outside the trusted lane credential dir fails
  closed and does not copy credential bytes.
- `TestProviderAuthGateOffDoesNotBypassClaudeHydration`: `provider_auth_gate=off`
  still runs Claude hydration; only the separate emergency hydration bypass can
  skip it, and that bypass records a `disabled` dependency.
- `TestClaudeHydrationReceiptRedaction`: fixture token strings, credential JSON,
  raw hashes, and private path substrings are absent from events, receipts,
  errors, doctor, dashboard, and metrics.
- `TestRecoveryProviderAuthCircuitBreaker`: a Claude
  `agent_mcp_discovery_stall` with stale/unverifiable provider auth records
  `reseed_required`, does not increment generic requeue/transfer counters, and
  does not escalate as `recovery_exhausted`.
- `TestProviderAuthReseedClearedByNewGeneration`: a newer source generation
  hydrates, clears the dependency, and permits exactly the jobs blocked against
  the stale generation to requeue.

## Load-Bearing Claims

| Claim | Evidence that supports it | Observation that refutes it |
|---|---|---|
| Claude hydration runs before real provider launch. | A stale Claude source causes `supervise.start` to return a provider credential error with no supervisor rows, scratch, session token mint, helper, tmux pane, or Claude process. | Any test or incident shows a real Claude process starts, then later wedges in MCP discovery because hydration failed. |
| Resolution does not become a privilege bridge. | Source selectors are daemon/operator-owned; destination selectors are lane-home or daemon-configured; workflow `CLAUDE_CONFIG_DIR` escape tests fail closed without copying bytes. | A workflow can set `CLAUDE_CONFIG_DIR` to an arbitrary writable path and cause the daemon to copy operator OAuth bytes there. |
| Rotation race is closed. | A test that changes source generation during copy either retries into one stable generation or fails `provider_credential_source_unstable`. | Destination receipt claims source generation A while the post-copy source observation is generation B. |
| Custody records are useful but private-safe. | Doctor/dashboard/recovery can join dependency state to the latest receipt id and failure class, while redaction tests prove no raw credential, token, or full private path is emitted. | Operators cannot tell why launch refused, or any emitted payload contains OAuth bytes, token substrings, raw hashes, or private path strings. |
| `provider_auth_gate=off` does not bypass the fix. | A test with `provider_auth_gate=off` still invokes Claude hydration and refuses stale credentials. | `provider_auth_gate=off` launches Claude with stale lane credentials. |
| Recovery stops burning generic retry budget on stale provider auth. | A stale Claude credential stall records provider-auth readiness debt and no generic requeue/transfer increment for that cause. | The same stale generation produces repeated same-attempt requeues and finally `recovery_exhausted`. |
| Freshness lead time is deterministic. | Credentials expiring at `now + 30m` or earlier refuse; credentials beyond the lead pass. | Launch decisions vary by workflow input or observed credential lifetime rather than the configured/constant lead. |
| Striatum control-plane credentials stay separate. | Hydrator inputs/outputs and receipts never read or copy `STRIATUM_MCP_TOKEN`, runtime `client-token`, daemon admin tokens, DSNs, or capability tokens. | Any code path copies, stores, or exposes a Striatum control-plane token while hydrating Claude provider OAuth. |

## Minimal Closure Scope for GH #583

Closure requires code on `origin/main` plus verifier evidence that:

- a Claude lane whose lane-local credential is stale is refreshed or refused
  before a real Claude process starts;
- a long dogfood spanning more than the Claude access-token TTL does not wedge
  new Claude lanes on credential-caused `agent_mcp_discovery_stall`;
- no manual `cp`, `chown`, `chmod`, or escalation resolution is needed for the
  #583 expiry class;
- provider-auth telemetry from RFC 0162 still reports expiry/absence, but launch
  correctness does not depend on the timer or metrics path.

An optional host timer in `halbritt/proximal` may pre-warm lane credentials every
20 to 30 minutes by calling the same hydrator or a thin wrapper around it. It is
not part of the correctness boundary. If the timer is absent or late, the
spawn-time gate must still hydrate or refuse synchronously.
