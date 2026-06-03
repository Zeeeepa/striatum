# RFC 0110 Product Cross-Exam
author: cross-examiner-codex-gpt-5.5-xhigh-003
artifact_kind: handoff
logical_name: cross_examiner_1
workflow: rfc-0110-pg-auth-panel
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 2
posture: product
target: docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_2.md
inputs:
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/draft/CANDIDATE_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/synthesis/CROSS_EXAM_SYNTHESIS_cycle_1.md
  - docs/rfcs/0110-daemon-postgres-authentication-and-database-enforced-write-boundary.md
  - docs/decisions/decision-log.md
  - docs/operator/BRIEF.md

## Interrogation

Target attempted: `sess_eab020240ffd8880cae29de0707d17b5`

Challenge I attempted to put to the cycle-2 convener:

> The cycle-2 synthesis says a leaked runtime DSN string becomes
> "uninteresting" and sequences L1 as audit_log -> artifacts -> events. Is that
> product claim limited to unauthorized mutation and hash/attempt invariant
> violation, or does it also cover read access and future durable-table writes?
> Which exact read surfaces, protected-table phases, and durable mutation classes
> may product docs call closed at each phase?

Structured turn reference: `interrogation.open` returned `capability_denied` with
message `interrogator session lacks the 'interrogate' capability` for this
interrogator session `sess_05a3992de33c494d7a3e56698aa2b6f6`, audit id
`6737178`. No interrogation id was created, so no `interrogation.ask` or target
rebuttal was possible.

Rebuttal reference: none. Because the question was not delivered, the absence of
a rebuttal is process evidence only; the findings below are grounded in the
published cycle-2 synthesis, RFC 0110, D164, and the current operator brief.

## findings[]

| id | severity | affected invariant | finding | closest acceptable answer | constraint shape required |
| --- | --- | --- | --- | --- | --- |
| PX3-001 | high | D164 "make a leaked runtime credential uninteresting"; local-first privacy boundary for daemon-owned PostgreSQL. | Cycle 2 correctly narrows the write claim with G1/G2, but still uses the broad phrase "a leaked DSN string is uninteresting." L1 revokes direct DML and gates write functions; it does not say that `striatumd_rw` loses broad `SELECT` on artifacts, events, sessions, queue/messages, principals, blockers, or payload JSON. A leaked DSN that cannot mutate can still be product-interesting if it can read authoritative state or private metadata. | Either narrow the claim to "a leaked DSN string is uninteresting for unauthorized mutation and hash/attempt invariant violation" or add an explicit least-privilege read contract for `striatumd_rw` with named allowed read surfaces and denied private surfaces. | `C-DSN-READ-SCOPE`: acceptance lists which tables/views `striatumd_rw` may read, which it must not read, and why. A PG-gated negative test proves a raw runtime connection cannot read any surface the product classifies as private; if reads remain broad, D164/spec language says this RFC is a write-boundary RFC, not a credential-confidentiality close. |
| PX3-002 | high | "The daemon RPC/artifact API is the sole durable write path"; phased audit_log -> artifacts -> events rollout. | The cycle-2 sequencing is phase-correct, but the product claim can be closed too early. During Phase 0 only `audit_log` is behind the new write function; during Phase 1 `events` may still be directly writable; during Phase 2 the full durable provenance surface is finally covered. If release notes, doctor, or the RFC says "database-enforced write boundary" before all protected surfaces are covered, the product overstates the live security posture. | Make the guarantee phase-scoped. Phase 0 may claim "audit writes are DB-gated"; Phase 1 may add "artifact writes are DB-gated"; only after Phase 2 plus the event transcript gate may the product claim the daemon API is the sole durable write path across audit/artifacts/events. | `C-PHASED-WRITE-CLOSURE`: every phase has a named protected-table set, corresponding direct-DML negative tests, and a doctor/status posture string. The final "sole durable write path" claim is gated on all three surfaces: `audit_log`, `artifacts`, and `events`. |
| PX3-003 | high | Audit provenance remains operator-verifiable across the PL/pgSQL hash cutover. | D164 called out the load-bearing L1 risk: current `VerifyRows` depends on Go JSON hashing with alphabetical key order, so moving append into PL/pgSQL can break every chain if the row format changes without verifier support. Cycle 2 answers with v3 bytea hashing, a verifier dispatch, and a default-off `audit.hash_format` flag, but this must be a product release gate, not only an eval detail. A SQL append function that creates rows the shipped verifier cannot validate destroys durable provenance even if the database write path is otherwise gated. | Treat audit format cutover as a named product state: no v3 row is producible until the binary contains `VerifyRows` v2/v3 dispatch, unknown-format failure, mixed-chain verification, and a runbook/doctor distinction between format skew and tamper. | `C-AUDIT-FORMAT-CUTOVER`: Phase 0 cannot claim "audit writes are DB-gated and valid" until `append_audit_row`, SQL v3 hash, Go `V3RowHash`, `VerifyRows` dispatch, the default-v2 cutover flag, and mixed-format tests ship together. Acceptance proves flag-off writes remain v2 and rollback-verifiable; flag-on writes are v3 and verifier-green. |
| PX3-004 | high | #87 closure and L2 "hardened default" without stranding upgrades. | Cycle 2 improves L2 by splitting secure-profile/fresh installs from legacy upgrades, but the current operator brief says #87 is only partial and the PG-less lane OS user is not built. A default-false legacy flag plus warning does not close #87 for the deployed live posture. Calling L2 a hardened default or issue closure before the supervised lane actually runs as a PG-less OS user with a blocking doctor path would mislead operators. | Treat L2 as target posture until enforced. Fresh/secure-profile installs can block; legacy can warn for compatibility; but #87 should remain partial/open until the PG-less OS-user path is implemented, the default-on successor is named, and live supervised lanes no longer inherit PG reachability by default. | `C-87-CLOSURE-GATE`: RFC/runbook records explicit closure criteria for #87: dedicated lane OS user available, PostgreSQL socket/TCP denial proven by `T-LANE-ISOLATION-NEG`, `daemon doctor` blocks under hardened posture, and issue/status language says "partial" until those gates are green in the deployed default or named secure profile. |
| PX3-005 | medium | Accepted decision D164 remains the operator-facing product contract. | The cycle-2 synthesis materially amends D164: L3 moves from `pgxpool.BeforeAcquire SET LOCAL` to an in-transaction prelude; GUCs become labels only; a new `daemon_auth` gate becomes the authority boundary; v3 hash supersedes the survey's JSON-parity wording for new rows; product claims become phase-scoped. The implementation-ready spec should not leave the accepted decision text pointing at the old contract while implementers start coding. | Add a "D164 amendment required before implementation merge" gate that updates the decision log/spec with the cycle-2 contract, not only after final code landing. The amendment should name the authority gate, phase-scoped claims, mixed v2/v3 verifier invariant, read-scope posture, and L3 transaction-prelude correction. | `C-D164-AMEND`: before the first behavior-changing implementation PR lands, `docs/decisions/decision-log.md` and `docs/reference/spec.md` no longer describe `BeforeAcquire SET LOCAL` as the attribution answer and no longer state leaked-credential or hardened-default claims more broadly than the cycle-2 constraints allow. |
| PX3-006 | medium | Future mutations cannot bypass the L1 authority model by falling outside the named protected tables. | The synthesis focuses on `audit_log`, `artifacts`, and `events`, but the product boundary is broader: daemon-owned PostgreSQL is live state and repository files are durable provenance. The existing schema has many mutating tables, and D164's reason is broader than the three headline surfaces: same-host direct PG access can bypass daemon capability/RPC policy. A future or existing durable write path outside the phased list could preserve the class of bug this RFC is meant to retire. | Define the L1 target as a mutation inventory, not only three table names. The RFC may explicitly scope Phase 0-2 to audit/artifact/event provenance, but it needs a guardrail that every new durable table is classified as direct-DML-allowed, read-only-to-runtime, or SECURITY-DEFINER-gated. | `C-MUTATION-INVENTORY`: `docs/reference/command-authority-matrix.md` or an adjacent authority table lists every daemon-owned table with its write-authority class. A test/guard fails new migrations that grant broad runtime DML on durable workflow tables without either a recorded product exception or a SECURITY DEFINER write function plus negative direct-DML test. |

## Product posture summary

Cycle 2 closes the original product-critical write spoofing hole in concept: raw
`striatumd_rw` function execution without daemon authority is no longer accepted,
and attribution GUCs are demoted below the trust boundary. The remaining product
risk is claim accounting. The plan is implementation-ready only if it says
exactly which properties are live at each phase, narrows "leaked DSN" to
mutation unless reads are redesigned, keeps #87 closure honest until L2 is
actually enforced, treats the v3 audit hash cutover as a product release gate,
and amends D164/spec text before implementation follows the revised contract.
