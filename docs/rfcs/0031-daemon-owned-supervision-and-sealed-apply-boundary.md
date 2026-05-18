# RFC 0031: Daemon-Owned Supervision and Sealed-Apply Boundary

Status: accepted (V2 foundation)
Date: 2026-05-11
Context:
[`RFC 0028`](0028-long-running-daemon-and-multi-repository-control-plane.md),
[`RFC 0030`](0030-daemon-rpc-server-and-version-skew-protocol.md) (accepted V2),
[`RFC 0033`](0033-storage-substrate-rewrite-for-daemon-v2.md) (accepted V2),
[`RFC 0009`](0009-long-lived-process-supervision.md),
[`RFC 0010`](0010-tool-harness-profiles.md),
[`RFC 0014`](0014-process-adapter-completion-guarantees.md),
[`RFC 0026`](0026-lane-attestation-and-operator-byline-honesty.md),
[`RFC 0027`](0027-sealed-patch-provenance-mode.md),
[`docs/DECISION_LOG.md`](../DECISION_LOG.md) (D080, D082, D086, D028, D036),
`src/striatum/supervisor.py`,
`src/striatum/process_adapter.py`,
`src/striatum/process_completion.py`

RFC 0031 is sequenced after RFC 0030 lands because daemon-owned supervision
and sealed-apply authority both flow over the RPC trust boundary RFC 0030
establishes.

Implementation status: dogfood-034 added daemon DB tables for daemon-owned
supervisors and apply receipts, daemon RPC method declarations for
`supervise.*` and `apply.*`, and fail-closed apply-key/refusal helpers.
D094/RFC 0043 later moved per-repository workflow and supervisor pointer
state into daemon-owned PostgreSQL; the original repo-local pointer table is
transition history. The first production apply path remains explicitly
capability-gated and must not be described as third-party cryptographic
non-repudiation. The Go production daemon now rotates and loads the local
Ed25519 `0600` fallback signing-key file through `daemon.key.rotate` and
advertises the public key in `daemon.hello`; OS keyring custody and full
reviewed-patch mutation remain deferred.

## Problem

V1 supervision (RFC 0009) lives in repo-local SQLite. The CLI starts a
detached child process, records a `process_supervisors` row in
`.striatum/state.sqlite3`, and writes packets to its stdin FIFO. The
operator (or a recovery sweep) is responsible for keeping the supervisor
alive across packets. The runner does not own the supervised process'
lifecycle; it observes it.

That worked for V1 because supervision was advisory. Three forces now
push supervision into the daemon:

1. **RFC 0026 lane attestation** treats a supervisor row + matching pid
   start-time token as the only honest source for lane-attested bylines.
   The runner already trusts the supervisor row for byline derivation;
   making the daemon the row owner removes the operator-write attack
   surface.

2. **RFC 0027 sealed-patch provenance** is structurally accepted but
   refuses to start until a hard local authority boundary exists. Sealed
   apply authority is the natural daemon responsibility: the daemon owns
   the signing key, owns the apply gate, and owns the receipt.

3. **D082 daemon-first product** makes ad-hoc operator-started
   supervisors the wrong default. The daemon should be the parent
   process and the lifecycle owner.

Round-2 dogfood-031 review finding A1 ("no daemon RPC server, just sweep
loop + shared registry SQLite") is closed by RFC 0030. RFC 0031 builds on
that boundary to close the next overclaim risk: V1 sealed-mode promises
without daemon-owned authority.

## Goals

- Move `process_supervisors` ownership from repo-local SQLite into the
  daemon, with a migration path for in-flight runs.
- Define daemon-mediated `supervise.start`, `supervise.send`,
  `supervise.stop`, `supervise.status`, `supervise.list` RPC methods.
- Define sealed-apply authority: daemon-owned apply service, signing key
  custody, apply receipt format, refuse-on-tamper rules.
- Make `provenance_mode: sealed_patch` start-able for the first time:
  daemon-mediated apply replaces the V1 "refuses to start" stub.
- Preserve RFC 0009 invariants: stdout/stderr to DEVNULL by default,
  packet delivery via FIFO, no terminal-output state.

## Non-Goals

- Defining cross-repo workflows. That is RFC 0032.
- Defining MCP mutation defaults. That is RFC 0032.
- Replacing the lane wrappers (claude/codex/gemini supervised wrappers
  remain).
- Hosting signing keys outside the local machine. Keys live in OS
  keyring or a `0600` fallback file.
- **Resistance to a malicious local-root attacker.** Daemon-owned
  supervision, the signing key, and the apply receipts are not designed
  to defend against an operator who deliberately reads the daemon's
  files, kills the daemon, or impersonates the daemon process. See the
  threat-model section below.

## Threat Model

The provenance machinery in this RFC defends against **over-eager AI
agents** acting through the documented interfaces. It does not defend
against a malicious local operator.

In scope:

- An agent process that exits its lane scope and tries to write outside
  its declared `write_scope.allowed_paths`. The daemon refuses.
- An agent process that publishes an artifact whose claimed byline does
  not match the supervisor it ran under (RFC 0026 attestation). The
  daemon refuses; the byline downgrades to `author: operator`.
- An agent process that requests `apply.reviewed_patch` without a
  matching reviewer verdict / patch digest / base-tree hash. The daemon
  refuses and records the denial in the audit chain.
- An agent process that crashes or exits silently mid-job. RFC 0014
  process-completion guarantees apply; the daemon does not invent
  success.
- An MCP client that holds a token without the `apply` capability and
  tries to call `apply.reviewed_patch`. The daemon refuses.

Out of scope:

- An operator who runs `cat ~/.striatum/daemon/signing_key` and uses the
  key elsewhere. The signing key is local-only and operator-readable by
  design.
- An operator who runs `pg_dump` against the daemon DB and edits the
  audit chain. Audit append-only enforcement uses Postgres roles for
  protection against accidental updates through the daemon API; an
  operator with the database superuser role can bypass it.
- An operator who modifies `.striatum/state.sqlite3` directly or
  modifies the daemon's worktree directly. Repo-local provenance has
  always been "operator can edit; we record artifacts so reviewers can
  notice."
- An adversary with code execution as the daemon's OS user. That
  adversary is the daemon's trust zone; we do not claim to defend
  against them.

The boundary this RFC enforces is "the AI did what its work packet
said, and an apply receipt corresponds to a real reviewer verdict
against the real patch." Not "no human can ever forge an apply
receipt." Sealed mode is a guardrail, not a cryptographic proof.

## Proposal

### 1. Supervisor ownership migration

The V1 supervisor row schema moves into the daemon DB (RFC 0033
substrate). Repo-local SQLite keeps a thin pointer:

```text
repo-local .striatum/state.sqlite3:
  process_supervisor_pointers:
    session_id PRIMARY KEY,
    supervisor_id,
    daemon_substrate_version,
    last_known_state,
    last_observed_at
```

The pointer lets repo-local code answer "did this session have a
supervisor?" without going through the daemon. The authoritative row
lives in the daemon DB:

```text
daemon DB process_supervisors:
  supervisor_id PRIMARY KEY,
  session_id,
  run_id,
  repository_id,
  adapter,
  command_json,
  cwd,
  scratch_path,
  stdin_pipe_path,
  pid,
  pid_start_time,
  state,            -- 'starting' | 'attached' | 'detached' | 'stopped' | 'lost'
  started_at,
  attached_at,
  heartbeat_at,
  ended_at,
  stop_reason
```

The daemon is the only writer for `process_supervisors`. Repo-local code
reads pointers, never writes them; pointer rows are updated by the daemon
through repo-local API calls when state transitions land.

### 2. Supervision RPC methods

Defined per the RFC 0030 envelope. Capability binding:

```text
supervise.start    write     (per repo)
supervise.send     write     (per repo)
supervise.stop     write     (per repo)
supervise.status   read      (per repo)
supervise.list     read      (per repo)
```

`supervise.start` is the major behavior change: the daemon spawns the
lane process. The daemon process becomes the supervised child's parent;
the daemon manages the FIFO, the pid file (if any), and the stop signal
chain. CLI/MCP clients invoke the RPC but never directly Popen() the
lane command.

For backward compatibility during the V2 transition, V1's
`striatum supervise start --session-id ...` kept a one-minor-release
direct-mode path. That transition is now closed: production supervision
routes through the daemon boundary, and `--no-daemon` is retired.

### 3. Daemon process lifecycle

The daemon becomes a real long-running parent process:

- On `daemon start`, the daemon establishes the RPC server (RFC 0030),
  opens the substrate (RFC 0033), and starts the supervisor lifecycle
  loop.
- The supervisor lifecycle loop probes every `attached` supervisor's
  pid (and pid_start_time per RFC 0026) on a configurable interval.
  Lost processes transition to `lost`; the daemon records the transition
  and surfaces it via `supervise.list`.
- The daemon stops supervised children cleanly on `daemon stop`. SIGTERM
  with a 5-second grace, then SIGKILL.
- The daemon restarts must reattach in-flight supervised processes. Each
  supervisor row records `pid_start_time`; the daemon verifies the
  recorded token against `/proc/<pid>/stat` (Linux) or platform
  equivalent. Mismatch transitions to `lost`.

### 4. Sealed apply authority

`provenance_mode: sealed_patch` (RFC 0027) becomes start-able when the
daemon is the apply authority. The shape:

- **Apply service**: a daemon RPC method `apply.reviewed_patch` that
  accepts `(patch_artifact_id, target_repo_id, reviewer_verdict_id)`.
  The daemon:
  1. Loads the patch artifact bytes and verifies the artifact hash
     matches the recorded digest.
  2. Loads the reviewer verdict and verifies the verdict's
     `patch_digest_hash` matches the patch.
  3. Verifies the patch base-tree hash matches the current target
     repository state at the apply point.
  4. Applies the patch to a daemon-owned worktree (NOT the operator's
     editable checkout).
  5. Records an apply receipt: patch digest, base-tree hash, post-apply
     tree hash, reviewer verdict id, daemon signing key id, timestamp,
     daemon version, substrate version.
  6. Returns the receipt id.
- **Signing key custody**: the daemon owns an Ed25519 keypair at
  install time. Private key lives in OS keyring when available; fallback
  is a `0600` file in the daemon runtime directory with degraded-trust
  warning. The daemon refuses to start sealed-mode runs if it cannot
  load its signing key. Public key is published via `daemon describe`.
- **Refuse-on-tamper**: any digest mismatch, base-tree drift, or
  missing reviewer verdict causes refusal with documented exit codes.
  The audit chain records the denial.
- **Receipt format**: append-only daemon DB row plus a corresponding
  Markdown artifact published into the run's evidence path with
  daemon-attested byline `author: striatumd-<instance-id>`.

`apply.reviewed_patch` has capability `apply` (new vocabulary entry in
RFC 0032 capability set; RFC 0031 introduces it here so sealed-mode is
gated explicitly). Tokens default to no `apply` capability; operators
who want daemon-applied patches must explicitly grant.

### 5. Sealed-mode workflow start-up gating

`run start` for a workflow declaring `provenance_mode: sealed_patch`
now requires:

- a daemon connection (no direct-mode fallback);
- a daemon with sealed-mode support advertised in
  `daemon.welcome.data.sealed_apply.supported`;
- a signing key loadable by the daemon at start time;
- at least one capability token with the `apply` capability bound to
  the run's repository_id.

If any condition fails, `run start` refuses with a documented exit code
and the workflow remains in `prepared`.

### 6. Apply gate in the workflow graph

Workflows may declare `apply_gate: true` on a build/handoff job. With
the gate set:

- the job's output must be a `patch_summary` artifact (kind defined by
  RFC 0027) with a digest field;
- the gate refuses to mark the job complete unless a downstream
  reviewer verdict references the patch digest;
- the gate refuses to apply unless the daemon's `apply.reviewed_patch`
  call succeeds.

The gate is opt-in; V1-shaped workflows without `apply_gate` continue
unchanged.

### 7. Compatibility with V1

- V1 dogfood workflows continue to register supervisors via
  `supervise start`; the daemon now owns them, but the CLI surface
  is preserved.
- V1 `process_supervisor` rows in `.striatum/state.sqlite3` are
  migrated to daemon DB at first daemon start after upgrade. The
  pointer table replaces the V1 row's daemon-side fields; repo-local
  rows for terminated supervisors are kept for run-summary
  reproduction.
- V1 supervised wrappers (`.striatum/bin/claude-supervised-wrapper.sh`,
  `gemini`, `codex`) keep working; the daemon is now their parent.
- V1 `provenance_mode: sealed_patch` workflows that refused to start
  in V1 now start under RFC 0031 daemon-apply authority. Workflows
  that explicitly want the V1 refuse-to-start behavior can declare
  `sealed_patch_provider: refuse` (documented as a debugging aid).

### 8. Daemon crash and restart

When the daemon crashes:

- Supervised lane processes survive because they are detached
  (`start_new_session=True`).
- On daemon restart, the daemon reattaches supervisors using
  `pid_start_time` verification.
- Lane processes whose `pid_start_time` mismatches are marked `lost`
  and their leases expire via the normal lease lifecycle.
- The substrate (RFC 0033) is responsible for transactional
  consistency of supervisor state across the crash boundary.

### 9. Test infrastructure

- A daemon-with-supervisors harness spawns a daemon and a fake lane
  command (a small shell script that reads packets and writes a log)
  so tests don't depend on agent CLIs.
- Restart tests kill the daemon and re-spawn it, asserting that
  in-flight supervisors are correctly reattached or marked lost.
- Sealed-apply tests exercise base-tree drift, digest mismatch,
  missing-reviewer-verdict, and the happy path with byte-equivalent
  receipts.
- Apply-gate workflow tests run a small workflow with a
  `patch_summary` artifact and a reviewer verdict referencing the
  patch digest.

### 10. Provenance and trust implications

- Daemon process compromise now affects supervised processes and
  sealed-apply authority. Mitigations: socket permissions, signing
  key custody, audit chain, version-handshake refusal. None of these
  defend against the operator themselves; see Threat Model above.
- The signing key is local-only and operator-readable by design (see
  Non-Goals and Threat Model). Sealed mode is an AI-guardrail, not a
  cryptographic proof of authorship or a defense against a malicious
  operator.
- RFC 0026 attestation rules still apply: bylines downgrade to
  `author: operator` when supervisors are unattested or missing.
- Apply receipts are local evidence that the AI did what its work
  packet claimed, not external proof that the operator can present to
  a third party.

## Compatibility and Migration

- RFC 0033 (substrate) lands first.
- RFC 0030 (RPC) lands next; daemon-mediated routing becomes default.
- RFC 0031 supervisor migration runs on first daemon start after
  upgrade; existing dogfood workflows continue to work.
- Sealed mode becomes start-able for the first time; documentation
  updates `docs/SPEC.md` and the RFC 0027 status accordingly.

## Downsides and risks

- Daemon is now in the critical path for supervised lanes. A daemon
  bug can affect every supervised run on the machine.
- Sealed-apply authority can be misread as a cryptographic proof.
  Documentation must resist overclaim. The Threat Model section is the
  authoritative scope statement; SPEC.md and README.md must reflect it
  exactly.
- Crash-during-apply requires careful idempotency: the apply receipt
  is recorded before the worktree write so a crash mid-apply produces
  a missing-receipt + clean worktree, not a half-applied patch.
- The operator-readable signing key is a deliberate non-goal (see
  Threat Model). If the product later needs an external-proof story,
  that is a separate RFC introducing a different key custody model;
  this RFC's "AI guardrail" framing does not need to bend.

## Benefits

- One owner for supervised processes. The runner no longer pretends
  the supervisor row is the truth while the OS is the actual owner.
- Sealed-mode workflows can finally start.
- Apply receipts give downstream review and recovery a concrete
  artifact to inspect.
- RFC 0026 attestation gains a stronger lower bound (daemon-owned
  supervisor, not operator-written row).

## Acceptance Criteria

- `supervise.start` over RPC spawns a lane process as the daemon's
  child; the daemon's `process_supervisors` row records pid and
  `pid_start_time` matching `/proc/<pid>/stat`.
- `supervise.send` writes a packet to the FIFO and records a
  `supervisor.packet_delivered` event on the substrate.
- Daemon restart with one alive supervised child reattaches the
  supervisor; a test asserts `pid_start_time` verification.
- `run start` against a `provenance_mode: sealed_patch` workflow
  succeeds when the daemon has a signing key and an `apply` token is
  granted; refuses otherwise with documented exit codes.
- `apply.reviewed_patch` happy path records an apply receipt and a
  Markdown evidence artifact bylined `author:
  striatumd-<instance-id>`.
- Base-tree drift, digest mismatch, and missing-verdict scenarios
  each refuse with the documented denial vocabulary.
- Documentation in `docs/SPEC.md`, `docs/MCP.md`, the RFC 0027 status
  block, `docs/CLI_REFERENCE.md`, and `docs/HOW_TO_HUMAN.md` is
  updated to name daemon-mediated supervision, sealed-apply, and the
  apply-gate workflow field.

## Open Questions

- Should the apply gate run against a daemon-owned worktree (as
  proposed) or against the operator's editable checkout? Recommendation:
  daemon-owned worktree, so the operator's worktree is never partially
  modified by daemon authority. Reviewers should push back.
- How is the daemon's signing key rotated? Recommendation: explicit
  `daemon.key.rotate` admin RPC; old key revocation is recorded in
  the audit chain. Receipts signed by the old key are still verifiable.
- Should sealed-mode runs require `require_daemon: true` implicitly, or
  is it an error to combine `require_daemon: false` with
  `provenance_mode: sealed_patch`? Recommendation: implicit; the
  workflow validator rewrites with a documented warning.
- What happens when an `apply` token is leaked? Recommendation:
  `daemon.token.revoke` plus operator-recorded decision. Per the
  Threat Model section, token theft by an attacker with code execution
  as the daemon's OS user is out of scope; this open question is about
  the more mundane "operator accidentally pasted the token into a
  chat" case.
- How does the daemon-applied worktree integrate with the operator's
  git history? Recommendation: the daemon writes to a private worktree
  and emits a patch + receipt; the operator merges via their normal
  git workflow. Daemon does not push, commit-amend, or rewrite
  history.

## Domain Modeling

Terms to add to `docs/UBIQUITOUS_LANGUAGE.md` after acceptance:

- **Daemon-owned supervisor** — a supervised lane process spawned and
  owned by the daemon, with its `process_supervisors` row in the
  daemon DB rather than in repo-local SQLite.
- **Supervisor pointer** — the repo-local `process_supervisor_pointers`
  row that lets repo-local code identify a session's supervisor
  without contacting the daemon.
- **Sealed-apply authority** — the daemon's privileged role in
  applying reviewer-approved patches against a daemon-owned worktree,
  with signing key custody and receipt issuance.
- **Apply receipt** — the daemon-recorded evidence of a successful
  sealed apply: patch digest, base-tree hash, post-apply tree hash,
  reviewer verdict id, signing key id, timestamp.
- **Apply gate** — the workflow-level opt-in field that requires a
  daemon-mediated apply receipt before a build job completes.
- **Daemon signing key** — the Ed25519 keypair the daemon uses to sign
  apply receipts. Local-only, stored in OS keyring or `0600`
  fallback file.
