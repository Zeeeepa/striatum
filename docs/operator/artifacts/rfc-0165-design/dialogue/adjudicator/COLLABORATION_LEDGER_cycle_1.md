---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-author-001
workflow: "rfc-0165-design"
run_id: "run_471c0965378cefa54c31015a74ad3162"
cycle: 1
topic: "RFC 0165 Claude provider credential freshness hydration for GH #583"
participants:
  - "holder-author-001"
  - "falsifier-reviewer-001"
  - "falsifier-reviewer-002"
  - "adjudicator-author-001"
entries:
  - kind: claim
    by: "holder-author-001"
    refs: ["dialogue:1"]
    text: >-
      The holder proposes a spawn-time Claude OAuth hydrator: resolve trusted operator and lane credential paths, copy and verify the operator credential before any real Claude process starts, persist redacted custody receipts, keep provider OAuth separate from Striatum control-plane tokens, make provider_auth_gate=off unable to bypass hydration, and route stale provider auth into reseed_required rather than generic MCP discovery retry.
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: >-
      Falsifier 1 shows the proposal's recovery circuit breaker is still blind to runtime credential expiry after a successful spawn-time hydration. A lane can launch with 35 minutes of freshness, do 45 minutes of local work, expire before its first model/MCP action, then enter agent_mcp_discovery_stall while the stored dependency still says ready and the latest receipt says passed; recovery therefore burns generic retry budget before a later requeue discovers reseed_required.
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: >-
      Falsifier 2 shows a deeper OAuth refresh-token-rotation gap. Copying the operator Claude credential file into lane homes gives lanes their own refresh token copy; when any lane refreshes, the OAuth server can rotate and invalidate the prior refresh token while the operator source file is not updated, causing concurrent/subsequent lanes and the operator login to use stale or invalidated refresh tokens.
  - kind: constraint
    by: "adjudicator-author-001"
    refs: ["dialogue:2", "dialogue:3"]
    text: >-
      Revision must replace or substantially constrain the local-file copy model so runtime refresh, concurrent lane refresh, and subsequent lane launch cannot desynchronize or invalidate the operator credential source.
  - kind: constraint
    by: "adjudicator-author-001"
    refs: ["dialogue:2"]
    text: >-
      Revision must make recovery classify runtime credential expiry as provider-auth readiness debt before consuming generic requeue or transfer budget.
verdict: "needs_revision"
rationale: >-
  Both falsifier challenges are material and unrebutted by the proposal as written. The holder resolves the seed's launch-time decisions at a useful level of concreteness, but the proposed local-file JIT hydration does not make GH #583 safe for long-running or concurrent Claude lanes: runtime expiry still leaks into generic MCP-discovery recovery, and refresh-token rotation can desynchronize the copied lane credential from the operator source credential. These failures attack the core correctness boundary, not just observability or polish, so the dialogue does not clear. The next revision must either move to a daemon-managed credential broker/access-token IPC model or prove an equally concrete OAuth-safe design that prevents lane refresh-token custody and operator-source invalidation, and it must add recovery-time freshness classification so stale runtime provider auth cannot burn generic retry budget.
findings:
  - id: F1-RUNTIME-EXPIRY-CIRCUIT-BREAKER
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:2"]
    affected_invariants:
      - "stale Claude provider auth must become readiness debt, not generic MCP discovery retry"
      - "recovery must not burn generic retry budget for provider-auth causes"
    challenge: >-
      Spawn-time hydration can pass, then the lane credential can expire before the lane first needs Claude. The latest dependency and receipt still say ready/passed, so recovery has no current provider-auth evidence and treats the stall as generic, burning retry or transfer budget before reseed_required is discovered on a later launch.
    closest_acceptable_answer: >-
      Recovery must perform a current freshness check against the lane credential or an equivalent daemon-owned provider-auth state before incrementing generic counters, and the supervisor/heartbeat path must report provider credential expiry or near-expiry as provider-auth debt before ordinary stall recovery fires.
    requested_constraint_shape:
      kind: gate
  - id: F2-REFRESH-TOKEN-ROTATION-DESYNC
    severity: critical
    posture: design
    status: open
    source_refs: ["dialogue:3"]
    affected_invariants:
      - "Claude OAuth credential custody must not corrupt or invalidate the operator source login"
      - "concurrent and subsequent lanes must not share stale copied refresh-token state"
      - "provider OAuth credentials and Striatum control-plane credentials remain separate"
    challenge: >-
      The JIT file-copy model gives lanes raw refresh-token custody. With OAuth refresh token rotation, a lane-side refresh can rotate the token in the lane file while the operator source file remains stale, invalidating the source for future lanes and possibly the operator's own CLI session.
    closest_acceptable_answer: >-
      Use a daemon-managed credential broker or an equally explicit OAuth-safe design where lanes do not receive raw refresh tokens and cannot independently rotate the operator credential family; add concurrent-lane and subsequent-lane RTR tests.
    requested_constraint_shape:
      kind: gate
constraints:
  - id: C1-NO-LANE-RAW-REFRESH-TOKEN-CUSTODY
    posture: credential_custody
    severity: critical
    kind: gate
    binding: true
    source_finding: F2-REFRESH-TOKEN-ROTATION-DESYNC
    source_refs: ["dialogue:3"]
    text: >-
      The revised implementation spec must prevent lanes from receiving or independently refreshing raw Claude OAuth refresh tokens. If it chooses a daemon-managed broker, name the IPC/MCP surface, the token lifetime, the caller identity checks, and how the Claude CLI/SDK is configured to use it. If it keeps any file-copy design, prove how refresh-token rotation cannot desynchronize or invalidate the operator source credential.
    verification:
      gate: "Tests simulate lane-side OAuth refresh, concurrent lane refresh, and subsequent lane launch; operator source credentials remain valid and no lane stores a raw refresh token."
    final_review_required: true
  - id: C2-RUNTIME-FRESHNESS-RECOVERY-CLASSIFICATION
    posture: recovery
    severity: high
    kind: gate
    binding: true
    source_finding: F1-RUNTIME-EXPIRY-CIRCUIT-BREAKER
    source_refs: ["dialogue:2"]
    text: >-
      The revised spec must add recovery-time provider-auth freshness classification for Claude stalls, using current credential freshness or broker state rather than only the spawn-time receipt. A runtime-expired or near-expiry credential must produce provider-auth debt and must not increment generic requeue or transfer counters.
    verification:
      gate: "A long-running lane whose credential expires after successful launch enters provider-auth reseed_required/unverifiable without consuming generic recovery budget or escalating as recovery_exhausted."
    final_review_required: true
  - id: C3-HEARTBEAT-OR-BROKER-TELEMETRY-FOR-DECAY
    posture: observability
    severity: high
    kind: gate
    binding: true
    source_finding: F1-RUNTIME-EXPIRY-CIRCUIT-BREAKER
    source_refs: ["dialogue:2"]
    text: >-
      The revised spec must name the daemon-owned signal that detects credential decay while a lane is running: either supervisor heartbeat freshness telemetry over a trusted channel, broker-owned token state, or an equivalent mechanism that does not trust lane-authored claims or raw provider output.
    verification:
      gate: "Heartbeat or broker-state tests report provider_credential_expired/expiry_too_near before generic MCP-discovery recovery handles the same lane."
    final_review_required: true
  - id: C4-TEST-MATRIX-COVERS-LONG-RUN-AND-RTR
    posture: verification
    severity: critical
    kind: gate
    binding: true
    source_finding: F2-REFRESH-TOKEN-ROTATION-DESYNC
    source_refs: ["dialogue:2", "dialogue:3"]
    text: >-
      The revised proposal's required tests must include long-running lanes crossing the access-token expiry boundary, concurrent lanes sharing an initial operator credential generation, subsequent lane launch after one lane refreshes, and redaction checks proving no raw OAuth bytes, refresh tokens, access tokens, private paths, provider output, or Striatum control-plane tokens are emitted.
    verification:
      expected_stage: "The next PROPOSAL.md names these tests and maps each to the source modules and state transitions it exercises."
    final_review_required: true
branches:
  design: blocked
---

# Collaboration Ledger - RFC 0165 design run (cycle 1)

author: adjudicator-author-001

## Verdict

**verdict: needs_revision**

The holder proposal is a serious launch-time design and resolves many required decisions from the seed: it names the supervise.start integration point, trusted source/destination resolution, source-generation race handling, redacted custody state, the provider_auth_gate=off boundary, recovery debt, a 30 minute freshness window, private-safe operator remediation, and a TDD build order. That is not enough to clear this gate.

Two material falsifier challenges landed and stand unrebutted by the proposal as written.

First, `falsifier-reviewer-001` shows that a lane can pass spawn-time freshness and still expire before it first needs Claude. In that case the persisted dependency still says `ready` and the latest receipt still says `passed`, so recovery sees an `agent_mcp_discovery_stall` with no current provider-auth evidence and burns generic recovery budget. The holder's 30 minute freshness lead time only moves the boundary; it does not classify runtime decay before generic recovery acts.

Second, `falsifier-reviewer-002` shows the local-file copy model is unsafe for rotating OAuth refresh tokens. If a copied lane credential refreshes, the lane-side file can receive the rotated refresh token while the operator source file remains stale. Future lanes then hydrate from an invalidated source credential, and the operator login can also be damaged. This directly attacks the claim that local-file Claude CLI usage can be made safe enough by JIT copy-hydration.

## Binding Revision Constraints

The next revision must close both gaps before downstream proposal publication:

1. Prevent lane custody of raw Claude refresh tokens, or prove an equally concrete OAuth-safe file model. A daemon-managed broker/access-token IPC design is the most direct shape; any alternative must explicitly handle refresh-token rotation, concurrent lanes, subsequent lanes, and operator-source validity.
2. Add recovery-time freshness classification so runtime-expired Claude credentials become provider-auth readiness debt before generic requeue/transfer counters are touched.
3. Name a trusted heartbeat, broker-state, or equivalent daemon-owned telemetry path for credential decay while a lane is running; do not rely on lane-authored claims or provider stdout/stderr.
4. Add tests for post-launch expiry, concurrent refresh-token rotation, subsequent lane launch after refresh, operator-source validity, and redaction of OAuth material, private paths, provider output, and Striatum control-plane tokens.

## Preserve

The revision should preserve the holder's useful work: launch refusal before supervisor rows/scratch/token mint/helper/Claude process, trusted path resolution with workflow path escape refusal, source-generation race checks, redacted custody receipts, private-safe doctor/dashboard messages, and the separation between provider OAuth credentials and Striatum capability/control-plane credentials.
