---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["threat_model", "rfc-0042", "repo-local-pg", "build", "track_c"]
---

author: reviewer-claude-opus-005

# Track C Build Review: RFC 0042 (claude, threat_model)

## Scope and Posture

Threat-model review of `docs/rfcs/0042-repo-local-state-to-postgres.md`
under the per-lane angle assigned to `claude_code`:

- CLI behavior when the daemon is unavailable (RFC 0042 §5).
- Correctness of the RFC 0039 scope revision (RFC 0042 §6).
- Cleanliness of the D006 / D007 / D028 supersession framing
  (RFC 0042 "Supersession Statement").

Required-checks confirmation appears at the bottom. I do not
re-litigate the schema / migration-safety axis (codex angle) or
the adversarial-failure axis (gemini angle); those are covered by
the matching lanes.

Verdict intent: **accept**. Severity: **low**. The body is
implementable as written; the issues I name below are
documentation-level mis-attributions and one forward-reference
collision that the consolidation job must resolve before
DECISION_LOG.md is updated.

## Trust Boundaries Introduced By RFC 0042

Enumerated so the verdict can acknowledge them:

1. **CLI → daemon socket (Unix, owner-only).** RFC 0030 envelope-v1
   over the existing socket. RFC 0042 narrows what crosses it: every
   workflow-state read and mutation now flows through this socket
   (§5). Same boundary, more traffic.
2. **Daemon → Postgres.** The daemon owns the only authoritative
   substrate. Postgres credentials live behind the daemon-owning OS
   user (D083 boundary preserved). No CLI-side Postgres connection.
3. **Daemon → repo-local SQLite (read-only, one-shot).** Restricted
   to the inside of `repo.migrate_local_state`. The Go daemon embeds
   a SQLite reader used only by this RPC (§6). This is a real net-new
   surface but tightly scoped: admin-gated, advisory-locked, and
   never reused outside migration.
4. **Daemon → `.striatum/` scratch.** The daemon writes sockets,
   pidfiles, supervisor FIFOs, wrapper scripts, optional read-only
   tombstones (§4). `.striatum/` is no longer authoritative state.
5. **Capability token → repository scope.** Tokens are repository-
   scoped per the existing capability matrix; acceptance criteria
   require that a token scoped to repo A cannot read or mutate repo
   B workflow state. This is the multi-tenant boundary in the
   single-user sense D083 defines.
6. **Audit log → metadata-only.** §8 forbids artifact contents,
   prompts, verdict rationale, blocker descriptions, request bodies,
   and transcripts in audit rows; preserves D028's no-transcripts
   policy.

Each of these is acknowledged below in the corresponding §5 / §6 /
supersession analysis.

## 1. CLI Behavior When Daemon Unavailable (§5)

The strong claims worth verifying for threat-model acceptance:

- `--no-daemon` is removed for state-touching verbs; the CLI never
  silently falls back to direct SQLite.
- Read verbs (`status`, `why`, `dashboard`, evidence export, etc.)
  also route through the daemon — closing the read-side bypass.
- Wrapper surfaces (`striatum.api.invoke`, local service
  `POST /v1/invoke`, MCP tools, web chat tools) all dispatch through
  the same daemon path.
- Refusal envelope carries `{error, reason, remediation, docs}` and a
  named exit code (11–19).

These claims are written cleanly and they close the previously open
attack surface where direct CLI code paths could mutate
`.striatum/state.sqlite3` and bypass capability checks / audit
append (§"Problem" calls this out explicitly).

### Issues found

**(1a) `striatum init` is double-classified.** The RFC body lists
`init` under "lifecycle verbs" that are state-touching (§5
inventory), which means it must route through the daemon. But §4
also says `striatum init` "no longer creates `state.sqlite3` or
seeds workflow rows; it only creates scratch directories and ignore
rules." The on-ramp list in §5 ("`striatum daemon start`, `striatum
daemon doctor`, and `striatum daemon repo add` are the on-ramp
verbs that may run before a repository has migrated") does not
include `init`.

Effect: a new operator on a fresh machine who runs `striatum init`
before `daemon start` either (a) gets refused with
`daemon_unavailable` (exit 11) — surprising for a verb whose only
effects are scratch directories and ignore rules — or (b) bypasses
the daemon, contradicting §5's no-bypass rule. The RFC body should
either move `init` off the lifecycle list or explicitly add it to
the on-ramp verbs.

**(1b) `daemon doctor` may run pre-handshake but reads tombstones.**
§8 says `daemon doctor --repo <path> --check-migration` "optionally
recomputes [the events rollup] from the read-only tombstone." §5
names `daemon doctor` as an on-ramp verb. The tombstone is
`0400`-mode and lives in `.striatum/`, so the operator owns it. The
`cutover_marker_sha256` in `striatumd.repo_state_migrations` is the
tamper-evidence pin. This is acknowledged but worth naming: doctor
must run against the Postgres-side hash first and treat the
tombstone read as advisory — the body does not explicitly state the
ordering.

**(1c) Exit codes 16 and 17 are good additions** — operators can
tell PG-down from socket-down from drift. No issue; calling out
because it materially improves the threat model.

## 2. RFC 0039 Scope Revision (§6)

The §6 revision moves the Go daemon from "registry-only Phase 1
shape" to "day-one gateway for the full workflow-state surface
(reads + mutations + supervisor pointers)." This closes the
split-brain attack window where the Go daemon would own registry
rows while the Python CLI continued opening repo-local SQLite. §6
states that closure explicitly.

Concrete revisions to RFC 0039 listed in RFC 0042 §6 are correct
and testable:

- DB layer embeds the full migration tree, including
  `0005_repo_local_state.sql`.
- Read milestone covers both daemon-owned and former repo-local
  reads.
- Mutation milestone enumerates the full verb table (run.prepare /
  start, session.register, job.{claim_next, ack, heartbeat,
  publish_artifact, verdict, submit_review, complete, block},
  recovery.*, worktree.*, supervisor_pointer.*).
- Supervised wrappers do not open SQLite.
- The one allowed SQLite opening sits inside
  `repo.migrate_local_state`.
- The RFC 0035 multi-repo harness exercises the full lifecycle
  against both Python and Go daemon cores before the Go daemon
  ships.

The §6 acceptance criterion is correctly phrased as a verifiable
property: "no repo-local SQLite file is opened by the daemon except
inside `repo.migrate_local_state`." A future audit can test this by
strace / Go syscall hook.

### Issues found

**(2a) Phase 1 Steps 1+2 already landed; §6 effectively amends what
Step 2 must contain.** RFC 0039's Status line (top of file) says
"Phase 1 Steps 1+2 landed in dogfood-042; Steps 3-6 deferred to a
Phase 2 dogfood." RFC 0042 §6 says Step 2's DB layer must embed
`0005_repo_local_state.sql`. If RFC 0042 lands after dogfood-042's
Steps 1+2, Phase 2 must re-run Step 2 to embed the new migration.
The RFC body does not name this dependency. The consolidation job
should add one sentence to §6 making it explicit: "Phase 2 work
re-embeds the migration tree once `0005_repo_local_state.sql`
exists; the already-landed Phase 1 Step 2 work continues to apply
the prior tree (0001–0004) until that point."

**(2b) Embedded SQLite reader is a new attack surface in the Go
binary.** §6 says "the Go binary includes a SQLite reader used only
by the one-shot `repo.migrate_local_state` RPC." Threat-model
implication: a CVE in the embedded SQLite library (e.g.
`modernc.org/sqlite` per the synthesis) becomes a daemon-process
attack surface. The RFC body should name the mitigation: read-only
open, capability-gated (`admin`), short-lived per migration call,
no reuse across RPCs. The synthesis mentions `modernc/sqlite`
(CGO-free) which limits memory-safety blast radius; the RFC body
could echo that. Low severity but worth recording.

**(2c) Cross-daemon-core ordering during dogfood-042 transition.**
§6 says "Phase 1 of the Go rewrite does not ship until both cores
pass the same suite." RFC 0042 itself does not gate on Go daemon
parity for Python-only operators to migrate. That is the right
call (Python daemon can drive `repo.migrate_local_state` today),
but the RFC body could state it explicitly: "Python daemon ships
the migration verb in V1; Go daemon parity is a Phase 2 RFC 0039
deliverable, not an RFC 0042 acceptance criterion."

## 3. D006 / D007 / D028 Supersession (Supersession Statement)

The RFC body's Supersession Statement is short and direct:

- D006: SQLite live-state substrate superseded by daemon Postgres;
  "repository files are durable provenance, not the live message
  bus" preserved.
- D007: `.striatum/state.sqlite3` location superseded; `.striatum/`
  remains per-repository scratch root.
- D028: "direct-CLI-write path into local state is superseded by
  daemon-mediated writes; curated artifact policy and
  no-transcripts default remain unchanged."

D006 and D007 partial-supersession framings are correct, clean, and
preserve the right halves.

### Issues found

**(3a) D028 is mischaracterized.** D028 as recorded in
`docs/DECISION_LOG.md` reads: *"Artifact policy: decisions, prompts,
findings, syntheses, markers, handoffs, and other idempotent build
artifacts are repo-published; transcripts are not captured or
published by default."*

D028 is **entirely about artifact policy**. There is no "direct-CLI-
write path into local state" half of D028 in the decision log.
RFC 0042's Supersession Statement attributes that half to D028
nonetheless. The synthesis (`docs/dogfood/042/track_c/DESIGN_SYNTHESIS.md`
§9) makes the same mis-attribution.

The actual decision that puts the binary in charge of SQLite writes
is **D009**: *"Agents update orchestration state through the
`striatum` binary/CLI. The binary owns SQLite writes, schema
invariants, leases, acknowledgements, state transitions, verdict
rules, and artifact validation."* RFC 0042 effectively supersedes
the "binary owns SQLite writes" half of D009 (the daemon-RPC layer
now owns those writes). D009 is not named in the supersession list
anywhere in the RFC body.

Recommended fix for the consolidation job:

- Restate D028 supersession as: "D028's no-transcripts artifact
  policy is preserved verbatim. RFC 0042 reinforces it by forbidding
  artifact contents, prompts, verdict rationale, blocker
  descriptions, request bodies, and transcripts in `striatumd.audit_log`
  rows (§8)." There is no D028 "half" to supersede.
- Add D009 to the supersession list: "D009's 'binary owns SQLite
  writes' half is superseded; daemon RPC writes the daemon-owned
  Postgres substrate. D009's 'narrow control surface for state
  mutation' principle is preserved and reinforced by the §5
  no-bypass rule."

**(3b) Forward reference to D093 collides with the existing D093.**
The Supersession Statement closes with: *"The Track C synthesis
identifies D093 as the decision-log entry for the umbrella
supersession; this RFC references that expected decision and leaves
decision-log reconciliation to the consolidation job."*

The synthesis is more confident: *"The DECISION_LOG entry for D093
(already accepted) is the umbrella decision."*

The actual D093 row in `docs/DECISION_LOG.md` is the **RFC 0040 V1
acceptance** (dogfood-041 / dogfood-driven MCP chat tools), not a
supersession umbrella for repo-local PG migration.

This is internally inconsistent:

- The synthesis claims D093 is already accepted as the umbrella
  decision — false against the current decision log.
- The RFC body more cautiously says D093 is the "expected
  decision-log entry" — but the ID is already taken.

Recommended fix for the consolidation job: rename the umbrella
decision to the next available ID (currently D094) and edit both
RFC 0042 §"Supersession Statement" and synthesis §9 to reference
that ID. Do this before DECISION_LOG.md is updated; otherwise the
consolidation PR will either overwrite D093 (losing RFC 0040 V1
acceptance) or land a row whose pointers do not match the RFC body.

**(3c) D082 / D086 / D087 / D088 reinforcement is correctly written.**
RFC 0042 names that D086 has a loophole clause ("V1 repo-local
SQLite remains the authoritative run-state store during the
transition") and that RFC 0042 closes it. Good — this is exactly
the kind of decision-log housekeeping a threat-model review wants.

**(3d) D083 preservation is explicit.** "The trust boundary remains
one OS user per machine. 'Multi-tenancy' in RFC 0042 means schema-
level row isolation keyed by `repository_id`." This forecloses the
threat-model misreading where capability-token scope per repository
might be confused with multi-user auth. Good.

## Required-Checks Confirmation

The build-review prompt names six checks. Status against the RFC
body as written:

| Check | Status | Notes |
| --- | --- | --- |
| Acceptance criteria implementable | ✓ | Thirteen criteria in §"Acceptance Criteria" are concrete; row-count parity, composite FK validation, audit chain v2 hash, refusal codes, and the Go-daemon line are all testable. |
| D006 / D007 / D028 supersession explicit, references D093 | partial | Explicit, but D028 is mischaracterized (3a), D009 is missing (3a), and the D093 ID collides with the existing D093 (3b). Body is otherwise clean. |
| `.striatum/` → operational scratch boundary defined | ✓ | §4 enumerates allowed contents (socket, pidfile, supervisor FIFOs, wrapper scripts, optional tombstone) and forbidden contents (live mutable `state.sqlite3`, authoritative markers). |
| Migration verb spec includes `--dry-run` + `--keep-sqlite-readonly` | ✓ | §3 spells out both flags. Dry-run writes nothing (no audit row, no cutover marker). `--keep-sqlite-readonly` produces a `0400` tombstone at `.striatum/state.sqlite3.pre-cutover.<timestamp>`. |
| RFC 0039 scope revision call-out | ✓ | §6 names the day-one gateway requirement, enumerates the verb table, names the embedded SQLite reader scope, and gates Phase 1 ship on RFC 0035 harness parity. See 2a / 2b / 2c for documentation polish. |
| Single-user trust (D083) unaffected | ✓ | Explicitly preserved in §"Supersession Statement" and the §1 "row-level isolation, not multi-user" framing. |

## Threat-Model Verdict Surface

Each trust boundary enumerated above is either acknowledged or
mitigated by the RFC body:

| Boundary | Status | Mitigation in RFC body |
| --- | --- | --- |
| CLI → daemon socket | acknowledged | RFC 0030 envelope-v1, owner-only socket, version-skew handshake (§5). |
| Daemon → Postgres | acknowledged | D083 boundary preserved; daemon-owning OS user only. |
| Daemon → SQLite (read-only, migration) | acknowledged + partially mitigated | Admin-gated, advisory-locked, one-shot. 2b suggests naming the CGO-free library as additional containment. |
| Daemon → `.striatum/` scratch | acknowledged | §4 enumeration of allowed contents; `daemon doctor` verifies wrapper hashes and socket permissions. |
| Capability token → repo scope | acknowledged | Acceptance criterion: token scoped to repo A cannot read or mutate repo B workflow state. |
| Audit log → metadata-only | acknowledged | §8 explicitly forbids contents/prompts/rationale/bodies/transcripts. |

All six boundaries are explicitly named and either bounded or
mitigated by the proposal. Accept.

## Suggested Edits For The Consolidation Job

Non-blocking; the body is implementable as written.

1. (3a) Rewrite the D028 paragraph of "Supersession Statement" to
   state that D028 is preserved verbatim (no halves). Add a new D009
   paragraph: "D009's 'binary owns SQLite writes' clause is
   superseded by daemon-mediated RPC writes; the narrow-control-
   surface principle is preserved and reinforced by §5."
2. (3b) Re-target the umbrella supersession decision to the next
   available ID (D094 at time of this review) in both the RFC body
   and `docs/dogfood/042/track_c/DESIGN_SYNTHESIS.md` §9. The
   existing D093 is the RFC 0040 V1 acceptance decision and must
   not be overwritten.
3. (1a) Either move `striatum init` off the §5 lifecycle/state-
   touching verb list, or add it to the on-ramp verbs explicitly.
   Current text implies both.
4. (2a) Add one sentence to §6 noting that the Phase 1 Go daemon
   work that already landed in dogfood-042 (Steps 1+2) will re-embed
   the migration tree in Phase 2 once `0005_repo_local_state.sql`
   exists.
5. (2b) Add one sentence to §6 naming the SQLite-reader containment:
   read-only open, admin-capability-gated, one-shot per migration
   RPC, CGO-free library.
6. (2c) State explicitly that the Python daemon ships the migration
   verb in RFC 0042 V1; Go daemon parity is a Phase 2 RFC 0039
   deliverable, not an RFC 0042 acceptance criterion.

None of the items above blocks RFC 0042 acceptance. Items (3a) and
(3b) must be resolved before `docs/DECISION_LOG.md` is updated in
the consolidation PR; otherwise the decision log will either
overwrite an accepted row (D093) or carry a row whose body
references a decision number that has already been claimed.
