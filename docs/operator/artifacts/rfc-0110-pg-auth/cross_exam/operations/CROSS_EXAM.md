# RFC 0110 — Cross-examination (operations posture, cycle 2)

author: cross-examiner-claude-opus-4.8-002
artifact_kind: handoff
logical_name: cross_examiner_5
workflow: rfc-0110-pg-auth-panel
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 2
posture: operations
target: docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_2.md
inputs:
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/draft/CANDIDATE_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/operations/CROSS_EXAM.md (cycle 1, this posture)
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/synthesis/CROSS_EXAM_SYNTHESIS_cycle_1.md
  - go/pkg/db/connection.go
  - go/pkg/db/audit.go

## Charge and posture

This is the **cycle-2 operations** cross-examiner of the RFC 0110 candidate
synthesis, re-run against the revised `SYNTHESIS_cycle_2.md` /
`CANDIDATE_cycle_2.md`. I challenge **only from the day-2 operations surface**:
deploy, upgrade, rollback, restart, disaster-recovery, single/multi-instance,
and operator diagnosability. Product, implementation, privacy, and eval are owned
by the sibling cross-examiners; I do not re-litigate the v3 hash design or the
spoofability proof of `daemon_auth` on their own terms — only where they change
how an operator deploys, restarts, recovers, and diagnoses.

**What the revision got right (recorded so the adjudicator can close it).** The
cycle-1 operations set (OPS-1…OPS-8 → `C-RESTART-OWNER-DEP`,
`C-L0-ADOPTION-VISIBLE`, `C-ROLLBACK-FORWARD-ONLY`, `C-DDL-DEPLOY-ORDER`,
`C-ROTATION-SINGLE-WRITER`, `C-OWNER-DR`, `C-DOCTOR-OWNER-REACH`,
`C-SOCKET-RELOCATE-MIGRATION`) is **genuinely folded into the spec text** — §5.2
fail-closed + owner-attributable diagnostic, §5.3 single-role doctor posture
finding, §4.5 flagged forward-only cutover (`audit.hash_format` default v2), §4.4
deploy ordering + startup precondition, §5.4 single-writer invariant + per-instance
role, §5.6 owner break-glass, §5.7 privilege-safe SD posture probe, §8.4 socket
blast-radius note. Each now carries a falsifiable gate. **From the operations
posture these eight are adequately discharged at the spec level** and I do not
re-open them; their gates remain for the implementer to make real.

**Where the revision opened new operations surface.** The load-bearing cycle-2
change — the **non-spoofable `daemon_auth` authority gate** (§2/§3) that closes the
critical `C-EXEC-AUTH` hole — is sound product/security-wise, but it inserts a
**new mechanism onto the critical write path** (`assert_daemon_authority()` runs
inside `append_audit_row`, which sits on every authoritative mutation). The
operations contract for that mechanism is **incomplete**, and the cycle-2
multi-host resolution (§5.4 per-instance roles) contradicts its own concurrency
probe. The four findings below are confined to that new surface and are grounded
in the cycle-2 text and current source on the run branch. None of the four
**re-litigates a closed cycle-1 row**; each is a genuinely new operations hazard
created by the cycle-2 remedy.

The candidate's "standing live for cross-examination" list (CANDIDATE §end)
pre-flags four attackable claims — spoofability, attribution-bleed, deploy-skew,
and hash parity. **Three of the four are product/impl/eval framings; none names
the day-2 operability of the authority gate** (its mid-run liveness, its
deploy-capability skew, or its multi-host probe). That gap is, again, the
evidence this posture exists to record.

---

## Lead falsifying interrogation (recorded; this job is non-interrogable)

> **Q (operations):** `assert_daemon_authority()` (CANDIDATE §2.2) accepts the
> presented secret only if it matches the registry digest **"within the
> registry's freshness window."** The secret is generated once at bootstrap, the
> registry row is UPSERTed once (§5.1), and the secret is held "for the pool's
> lifetime." On a daemon that has been up for days without a restart: **what is
> the freshness window, what refreshes the registry row before it lapses, and
> what does an operator see the moment it lapses** — given that `append_audit_row`
> is on the path of *every* authoritative mutation?

**Why no rebuttal was possible.** This job is a non-interrogable review job
(packet `interrogable:false`); as in cycle 1 the interrogation is recorded
textually. The candidate's own standing-for-cross-exam list does not address
mid-run authority liveness, and §5.2's fail-closed covers only *bootstrap* owner
failure — not a *mid-run* authority lapse. **T-OWNER-FAILCLOSED** tests bootstrap;
**T-EXEC-AUTH** tests rejection of an *absent/wrong* secret. Neither tests a
*correct* secret aging out of the window. The spec therefore leaves the answer
undefined in exactly the dimension that decides whether the authority gate is an
availability time-bomb. Recorded as **OPS-9** below — the single most load-bearing
new operations finding.

---

## findings[]

Severity scale: **HIGH** = silent availability wedge of a hard-prerequisite
daemon / silent inversion of a stated goal; **MEDIUM** = recoverable but
undocumented operator hazard; **LOW** = friction. All four rows are **new in
cycle 2** — created by the cycle-2 remedy, not carried from cycle 1.

| id | sev | affected invariant | one-line challenge | closest acceptable answer |
| --- | --- | --- | --- | --- |
| **OPS-9** | HIGH | write availability (`append_audit_row` on every mutation; `assert_daemon_authority` now gates it) | The authority "freshness window" is undefined with no refresh contract; if finite and unrefreshed, a long-running daemon's correct secret ages out → every write `RAISE`s `28000` with no operator signal — a silent total-write wedge created by the `C-EXEC-AUTH` fix. | Define the window semantics: lifetime-of-instance validity (window bounds only *dead-instance* digest reuse) **or** a refresh cadence that fails closed with an owner-attributable diagnostic + a doctor "authority expiring/lapsed" probe. Add a mid-run authority-liveness test. |
| **OPS-10** | HIGH | deploy safety / write availability (the invariant `C-DDL-DEPLOY-ORDER` was extracted to protect) | §4.4's startup precondition checks function **presence** (`to_regprocedure(append_audit_row)`, pgcrypto), not **authority-capability parity**. Owner DDL vN (functions now call `assert_daemon_authority`) + an old vN-1 binary that never sets `striatum.daemon_auth` **passes the precondition, then fails every write at runtime** — re-opening the exact audit-write wedge C-DDL-DEPLOY-ORDER promised to make fail-fast. | The precondition must assert **capability parity** (owner DDL stamps a schema-contract/`requires_daemon_auth` marker; the binary refuses to serve unless its authority capability ≥ the schema's requirement) and must check the **whole** authority dependency set (registry table + `assert_daemon_authority`), not just `append_audit_row`. |
| **OPS-11** | HIGH | the RFC's own multi-principal / remote-PG substrate goal (RFC 0107), and operator trust in doctor | §5.4's concurrent-rotator probe alarms on "a recent `daemon_auth_registry.rotated_at` from a **different `instance_id`**" — but §5.4's own multi-host resolution gives each instance a per-instance role and its **own** registry row, so **every legitimate multi-host node trips the probe** on its peers. The two halves of §5.4 contradict; the probe keys on instance-id difference, not role collision. | Key the probe on **role collision** (two live instance_ids that rotated the **same** runtime role within a window), recording the rotated role in the registry; sanctioned per-instance-role peers (distinct roles) must not trip it. Add a multi-host no-false-alarm test. |
| **OPS-12** | MED | pool/write availability under transient PG stress; interaction with rotation | §3.3 destroys (not just `DISCARD ALL`-resets) any connection whose tx errored/cancelled. With the pool's default `statement_timeout=60000` and `context` cancellation, a transient PG blip becomes a connection-destruction + reconnection storm; if it overlaps a rotation (a peer restart / manual `ALTER ROLE` before §5.4 per-instance roles land), the mass reconnect hits with the old password → `28P01` pool collapse with no self-heal — OPS-5's mechanism, amplified by the discard policy. | Bound the discard: discard only on **attribution-poisoning** errors; for transient operational errors (timeout/cancel/serialization) a `DISCARD ALL` reset suffices without destroy. Add reconnect backoff + a doctor signal when reconnect-auth failures spike (catches a rotation collision in-band). Document the discard↔rotation interaction. |

---

## Detailed findings

### OPS-9 — The `daemon_auth` freshness window is an undefined silent write-wedge (HIGH)

**Invariant.** Write availability. `append_audit_row` sits on the critical path of
**every** authoritative mutation; the cycle-2 change makes it begin with
`assert_daemon_authority()` (CANDIDATE §2.2, §4.3). Anything that makes that assert
fail makes **all** mutations fail — and the daemon is a hard prerequisite for every
verb (AGENTS.md Product Boundary).

**Challenge.** `assert_daemon_authority()` "compares `sha256(presented || salt)` to
`daemon_auth_registry.digest` for the current instance, **within the registry's
freshness window**" (CANDIDATE §2.2, verbatim). The spec introduces a *freshness
window* but never defines it, and the lifecycle around it is single-shot: the
secret is generated once at bootstrap, the registry row is UPSERTed once (§5.1),
and the secret is "held in process memory for the pool's lifetime" (§2.2). There is
**no refresh path** in the spec. Two readings, both bad:

- **Finite window, no refresh** → a daemon that stays up longer than the window
  (the *normal* state of a long-lived service) crosses the boundary and
  `assert_daemon_authority()` starts raising `28000` on a **correct** secret. Every
  audit/artifact/event write fails while the process and the runtime credential are
  perfectly healthy. The only "fix" is a restart — turning routine uptime into a
  scheduled outage, and a *hard-prerequisite* one.
- **Infinite window** → it is not a freshness window at all; a stale digest from a
  crashed prior instance (or a reused `instance_id`) lingers indefinitely, which
  *also* undercuts the §5.4 concurrent-rotator story (a dead instance's row stays
  "recent-ish" forever).

Either way the operator has **no probe**: §5.2's fail-closed and **T-OWNER-FAILCLOSED**
cover only *bootstrap* owner failure; **T-EXEC-AUTH** asserts an *absent/wrong*
secret is rejected. Neither exercises a *valid* secret aging out mid-run. So the
cycle-2 fix for the **critical** finding installs a new **silent total-write
wedge** — precisely the class (silent availability inversion) this posture exists
to catch, now produced by the remedy.

**Candidate's current answer.** None. §2.2 names the window; nothing defines its
length, refresh, or operator-visible lapse behavior.

**Closest acceptable answer.** Make the authority lifecycle explicit and
operable, choosing one:
- **(a) Lifetime-of-instance validity:** the registry row is valid for the live
  instance's whole lifetime; the "freshness window" bounds only **reuse of a dead
  instance's digest** (a liveness heartbeat in `rotated_at`, or a teardown that
  marks/deletes the row on graceful shutdown + a doctor probe for an orphaned row).
  A live daemon never self-wedges.
- **(b) Finite window with a refresh contract:** the daemon re-UPSERTs the digest
  before expiry; a refresh failure **fails closed with an owner-attributable
  diagnostic** (same shape as §5.2) rather than silently raising `28000` on the
  write path; `daemon doctor` reports "authority window lapsing/lapsed" as a posture
  finding so the wedge is diagnosable in-band.

Add a mid-run liveness test (below) so the window's day-2 behavior is pinned, not
implied.

**Required constraint shape — `C-AUTH-WINDOW-LIVENESS`:** the `daemon_auth`
freshness window has a defined lifecycle — a live instance never self-wedges, and
any authority lapse is fail-closed + owner-attributable + doctor-visible, never a
silent `28000` on the mutation path.

> **T-AUTH-LIVENESS** (proposed): a daemon whose registry row has aged past the
> window (or whose refresh was forced to fail) **either** still writes (lifetime
> validity) **or** surfaces an owner-attributable fail-closed + a doctor posture
> finding — and in no case silently fails every write with no operator signal.

### OPS-10 — Startup precondition checks presence, not authority-capability parity; the authority gate re-opens the deploy-skew wedge (HIGH)

**Invariant.** Deploy safety / write availability — the very invariant
`C-DDL-DEPLOY-ORDER` (my cycle-1 OPS-4) was extracted to protect: version skew must
**fail fast with an actionable error**, never silently wedge audit writes.

**Challenge.** §4.4's precondition checks **presence**:
`to_regprocedure('striatumd.append_audit_row(...)')` present and `pgcrypto`
installed. That was sufficient in cycle 1, when the only skew was "function
missing" or "REVOKE before binary." **Cycle 2 changed the function body**: every
write fn now calls `assert_daemon_authority()` and depends on the runtime binary
setting `striatum.daemon_auth` in its transaction prelude (§3.1) — a capability the
*pre-cycle-2 binary does not have*. This creates a skew direction the presence
check cannot see:

- **Owner DDL vN applied (functions call `assert_daemon_authority`, registry table
  exists), running binary is vN-1** (predates §3, never sets the secret). The
  precondition finds `append_audit_row` present → **passes**. Then **every** write
  calls `assert_daemon_authority()`, finds no secret in the GUC, and `RAISE`s
  `28000` → **every audit write fails at runtime, after the precondition cleared
  it.** This is exactly the "silently wedge audit writes" outcome C-DDL-DEPLOY-ORDER
  promised to convert into a fail-fast — now re-opened *through the authority gate*.

§4.4 hand-waves that the precondition "detects the mismatch (binary capability vs
schema state)," but the only mechanism named is `to_regprocedure` presence, which
carries **no** information about whether the calling binary satisfies the
function's new runtime authority requirement. The precondition also checks only
`append_audit_row` + pgcrypto — **not** the registry table or
`assert_daemon_authority` itself, so a partial owner-DDL apply (functions present,
registry absent) likewise passes presence yet fails at the assert.

**Candidate's current answer.** §4.4 claims the precondition catches both skew
directions, but its stated check is presence-only and predates the authority gate
it must now also guard.

**Closest acceptable answer.** The startup precondition must assert
**capability parity**, not mere presence: the owner DDL stamps a
schema-contract marker (e.g., `schema_contract_version` / a `requires_daemon_auth`
flag in an owner-owned meta row), and the binary **refuses to serve unless its own
authority capability ≥ the schema's required capability**. The check must cover the
**whole** authority dependency set (`append_audit_row` **and**
`assert_daemon_authority` **and** `daemon_auth_registry` **and** pgcrypto), so a
binary/schema authority mismatch fails closed with an actionable diagnostic
*before* the first mutation, not as a runtime `28000` storm.

**Required constraint shape — `C-DEPLOY-CAPABILITY-PARITY`** (sharpens
`C-DDL-DEPLOY-ORDER`): the startup precondition verifies binary↔schema **authority
capability parity** over the full authority dependency set; an old-binary /
authority-bearing-schema skew fails closed with an actionable error, never a
runtime `28000` wedge.

### OPS-11 — The concurrent-rotator probe false-positives on the multi-host posture §5.4 enables (HIGH)

**Invariant.** The RFC's stated RFC-0107 multi-principal / remote-PG substrate goal
**and** operator trust in `daemon doctor` (a probe that cries wolf on every healthy
multi-host node is worse than no probe).

**Challenge.** §5.4 resolves OPS-5 with two clauses that collide:

- **Detection (default/local):** `daemon doctor` flags a concurrent rotator on "a
  recent `daemon_auth_registry.rotated_at` from a **different `instance_id`**."
- **Enablement (remote/multi-host):** use a **per-instance role**
  (`striatumd_rw_<instance>`) so concurrent daemons never share a rotated
  credential.

But `daemon_auth_registry` is keyed `(instance_id, digest, rotated_at)` in **one**
shared PG. In the sanctioned multi-host posture, **each** instance writes **its
own** row with its own `instance_id` and a recent `rotated_at`. So every node,
running the same probe, observes "a recent rotation from a different `instance_id`"
— and **every legitimate peer trips the concurrent-rotator finding on every other
peer.** The probe keys on *instance-id difference*, which is the normal state of
the multi-host deployment §5.4 is trying to enable; it cannot distinguish a hostile
or accidental second daemon **sharing my role** from a sanctioned peer **with its
own per-instance role**. The two halves of §5.4 are mutually inconsistent: the
detection makes the enablement un-operable without a permanent false alarm.

**Candidate's current answer.** §5.4 states both clauses but never reconciles the
probe's keying with the per-instance-role multi-host model it also prescribes.

**Closest acceptable answer.** Key the probe on **role collision**, not instance-id
difference: record the rotated **role** in the registry and alarm only when two
**distinct live `instance_id`s rotated the same runtime role** within a window.
Sanctioned per-instance-role peers (distinct roles) must **not** trip it; a real
shared-role second daemon **must**. State this so the remote-PG path is operable
without doctor noise.

**Required constraint shape — `C-ROTATOR-PROBE-ROLE-SCOPED`** (sharpens
`C-ROTATION-SINGLE-WRITER`): the concurrent-rotator probe is **role-collision
scoped** (two live instances on the same role), so the §5.4 per-instance-role
multi-host posture produces zero false positives while a genuine shared-role
second writer is still detected.

### OPS-12 — Discard-on-error can turn a transient blip into a reconnection storm that collides with rotation (MEDIUM)

**Invariant.** Pool/write availability under transient PG stress, and the
interaction of connection churn with the single-writer rotation model.

**Challenge.** §3.3 (correctly, for attribution hygiene per EV-004) **destroys** —
not merely `DISCARD ALL`-resets — any connection whose transaction errored or was
cancelled (`pgxpool` destroy). Current source grounds the blast radius: the pool is
built with a default `statement_timeout=60000` and `QueryExecModeSimpleProtocol`
(`connection.go`, `Connect`), and handlers run under cancellable `context`. So a
transient PG slowdown (lock contention, a slow disk, a checkpoint storm) that
produces a **burst** of statement-timeouts / context-cancels destroys a **burst**
of connections; `pgxpool` then re-establishes them — a thundering herd of new
backends during the exact window PG is already stressed. Worse, that mass
re-establish **re-authenticates** as `striatumd_rw`: if the blip overlaps a
rotation (a peer restart, or a manual `ALTER ROLE` in the pre-§5.4 shared-role
posture), the reconnect storm hits with the **old** password → `28P01` on every new
connection → pool collapse with **no self-heal without a restart** (the OPS-5
mechanism, now *amplified* by the discard policy rather than bounded). The discard
policy is right for hygiene; its day-2 interaction with reconnect + rotation is
unbounded and undocumented.

**Candidate's current answer.** §3.3 specifies the discard for correctness; it does
not bound the resulting reconnect behavior or note the rotation interaction.

**Closest acceptable answer.** Distinguish **attribution-poisoning** errors (must
destroy the connection) from **transient operational** errors
(timeout/cancel/serialization), where a `DISCARD ALL` reset is sufficient without
destroying the connection; add reconnect backoff so a blip cannot become a herd;
and emit a `daemon doctor` / log signal when reconnect-auth failures spike, so a
rotation collision is caught **in-band** rather than as a silent pool collapse.
Document the discard↔rotation interaction in the L0 runbook.

**Required constraint shape — `C-DISCARD-RECONNECT-BOUND`** (sharpens
`C-ATTR-RESET-FAIL` × `C-ROTATION-SINGLE-WRITER`): connection destroy is scoped to
attribution-poisoning errors; transient operational errors reset without destroy;
reconnect is backoff-bounded and a reconnect-auth-failure spike is doctor-visible.

---

## What the adjudicator should extract (operations rows, cycle 2)

The eight cycle-1 operations constraints remain extracted and are **adequately
folded** in cycle 2 (close them against their §-anchors and gates). The **four new**
rows below are additive and target only the cycle-2 surface; they are
severity-ordered.

| id | binding constraint | sev | verified by |
| --- | --- | --- | --- |
| `C-AUTH-WINDOW-LIVENESS` | `daemon_auth` freshness window has a defined lifecycle; a live instance never self-wedges, and any authority lapse is fail-closed + owner-attributable + doctor-visible, never a silent `28000` on the write path. | HIGH | **T-AUTH-LIVENESS** (aged/refresh-failed instance: still writes OR fail-closed+doctor, never silent) |
| `C-DEPLOY-CAPABILITY-PARITY` | startup precondition verifies binary↔schema authority-capability parity over the full authority dependency set (`append_audit_row` + `assert_daemon_authority` + `daemon_auth_registry` + pgcrypto); authority skew fails closed pre-mutation. | HIGH | deploy-skew test: authority-bearing schema + non-authority binary fails the precondition with an actionable error (not a runtime `28000`) |
| `C-ROTATOR-PROBE-ROLE-SCOPED` | the concurrent-rotator probe is role-collision scoped (two live instances on the same role); the §5.4 per-instance-role multi-host posture yields zero false positives, a genuine shared-role second writer is still flagged. | HIGH | multi-host no-false-alarm test + shared-role positive-detection test |
| `C-DISCARD-RECONNECT-BOUND` | connection destroy scoped to attribution-poisoning errors; transient operational errors reset without destroy; reconnect is backoff-bounded; reconnect-auth-failure spike is doctor-visible. | MED | transient-error-burst test (no connection-destroy storm) + rotation-collision doctor-signal test |

---

## Posture verdict

**The cycle-2 revision genuinely closed the cycle-1 operations findings** — all
eight are folded into spec text with falsifiable gates, and from this posture they
are adequately discharged. Credit recorded.

**But the cycle-2 remedy for the *critical* finding introduced new operations
surface that is not yet implementation-ready.** Two HIGH findings are silent
availability wedges of a hard-prerequisite daemon created by the new authority
gate — **OPS-9** (the freshness window is undefined → a long-lived daemon's correct
secret can age into a silent total-write wedge) and **OPS-10** (the deploy
precondition is presence-only → an authority-bearing schema + a non-authority
binary passes the check, then `28000`-wedges every write, re-opening
`C-DDL-DEPLOY-ORDER`). A third HIGH, **OPS-11**, is a structural contradiction
inside §5.4: the concurrent-rotator probe false-positives on the per-instance-role
multi-host posture it is meant to enable. **OPS-12** (MED) is an availability
amplifier where the (correct) discard-on-error policy can turn a transient blip
into a rotation-colliding reconnect storm.

None of the four requires abandoning the cycle-2 design; each is a **bounded
spec-completion item** — define the authority window's lifecycle, make the deploy
precondition capability-aware, role-scope the rotator probe, and bound the discard.
Recommended disposition from the operations posture: **needs_revision**, narrowly,
with the four `C-*` rows above extracted as binding constraints for the revision
convener to discharge. The design is sound; the new mechanism's **operational
contract** is incomplete.

— recorded for the cross-exam synthesis (`../synthesis/`) and adjudication.
