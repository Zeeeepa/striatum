---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0040", "mcp-harness", "supervised-watcher"]
---

author: reviewer-claude-opus-001

# RFC 0040 Design Synthesis — Threat-Model Review

Scope per RFC 0031 §Threat Model: over-eager AI agents through documented
interfaces + operator-mistake footguns. Malicious-local-root is OUT OF
SCOPE. Subject: `docs/dogfood/040/DESIGN_SYNTHESIS.md`.

## Trust Boundaries Introduced

The synthesis adds or perturbs five trust boundaries. Each is enumerated
with its mitigation status.

1. **`surgical_recovery` capability (new vocabulary entry).** Privileged
   bypass of the repo-write stale-lease refusal that exists precisely to
   prevent recovery of a still-running repo-write job. Mitigation: added
   to the closed RFC 0030 vocabulary, gated separately from `recovery`,
   audit row appended with operator reason, validation refuses live
   supervisors. Acknowledged with finding F1.

2. **Composite tools as atomic state mutators.** `publish_on_behalf` and
   `surgical_recovery` collapse 3-4 RPC state transitions into one
   audited row. The risk is that the audit chain loses the per-step
   shape that reviewers depend on for forensic reconstruction.
   Mitigation: synthesis specifies `composition_steps` metadata
   capturing each step + ids, single row preserves "operator's mental
   model" without losing trace. Acknowledged.

3. **Supervised-progress watcher as a daemon-internal heartbeat source.**
   The watcher calls `heartbeat` on the daemon's own behalf based on
   filesystem signals from a supervised process. The risk is that an
   over-eager supervised AI can extend its own lease indefinitely by
   touching its log. Mitigation: 600s idle threshold caps the no-progress
   window. Watcher is daemon-internal, not remotely triggerable. Reads
   file metadata only, never contents. Acknowledged with finding F2.

4. **`workflow upgrade` verb mutating committed workflow.json.** A CLI
   command that rewrites declared harness-profile instructions, i.e.,
   the operator's accepted boundary on supervised model behavior.
   Mitigation: refuse-on-conflict default, dry-run default, refuse on
   running workflow, scoped to `harness_profiles.*.native_delegation.
   instruction`. Acknowledged with finding F3.

5. **MCP exposure of `claim_next` / `ack` / `verdict` / `complete` as
   structured tools.** These were previously CLI-only. The risk is
   that operator-AI sessions now invoke lifecycle transitions via the
   chat-tool surface, broadening the call sites for the same authority.
   Mitigation: capability gating reuses the same per-method requirements
   from RFC 0030; `tools/list` filters by token; audit row appended per
   call (allowed or denied) with `transport: "mcp"`. Acknowledged.

## Specific In-Scope Checks

### surgical_recovery is admin-only + short-lived token

The synthesis states: "admin-only in product posture and should be
issued as a short-lived token; 15 minutes is the recommended maximum."
Denial vocabulary `capability_missing` and
`surgical_recovery_validation_failed` are documented.

This satisfies the boundary, but the "should be" framing is advisory
rather than enforced. See finding F1.

### Composite tools record operator-supplied reason

Both `publish_on_behalf` and `surgical_recovery` require non-empty
capped `reason` in input schema; reason is stored as `operator_reason`
in the single audit row alongside `composition_steps`. Verified in the
synthesis schemas. Satisfied.

### Supervised-progress watcher concurrency

- Per-supervisor task isolation: "one daemon-owned watcher task per
  attached supervisor." Satisfied.
- Watcher x `surgical_recovery` race: "resolved by using the same
  transactional job/lease checks; if the job is no longer in the
  heartbeatable state, the watcher skips." Adequate at the design
  level; the implementer must lock at the lease/job row rather than
  rely on read-then-write. See finding F4.
- Supervisor crash detection: synthesis defers to the existing daemon
  supervisor lifecycle loop ("cancel on `stopped`, `detached`, or
  `lost`"). The watcher does not itself probe pid liveness. The
  reviewer role doc explicitly asked for `os.kill(pid, 0)` style
  probing; the synthesis assumes the supervisor lifecycle loop will
  surface `lost` quickly enough. See finding F5.

### workflow upgrade refuses by default on conflict

Verified: dry-run default, refuse-on-conflict by default, refuse on
running workflow, only edits `harness_profiles.*.native_delegation.
instruction`. `--force` overrides. See finding F3.

### Adversarial cases from gemini's design in test strategy

Gemini's adversarial list (per synthesis §3-way build review):
- live-supervisor surgical recovery ✓ ("attached/live supervisor
  refusal")
- concurrent composite calls ✓ ("concurrent second call refusal")
- missing log files ✓ ("missing log directories")
- active workflow upgrade ✓ ("running-workflow refusal")
- token leakage across composite boundaries — NOT explicitly listed.
  See finding F6.

### 3-way build review write scope isolation

Verified against `docs/dogfood/040/workflow.json` lines 374, 397, 420:
each build reviewer writes to its own subdirectory
(`review/build/codex/`, `.../claude/`, `.../gemini/`). No shared write
scope. Collusion via filesystem is prevented. Satisfied.

### Implementer split disjoint

Verified against `workflow.json` lines 287-301 (systems-codex) and
321-350 (ergonomics-claude). File-level disjoint:
- `src/striatum/cli/recovery.py` is codex-only;
  `src/striatum/cli/parser.py`, `dispatch.py`, `workflow.py` are
  claude-only. Shared `src/striatum/cli/` namespace but no file
  overlap.
- `src/striatum/web/chat_tools.py` is claude-only (registry/tool
  entries); `src/striatum/daemon_rpc/` is codex-only (the underlying
  RPC route). The cross-cutting risk is conceptual integration
  (claude declares a tool name that codex's registry must accept),
  not file-write overlap. Satisfied as a write-scope concern.

### Capability scope: daemon-global vs repo-scoped

The synthesis states: "The method is repository-scoped to one job/repo,
but the capability is privileged and should be granted only deliberately
by an admin operator." This is ambiguous about whether the token
binding itself is per-repo or daemon-global. See finding F7.

### Denial vocabulary documented

`no_active_lease`, `lease_busy`, `surgical_recovery_validation_failed`,
`capability_missing` all explicitly listed in the synthesis composite
sections. Satisfied.

### Watcher not remotely triggerable

Watcher is daemon-internal background task started/cancelled by the
supervisor lifecycle. No RPC method exposes it. No MCP tool triggers
it. Synthesis confirms "daemon-owned" and "reads file metadata only."
Satisfied at the design level; see F8 for the path-traversal sub-case.

## Findings (non-blocking)

### F1 — `surgical_recovery` admin-only is advisory, not enforced

Severity: medium. Non-blocking.

The synthesis says "admin-only in product posture and should be issued
as a short-lived token; 15 minutes is the recommended maximum." The
token issuance code path (`daemon.token.issue` or equivalent) is not
specified to refuse `surgical_recovery` on non-admin token requests, or
to cap the TTL at 15 minutes when the capability is requested. Without
an enforced cap, the recommendation drifts in practice (the operator
issues themselves a long-lived `surgical_recovery` token during one
dogfood and never revokes it).

**Recommendation for implementers**: in `daemon_rpc/capability.py`,
the issuance path for `surgical_recovery` should:
- Require an admin-tagged operator role (introduce/reuse), or default-
  refuse and require an explicit override flag the operator records.
- Cap the issued TTL at 15 minutes (configurable) and refuse longer.
- Emit an audit row on issuance with the operator's reason.

Non-blocking because the operator-mistake threat model assumes a
careful admin, but the enforced cap is a low-cost defense-in-depth.

### F2 — Heartbeat-on-mtime is bypassable by `touch`

Severity: medium. Non-blocking; explicitly acknowledged in RFC 0040
open questions.

An over-eager supervised AI can extend its own lease indefinitely by
`touch <log_path>` in a tight loop. The 600s idle threshold only
catches the no-progress case, not the runaway-progress case.

The RFC 0040 open question acknowledges this trade-off and proposes
a sidecar-signal upgrade for V1.5 if false-positive rate is too high.
The threat-model verdict here is "accept with documentation": the V1
mtime approach is a documented soft signal, and a maximum total lease
extension cap (e.g., 4× the original lease duration) would harden it.

**Recommendation for implementers**: log a metadata event on every
watcher-triggered heartbeat (which the synthesis already implies via
audit rows) so post-hoc inspection can detect runaway heartbeat loops.
Consider an absolute "total-watcher-extensions-since-claim" cap as a
defense-in-depth that does not require sidecar signals.

### F3 — `workflow upgrade --force` does not preserve the prior instruction

Severity: medium. Non-blocking.

The synthesis says `--force` "replaces the instruction with the
canonical current text for the matching `tool_family`" but does not
say whether the prior instruction text is logged anywhere before
replacement. If an operator deliberately customized the harness-profile
fragment (per RFC 0040's open question on operator override), `--force`
silently destroys that customization with no rollback artifact.

**Recommendation for implementers**: when `--force` actually replaces
a customized instruction, the upgrader should:
- Write a `.upgrade-backup` file alongside (or in a deterministic
  location) containing the prior instruction text, the timestamp, and
  the operator's `--force` invocation.
- Emit a stderr warning that names the file.
- Optionally land the backup as a CHANGELOG-style entry in the workflow
  directory rather than a sibling file.

This addresses an operator-mistake footgun that the threat model
explicitly covers.

### F4 — Composite + watcher transactional locking is not specified

Severity: low. Non-blocking.

The synthesis says race resolution uses "the same transactional job/
lease checks." For correctness this needs to be row-level locking
(`SELECT … FOR UPDATE` on lease/job rows, or equivalent at the daemon
storage layer) rather than read-then-write, otherwise a watcher heart-
beat and a concurrent `surgical_recovery` validation can both observe
"lease still active" and race on the update.

**Recommendation for implementers**: the watcher's heartbeat path and
the composite tools' validation path must take a transaction-scoped
lock on the lease row before reading state. Tests should cover the
race directly (two concurrent operations, one must lose with a
documented denial code rather than corrupt state).

### F5 — Watcher does not probe pid liveness directly

Severity: low. Non-blocking.

The reviewer role doc asks for `os.kill(pid, 0)` style probing in the
watcher. The synthesis defers to the supervisor lifecycle loop's
periodic pid-and-pid_start_time probe to surface `lost` state. The
watcher then cancels on `lost`. This is functionally correct but
asymmetric: a dead supervised process whose mtime is recent (stale
buffered log flush, or filesystem caching) could trigger a heartbeat
in the brief window before the supervisor loop catches up.

**Recommendation for implementers**: inside the watcher's heartbeat
path, after observing mtime growth and before issuing the internal
heartbeat, do a cheap `os.kill(supervisor.pid, 0)` (or platform
equivalent) and skip the heartbeat if the process is dead, then cancel
the watcher's own task. This avoids depending on the supervisor loop's
timing.

### F6 — Token leakage across composite boundaries not in test strategy

Severity: low. Non-blocking.

Gemini's adversarial case "token leakage across composite boundaries"
is not explicitly enumerated in the synthesis's Test Strategy section.
The relevant case: a token with `write` capability calls
`publish_on_behalf` supplying `verdict` (which should require `review`);
the composite's per-step capability check must fail rather than allow
the verdict step under `write` because the composite operates as one
unit.

**Recommendation for implementers**: add to systems tests a case that:
- Authenticates with a `write`-only token (no `review`).
- Calls `publish_on_behalf` with a `verdict` argument.
- Asserts the call fails with `capability_missing` before any state
  mutation, audit row records the denial, no partial state lands.

This closes the gap the gemini-lane review would otherwise discover
during build review.

### F7 — Capability binding scope (daemon-global vs repo-scoped) is ambiguous

Severity: medium. Non-blocking.

The synthesis says the `surgical_recovery` method is repo-scoped to one
job/repo, but the capability "should be granted only deliberately by an
admin operator." It does not explicitly state whether the token's
capability binding is per-repo (like `write`, `claim`, `apply`) or
daemon-global (operator-administrative). The distinction matters for
least-privilege: a per-repo `surgical_recovery` token leaks only
recovery authority for that repo, while a daemon-global token enables
recovery against any repo on the host.

**Recommendation for implementers**: declare `surgical_recovery` token
bindings as **per-repo** by default (matching `write`/`claim` shape).
Operators wanting a multi-repo recovery token issue multiple tokens or
a token with explicit `repos: [...]` enumeration. Daemon-global is not
necessary for the dogfood-038 friction this RFC removes (it was always
one specific repo, one specific job). Document the binding shape in
`docs/MCP.md`.

### F8 — Watcher path-traversal via symlink in supervisor scratch

Severity: low. Non-blocking.

The watcher resolves `log_path` from the supervisor's `scratch_path`.
If a supervised AI writes a symlink into its scratch directory
(`<scratch>/codex-logs/packet-NNNN.log -> /tmp/external-file`) and
then touches that external file, the watcher's `os.stat` follows the
symlink and triggers a heartbeat based on activity the watcher did not
intend to monitor. The reviewer role doc explicitly called out "no
path traversal from log_path."

**Recommendation for implementers**:
- Use `os.lstat` (not `os.stat`) so symlink mtime, not target mtime,
  is what the watcher observes.
- Refuse to follow symlinks at all in the watcher. If the discovered
  `log_path` is a symlink, log a warning and treat as missing.
- Validate the resolved path remains inside the supervisor's declared
  `scratch_path` via `os.path.commonpath` or `pathlib.Path.resolve()`
  + prefix check.

## Verdict Rationale

The plan defensibly handles each trust boundary introduced. The composite
audit shape, capability gating, watcher isolation, write-scope disjoint-
ness across reviewers and implementers, and explicit denial vocabulary
are all in place. The eight findings above are concrete refinements,
not boundary violations: F1, F3, F7 harden capability/upgrade semantics
against operator mistakes; F2, F5, F8 close mtime/symlink/pid-probe
gaps in the watcher; F4 specifies the transactional locking pattern
that the synthesis already implies; F6 adds a missing test that the
build-phase gemini reviewer would otherwise discover.

Each finding is explicitly non-blocking. Implementers should treat F1,
F3, F7, F8 as priority hardenings to land in V1 alongside the base
implementation; F2, F4, F5, F6 can land as part of the named test
suites without delaying the V1 slice.

The verdict is `accept_with_findings`. The synthesis is ready for the
implement phase. The findings here should be reflected in build-phase
work and re-checked by the build-phase threat-model reviewers.
