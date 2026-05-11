---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["devils_advocate", "rfc-0028", "daemon", "design", "round-3"]
---

# Devil's Advocate Review (Round 3): RFC 0028 V1 Daemon Implementation Plan

author: reviewer-claude-opus-004

date: 2026-05-11
status: completed
target: docs/dogfood/031/DESIGN_SYNTHESIS.md (revision 3)
posture: devils_advocate

## Posture

Fresh devil's-advocate pass on revision 3. Verdict acceptance means the
claims survived the strongest counterarguments I can mount. Acceptance
with findings means the findings are non-blocking — the implementer can
correctly resolve them from the synthesis text plus standard discipline,
without another design revision cycle. The findings below are explicitly
non-blocking.

## Summary

Revision 3 closes the round-2 blockers (B1, B2, B3) and the eight
non-blocking findings (NB1–NB8) with concrete edits, not hand-waves.
The hash-chain rule is now end-to-end across rotation, with `daemon
doctor` walking every retained manifest. Repo removal is idempotent,
cascade-revokes capabilities, audits each revocation, preserves audit
rows, and refuses `repository_id` reuse. `striatum://daemon/audit` is
removed from V1 MCP entirely. Capability vocabulary is now `read` and
`admin` only — no reserved strings. `repo rebind` is gone. The daemon
byline is fixed. Cross-platform scope is named (Linux + macOS). Crash
mid-sweep semantics are stated. Health-probe audit is per-request.
Forced-daemon exit-code classes are tabulated.

Every load-bearing devil's-advocate attack from round 2 has a concrete
answer in revision 3. The remaining attacks I can mount are
implementer-guardrail concerns, not design defects. Verdict:
`accept_with_findings` with eight small, explicitly non-blocking
guardrails the implementer should carry into the build handoff. None
of them require another design revision cycle.

## 1. Round-2 Blocker Closures (confirmed)

| Round-2 blocker | Revision 3 closure | Assessment |
|---|---|---|
| B1: Hash-chain continuity across rotation unspecified. | "V1 uses one continuous hash chain across segments. The first row in segment N+1 has `previous_hash` equal to segment N's `last_hash`; segment manifest rows are append-only through daemon APIs; `daemon doctor` walks every retained manifest and retained row range end-to-end." Test Matrix adds "Audit chain across rotation: rotate segments, delete or forge data in a closed segment, doctor surfaces chain break by comparing `audit_segments.last_hash` to next segment's first `previous_hash`." Retention tombstones explicitly reduce row-level verification but preserve cross-segment continuity evidence. | Closed. The strongest "audit detects tamper" claim now holds across rotation. |
| B2: `repo remove` cascade semantics unspecified. | Idempotent; revokes every live repo-scoped capability; audits each revocation with `authorization_result="revoked"`; preserves audit rows; cascades `scheduler_cursors` to terminal removed state; `repository_id` never reused; doctor reports removed ids referenced by retained audit. Multiple Test Matrix entries. | Closed. Default-deny is now whole across the remove/re-add boundary. |
| B3: `striatum://daemon/audit` exposed via MCP. | "`striatum://daemon/audit` is removed from V1 MCP. Audit is available only through daemon CLI/socket or loopback HTTP admin endpoints. MCP remains resources-only and excludes cross-repository audit metadata." | Closed. The MCP narrowing claim is now coherent. |

## 2. Round-2 Non-Blocking Closures (confirmed)

| Round-2 NB | Revision 3 closure | Assessment |
|---|---|---|
| NB1: Daemon byline. | Fixed format `author: striatumd-<instance-id>`, "distinct from operator, role, lane, and model bylines." | Closed at the design level. (See NB-R3-2 below for one residual.) |
| NB2: Cross-platform packaging gap. | Linux + macOS only. Windows deferred. Linux uses XDG paths; macOS uses Keychain + `~/Library/Application Support/striatum/` + `~/Library/Caches/striatum/runtime/` with `0700`/`0600`. macOS runtime-file fallback warns degraded-trust because it survives reboot. CI must cover each claimed branch. | Closed. (See NB-R3-1 below for one residual.) |
| NB3: Reserved-but-not-exposed capability strings. | Removed. V1 vocabulary is only `read` and `admin`. | Closed. |
| NB4: `repo rebind`. | Removed. Operators must `repo remove` then `repo add`. | Closed. |
| NB5: `recovery` capability. | Removed. Sweep uses daemon-internal authority. V1 has no client-facing recovery mutation endpoints. | Closed. |
| NB6: Crash during sweep. | Scheduler cursors advance only after sweep completion *and* repo-local events/audit durably written. Daemon killed mid-sweep retries that run on restart. Test Matrix has "Crash during sweep: kill daemon mid-sweep; on restart the interrupted run is the next eligible sweep target, not skipped, and `sweep_degraded` is not falsely set." A crash alone must not mark `sweep_degraded`. | Closed. |
| NB7: Health-probe audit summarization. | Per-request rows with `command="health"` and `repository_id=null`. No summarization in V1. | Closed at the design level. (See NB-R3-3 below for one residual.) |
| NB8: Forced-daemon exit-code classes. | Table with classes 9/10/11/12/13. | Closed. |

## 3. Round-3 Findings (non-blocking, implementer guardrails)

Each of these is an implementer-resolvable seam in revision 3. They are
**not** blockers. They should be carried into the V1 build handoff
either as resolved-in-build decisions or as named follow-ups in
`docs/TODO.md` with explicit RFC-section anchors. They are not design
defects requiring another revision cycle.

### NB-R3-1. Keyring-availability detection rule is left to the implementer

§"Token lifecycle" says "OS keyring is preferred. … If keyring storage
is unavailable, the fallback is an owner-only runtime file under the
platform runtime directory with mode `0600`, a short expiry
recommendation, and a degraded-trust warning."

The rule does not say *how* the daemon (or CLI client) decides keyring
storage is "unavailable." On Linux the `keyring` library has multiple
backends (Secret Service, kwallet, gnome-keyring, file, fail). On
macOS, Keychain is normally present but sandboxed contexts can still
fail at write time. The risk is a silent fallback when the keyring is
present-but-write-failing: degraded trust without operator awareness.

**Non-blocking recommendation.** The implementer should:
- pick a single keyring backend per platform in V1 (Secret Service on
  Linux, Keychain Services on macOS), refuse other backends, and treat
  refusal as "unavailable";
- emit the degraded-trust warning at *every* runtime-file fallback
  read/write, not only at mint time;
- include the keyring backend identity in `daemon doctor` output so
  operators can confirm which storage is in effect.

### NB-R3-2. Instance-id lifetime semantics are unspecified

§"Resident Recovery Scheduler" specifies the byline as `author:
striatumd-<instance-id>` but does not say what an instance-id is or
when it changes. Plausible readings:

1. **Per-daemon-process** (rotates on every daemon restart). Then the
   repo-local timeline shows different daemon identities for each
   process lifetime and operators cannot ask "what did the daemon
   produce as a whole." This is the easiest implementation and is
   probably acceptable, but the operator UX needs to know the rotation
   rule.
2. **Per-registry-lifetime** (lives in `daemon_meta`, regenerated only
   on registry recreation). Then bylines correlate across restarts of
   the same install, which is the more useful operator timeline.
3. **Per-host** (machine-derived). Then bylines correlate across
   reinstalls; probably *too* persistent for V1.

The implementer can reasonably pick any of the three from the current
text.

**Non-blocking recommendation.** Pick option 2 (persisted in
`daemon_meta`, regenerated only on registry recreation) so the
repo-local timeline remains queryable across normal daemon restarts.
State the rule in the build handoff and add one Test Matrix entry
asserting that two daemon restarts on the same registry produce the
same `striatumd-<instance-id>` byline on emitted events.

### NB-R3-3. Health-flood audit growth is not bounded

NB7 is closed at the "every request recorded" criterion: each health
probe writes a row. But that criterion now interacts with the
hostile-local-client model. A peer process under the same UID does not
need a token to call `/health`; the audit writer accepts the row and
appends. Retention defaults to 90 days *queryable*, time-based, not
row-count-based. A sustained probe flood can:

- bloat the active audit segment until rotation;
- accelerate segment rotation faster than the manifest sidecar export
  can keep up;
- raise total disk pressure to the point that legitimate audit writes
  may compete with retention sweeps.

The hostile-local-client model accepts this in principle (the synthesis
already says V1 does not defend against an adversarial local UID with
filesystem write access), but the design body does not call this
specific volumetric attack out as accepted. The Test Matrix entry
"Hostile local clients" covers oversized payloads, unknown endpoints,
malformed token ids, and replayed request ids — not volumetric health
flood.

**Non-blocking recommendation.** Pick one:
- (a) State explicitly that V1 accepts volumetric health-probe flood
  as in-scope for the hostile-local-client model and depend on OS-level
  rate limiting or operator monitoring to detect it; or
- (b) Add a daemon-side per-source-PID health-probe rate limit
  (denied health probes still write rows with
  `authorization_result="denied"` and a `health_rate_limited` denial
  reason, capping per-second growth).

Either is acceptable. The current text leaves it implicit, which means
the implementer will probably ship (a) without naming it.

### NB-R3-4. Scope-filtered MCP enumeration is not explicitly tested

§"MCP Surface And Capability Defaults" lists `striatum://daemon/repos`
and `striatum://daemon/dashboard` as daemon-level resources, and says
"Read tokens are scoped to repository ids and filter resource lists
accordingly." The Test Matrix has "Capability scoping: Read token
scoped to repo A cannot read repo B" — which covers per-repo
resource reads but does not explicitly cover the *enumeration*
resources `striatum://daemon/repos` and `striatum://daemon/dashboard`.

Risk: an implementer can read "filter resource lists accordingly" as
"filter the repo list" but ship `striatum://daemon/dashboard` returning
all-repo dashboard data to a repo-scoped read token, on the reasoning
that "dashboard is daemon-level, not repo-level." That is exactly the
cross-repo reconnaissance channel the B3 fix was supposed to close.

**Non-blocking recommendation.** Add one Test Matrix entry: "Scoped
read token from repo A receives only repo A in
`striatum://daemon/repos` and sees only repo A's runs/blockers/jobs in
`striatum://daemon/dashboard`. Admin tokens see all." The rule itself
is stated; the test should be explicit.

### NB-R3-5. Direct-CLI repo-local migration trigger is unstated

§"Schema And Migration" says "Keep repo-local schema changes minimal:
the only acceptable V1 repo-local schema addition is the repository
identity needed to prevent path-based capability transfer. If the
existing schema already has a suitable stable identity, reuse it."

If existing schema does not have a suitable identity, V1 adds one
repo-local column. §"Existing-state registration plan" step 3 says
`repo add` "By default it connects through the existing repo-local
path and applies pending migrations. With `--no-migrate`, it first
checks whether migrations would be needed and refuses rather than
mutating repo-local state." — this covers the `repo add` path.

It does not cover what direct-CLI commands do for users who never run
`repo add` but who upgrade Striatum and continue using their existing
`.striatum/state.sqlite3`. If the new identity column lands as a
schema migration, the next direct-CLI command will trigger the
migration silently, which is a behavioral change for users who never
adopted daemon mode and never intend to.

**Non-blocking recommendation.** Pick one in the build handoff:
- (a) Reuse existing schema identity (if a suitable column exists,
  document which one and state that no migration is needed); or
- (b) Add the migration but document that it is *only* applied by
  `repo add`, not by direct-CLI commands, and have direct-CLI
  commands run cleanly against the pre-migration schema by treating
  the identity column as opt-in.

Option (a) is preferable if a suitable column exists. The current
text does not name one, so the implementer needs to choose.

### NB-R3-6. Request-id semantics are undefined outside the audit row

§"Audit Log Shape" includes `request_id` "when supplied" as an audit
field. The Test Matrix entry "Hostile local clients" includes
"replayed request ids are denied without tracebacks." Nothing in the
design body explains where `request_id` comes from, whether the
daemon enforces request-id uniqueness as a replay defense, what
window of uniqueness applies, or how the daemon detects a replay.

Risk: implementer reads "denied without tracebacks" as "if the client
supplies a duplicate request_id within the same session, return a
denial" — and ships no global replay protection, which is a different
defense.

**Non-blocking recommendation.** Either (a) state that `request_id`
is per-request and is informational only (no replay defense — the
"replayed request id" test asserts that the daemon does not crash on
duplicates), or (b) define a request-id replay window and the
audit/deny path. Either is acceptable. The synthesis can defer the
defense entirely; what should not be left implicit is whether
"replay defense" is V1 scope.

### NB-R3-7. Service-installer wording branching is not test-gated

§"Resident Recovery Scheduler" and §"Accepted Implementation Scope"
say service-manager support is optional for this slice. Docs/UI must
say "foreground daemon sweep" if `daemon install` does not ship for
the relevant platform, and may say "installed resident recovery" only
for platforms where `daemon install` *did* ship. This is a
documentation conditional that the implementer must apply per
platform.

Risk: the implementer ships `daemon install` for Linux user systemd
only and then product docs say "installed resident recovery" globally,
including for macOS where no `daemon install` shipped. That overclaim
is exactly what NB7's predecessor (round-1 F1 against "resident
recovery") was trying to close.

**Non-blocking recommendation.** Add one Test Matrix entry asserting
that `striatum daemon --help` and any product documentation that
appears in the V1 docs slice use "foreground daemon sweep" for
platforms where `daemon install` did not ship, and use the
installed-residency language only for platforms where it did. This
keeps the wording honest under partial implementation.

### NB-R3-8. Stale daemon socket cleanup after crash is implicit

§"Implementation Landing Order" item 3 lists "lifecycle lock, version
handshake, health endpoint" but the synthesis does not say how the
daemon handles a stale Unix socket left by a previous unclean exit. On
Linux, a leftover socket at the canonical path will cause `bind` to
fail with `EADDRINUSE` unless the new daemon unlinks it first; if the
new daemon unlinks unconditionally, it can race a still-live instance.

This is normal daemon hygiene, but it is not explicitly stated and the
Test Matrix has no "second daemon refuses to start while first is
healthy" entry.

**Non-blocking recommendation.** Add one rule: the daemon takes a
lifecycle lock (e.g., `flock`/`O_EXCL` lockfile alongside the socket)
*before* unlinking a stale socket. If the lock is held by a live
process, the second daemon refuses to start with exit-code class 10.
Add one Test Matrix entry: "Second daemon start refuses with class 10
while first instance is healthy; cleanly recovers from socket left by
killed previous daemon."

## 4. Scope-Creep Audit

Per the prompt: scope creep beyond RFC 0028 §Acceptance Criteria and
§8 phased steps 1–6. Revision 3 is not creeping. The vocabulary creep
flagged in round 2 NB3/NB5 is now gone. Capability vocabulary is
`read`/`admin` only. `recovery` was removed. `daemon_supervisors` was
removed in round 1. Reserved strings are gone.

`audit_segments` rotation is on the more-than-strict-minimum side, but
round-1 F3 demanded it and revision 3 honors that demand. I am not
asking to walk it back.

The implementation landing order (§"Implementation Landing Order") is
sequenced cleanly: registry helpers, then `repo add/list/remove`, then
foreground daemon + audit, then tokens, then daemon read mode, then
MCP resources, then sweep, then docs/tests. None of those exceed the
acceptance criteria.

## 5. Tenancy Audit

- **Repository tenant:** `repositories(repo_identity, repo_root, ...)`
  with uniqueness on `repo_identity` and on active canonical `repo_root`
  prevents path-based identity transfer. Removal idempotency and
  `repository_id` non-reuse prevent stale capability laundering.
  Modeled. Good.
- **Client tenant:** `clients` and `client_capabilities` with token id,
  hash, salt, expiry, revocation, scope. Modeled. Good.
- **Operator tenant:** Explicitly absent. Documentation says "V1 does
  not introduce operator tenancy. The operator trust boundary is the
  current OS user plus local file permissions." Good.
- **Shared workstation:** Explicitly deferred. Good.

The tenancy story is coherent. No remaining devil's-advocate attacks on
this axis.

## 6. Migration Audit

- **Existing `.striatum/state.sqlite3` runs:** `repo add` handles the
  daemon-mode case. `--no-migrate` covers the read-only inspection
  case. Direct-CLI migration trigger is the residual seam — see
  NB-R3-5. Material but non-blocking.
- **Direct-CLI fallback:** Explicit; no auto-probe; forced daemon mode
  never silently falls back; forced-daemon exit-code classes are
  tabulated. Good.
- **Repository removal/re-add:** Cascade semantics now defined per B2.
  `repository_id` non-reuse closes the capability laundering path.
  Good.

## 7. Provenance-Claim Audit

The synthesis is careful and consistent. Specifically clean:

- §"Staging Plan For Provenance Honesty" reaffirms that daemon mode
  does not strengthen lane attestation, does not implement sealed
  apply, does not create hosted service semantics, and that token
  labels, client ids, socket connections, and audit rows are never
  artifact bylines.
- `sealed_patch` is rendered as unsupported/unstartable in dashboard
  UI; the daemon does not add apply endpoints, key storage, signing,
  or receipt issuing.
- Daemon-emitted recovery event bylines are namespaced (`striatumd-…`)
  and never imply an operator/role/lane/model author.
- Daemon audit is per-machine, append-only at the API, hash-chained
  with cross-segment continuity, and explicitly disclaimed against an
  adversarial local filesystem writer.

No remaining overclaim. The strongest residual ("the hash chain can
detect some local tamper and corruption in retained manifests and rows,
but it is not transcript evidence, source-byte provenance, human
identity proof, model-token authorship proof, or resistance to a local
filesystem writer") is exact.

## 8. Required Revisions Before Implementation

None.

All eight findings above (NB-R3-1 through NB-R3-8) are non-blocking
implementer guardrails. They can be resolved during the build handoff
or carried as named follow-ups in `docs/TODO.md` with RFC-section
anchors.

The verdict acceptance condition is met: every load-bearing
devil's-advocate attack from round 1 and round 2 has survived the
strongest counterargument I can mount against revision 3.

## 9. Verdict

`accept_with_findings`.

All findings are explicitly non-blocking. Revision 3 of
`docs/dogfood/031/DESIGN_SYNTHESIS.md` is implementation-ready from the
devil's-advocate posture for the RFC 0028 V1 acceptance-criteria slice.
