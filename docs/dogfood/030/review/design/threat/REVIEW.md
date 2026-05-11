---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "high"
tags: ["threat-model", "provenance", "containment", "attestation"]
---

# Threat-Model Review: RFC 0026 and RFC 0027 Design Synthesis

author: reviewer-claude-opus-002
date: 2026-05-11
status: final
target: docs/dogfood/030/DESIGN_SYNTHESIS.md
verdict: accept_with_findings

## Summary

The synthesis is structurally sound for a threat-model posture: it names the
three provenance modes, quarantines source-byte provenance behind containment,
keeps the local-first boundary, refuses silent sealed-mode degradation, and
explicitly disclaims model-token authorship. The core invariant on
DESIGN_SYNTHESIS.md:27 is the right framing: *"byline honesty is not
source-byte provenance; patch digests are not sealed provenance; signed
receipts are not tamper-resistant while the signing key and protected source
remain writable by the same operator authority."*

I am voting `accept_with_findings` rather than `needs_revision` because (a) the
companion security review already issued `needs_revision` covering
`starting`-supervisor weakness, sealed-mode path coverage, receipt
authority, and operator-label confusability; (b) the synthesis explicitly
labels Phases 1-4 as advisory until Phase 5 ships containment; and (c) the
threats below are tightening of contracts the implementer must observe inside
the existing phased plan, not new scope-blocking gaps.

The findings below enumerate trust boundaries the synthesis introduces or
reshapes that I did not see acknowledged with concrete mitigations. The
implementer must address THR-001, THR-002, and THR-004 before the
corresponding phase ships, even if the security reviewer's `needs_revision`
revision pass does not pick them up directly.

## Trust Boundary Map

The synthesis introduces or refines these trust boundaries. "Mitigated" means
the synthesis names a concrete defense; "acknowledged" means out-of-scope by
explicit non-goal; "gap" means I did not find acknowledgment.

| Boundary | Status | Notes |
|---|---|---|
| Operator -> `.striatum/state.sqlite3` direct write | acknowledged | Local-first non-goal (RFC 0026 non-goals; synthesis L34) |
| Operator -> source code (advisory mode) | acknowledged | RFC 0027 problem statement; mitigated only by Phase 5 |
| Operator -> `lanes.<>.command` (workflow author) | acknowledged | RFC 0026 non-goals; ghost-write under real supervisor accepted |
| Operator -> `os.kill(pid, 0)` PID reuse / wrong process | **gap** | THR-001 below |
| Operator -> ghost-writing in lane scratch then `patch capture` (Phase 3-4) | partial | "advisory" label asserted; machine-readable contract not specified — THR-002 |
| Operator -> mutating live `workflow.json` mid-run | partial | Snapshot semantics assumed but not stated for sealed-mode lane.command — THR-003 |
| Operator -> cryptographically signed receipt before key isolation (Phase 4) | partial | "authority: advisory" label asserted; signing-prohibition contract missing — THR-004 |
| Operator -> setting `require_attested_lane: true` on non-review jobs in Phase 1 | gap | Phase 1 deferral leaves validator behavior undefined — THR-005 |
| Operator -> subverting cross-platform support probe (LD_PRELOAD, env, fake mount) | gap | Probe shape undefined; "explicit unsupported" depends on a not-yet-written function — THR-006 |
| Operator -> downgrade after Phase 3 schema migration | acknowledged | RFC 0006 forward-only; flagged for the record — THR-007 |
| Operator -> Striatum-created sealed signed commit pushed to remote | acknowledged | Striatum does not push; downstream tooling concern noted, non-blocking |
| Verdict-over-digest-A satisfying digest-B apply | mitigated | Phase 3 testable milestone explicit |
| Sealed-run start on writable protected paths | mitigated | Phase 5 refusal called out |

## Findings

### THR-001: Liveness probe must bind to supervisor identity, not pid

The synthesis grounds lane attestation in
`os.kill(pid, 0)` plus a `process_supervisors` row in `starting` or
`attached` (DESIGN_SYNTHESIS.md:19, 47-49). The codex security review SEC-001
already calls for excluding `starting`. The threat-model concern is more
specific: even with `attached` plus a successful `os.kill(pid, 0)`, the
runner has no evidence that the pid still names the process the supervisor
spawned. Linux pids wrap (`/proc/sys/kernel/pid_max` defaults to 4194304 but
many distributions still ship with 32768). On long-running operator hosts a
supervised lane can exit, the pid can be reused by an unrelated process
within the same UID, and the next attestation lookup will succeed against
the wrong process.

This matters because RFC 0026's stated guarantee is "a process from this
lane's command is alive on the recorded pid for this session" (RFC 0026
non-goals, lines 124-129). PID reuse falsifies that guarantee with no
adversarial action — a good-faith long-lived operator session is enough.

Required mitigation, to land in Phase 1:

- Capture a stable identity at `supervise start` time. The cheap option on
  Linux is reading `/proc/<pid>/stat` field 22 (`starttime`) at fork time
  and re-reading at every liveness check. The strong option is `pidfd_open`
  with `pidfd_send_signal(SIGCONT, 0)` for a true identity-bound liveness
  probe.
- Store the identity token (`starttime` jiffies, or pidfd open-and-cache)
  alongside the existing `process_supervisors` row.
- Treat identity-mismatch the same as a dead process: downgrade to
  `author: operator`, refuse `require_attested_lane`, mark the supervisor
  `lost`.
- Add a negative test: spawn a supervised process, kill it, exhaust pids
  until the recorded pid is reused by an unrelated subprocess in the same
  UID, attempt a publish, assert downgrade to `author: operator`.

Without this, RFC 0026's byline honesty is silently breakable on Linux by an
attacker who never leaves user-mode and never edits SQLite.

### THR-002: Phase 3 patch capture needs a machine-readable advisory label, not only export wording

The synthesis says advisory patch capture is acceptable before containment
"but every surface must label it advisory" (DESIGN_SYNTHESIS.md:24, 63-66,
135-139, 222). The threat is that "labeled advisory" today is implemented
in long-form export language ("does not prove" sentences), which is
durable but not machine-readable. Phase 3 introduces `patch_artifacts`
with `patch_sha256`, `paths_json`, `blob_hashes_json`, `hunk_hashes_json`,
and producer-binding columns. A downstream tool — particularly an LLM
coordinator chained to `evidence export --json` — sees a patch object with
strong-looking metadata and a producer attestation chain. It will not
parse the surrounding prose disclaimer.

In Phases 3-4 a `patch_artifacts` row binds a `producer_supervisor_id` to
the captured bytes, but the captured bytes are whatever happens to be in
the lane scratch workspace at capture time. Without containment, the
operator wrote those bytes; the supervisor merely attests "a process for
the lane was alive." This is *exactly* the false-convergence failure
RFC 0026/0027 are meant to close, just at a different layer.

Required mitigation, to land in Phase 3:

- Add a top-level field on `patch_artifacts` (and on its rendered JSON
  shape in evidence export, status JSON, and the web UI) such as
  `provenance_authority` with a closed enum `{advisory, apply_gate_only,
  sealed}`. Phase 3 captures must always set this to `advisory` until
  Phase 5 containment ships.
- Mirror the same field on `verdict_review_targets` and on `apply_receipts`
  when those land. A receipt or apply over an advisory patch must
  propagate `advisory` (the result is no stronger than the weakest input).
- Make the `provenance_authority` field a required argument to any
  consumer-facing renderer of patch artifacts. Refuse to render if it is
  missing. This is cheap to enforce in the artifact aggregate.
- Add a negative test: capture a patch under advisory authority, render
  evidence export JSON, assert the field is present at every level the
  patch is referenced.

This is the implementation contract behind the synthesis line "every
surface must label it advisory." Without making the label structural and
mandatory, the staging plan loses its honesty floor.

### THR-003: Lane-command source must be the snapshot in sealed mode

Phase 5 promises operator write denial against protected source paths
(DESIGN_SYNTHESIS.md:69-74, 204-209). The synthesis does not state where
`lanes.<>.command` is read from during a sealed run. If the runner re-reads
the live `workflow.json` at `supervise start` time, an operator can edit
`lanes.codex.command` mid-run between the workflow validation pass and the
next supervisor spawn — swap a real lane for `bash -lc 'sleep infinity'` —
and obtain attested bylines for ghost-written work. The protected-path
policy does not help here unless `workflow.json` is also under
`protected_paths`, which it cannot be because the operator owns the
control workspace.

Required mitigation, to land in Phase 2 alongside `provenance_mode`
surfacing:

- State explicitly in the synthesis (and enforce in implementation) that in
  `sealed_patch` mode all `lanes` and workflow validator inputs come from
  the run's snapshotted workflow blob, not from the live file on disk.
- `doctor` should warn if the live workflow.json hash differs from the
  run's snapshot during an active sealed run.
- Add a negative test: start a sealed run, mutate the live workflow.json
  to change `lanes.codex.command`, supervise start a codex session,
  assert the snapshot command is the one that ran.

This is mostly a clarification of what I assume the runner already does
for the workflow snapshot, but it needs to be load-bearing for sealed-mode
acceptance.

### THR-004: No cryptographic signatures on Phase-4 receipts

The synthesis says apply and receipt output "must say `authority: advisory`
or `authority: apply_gate_only`; it must not call the result sealed"
(DESIGN_SYNTHESIS.md:65-67) and defers `keys init|rotate|export-public`
to "when receipt signing lands" (line 113). The order is left ambiguous:
nothing in the staging plan forbids Phase 4 from shipping signed
receipts before Phase 5 containment.

The threat is the same one SEC-003 names from a different angle. A
cryptographic signature on a receipt is independent of the receipt body's
`authority: advisory` field. Receipt-verifier tooling — ours or
downstream consumers like CI gates, audit pipelines, or LLM-driven
release tooling — typically gates on signature validity, not on free-text
metadata. A signed advisory receipt is the worst overclaim shape because
the cryptographic proof is real even when the trust root is operator-
writable.

Required mitigation, to land before Phase 4:

- Add an explicit synthesis invariant: receipts in `provenance_authority:
  advisory` or `apply_gate_only` MUST NOT carry a cryptographic
  signature. The receipt format may include a `signature` field whose
  value is `null` in pre-Phase-5 receipts.
- Alternatively (less preferred), require all signed receipts to include
  a `key_authority` field with values `{operator_writable,
  runner_isolated}` and require verifier output to refuse a positive
  "valid" result for `operator_writable`-keyed receipts.
- `striatum keys init` must not function while the run's
  `provenance_mode != "sealed_patch"` AND a Phase-5 containment probe
  has not succeeded. Without this, the keys command becomes a foot-gun
  the operator can use to start signing advisory bytes today.
- Add a negative test: in advisory mode, attempt `striatum keys init` and
  assert refusal; in advisory mode, render a receipt and assert the
  signature field is `null` (or absent).

Receipts before Phase 5 are evidence-quality bundles, not provenance
proofs. The implementation contract should make that mechanically true,
not only documentary.

### THR-005: `require_attested_lane` validator behavior on non-review jobs in Phase 1

The synthesis says `require_attested_lane` is a Phase 1 review-job gate,
with non-review use "deferred until patch capture has producer-side
semantics" (DESIGN_SYNTHESIS.md:21). The threat is that Phase 1's workflow
validator does not say what happens when an existing or future workflow
sets `require_attested_lane: true` on a build, synthesis, or generic job.

Three behaviors are possible: (a) the validator accepts and silently
ignores the flag; (b) the validator accepts and the runner enforces only
on review jobs; (c) the validator refuses. (a) and (b) both look correct
to the workflow author and can hide the same false-provenance failure
mode RFC 0026 closes for reviews — a workflow author who believes their
build job is gated for attestation will be wrong.

Required mitigation, to land in Phase 1:

- Validator must refuse `require_attested_lane: true` on non-review jobs
  until producer-side semantics ship. Error message must name the
  expected job type.
- Add the rejection to the schema/migration test matrix.

This avoids workflows latching onto a defense that does not exist yet.

### THR-006: Cross-platform support probe must be defined as a stub in Phase 2

The synthesis says `sealed_patch` "must validate structurally but `run
start` must refuse it unless the authority probe reports supported
containment" (DESIGN_SYNTHESIS.md:55). The probe shape itself is deferred
to Phase 5 ("containment authority metadata only when the first mechanism
is selected", line 96-97). The threat is that Phase 2 ships
`provenance_mode` validation while the gate it depends on is undefined; a
naive implementation may pick "platform.system() == 'Linux'" as a stand-in
and report supported on any Linux host, including hosts without bwrap,
without ACLs, and inside operator-controlled WSL.

Required mitigation, to land in Phase 2:

- Define the probe as a stub that reports `supported: false,
  reason: "no containment mechanism shipped"` on every platform until
  Phase 5 selects a mechanism. The stub is honest; a heuristic is not.
- WSL detection: prefer reading `/proc/sys/kernel/osrelease` for
  `microsoft` substring and reporting `unsupported` until WSL containment
  is explicitly tested.
- Add a negative test: on every CI runner platform, assert
  `striatum run start --provenance-mode sealed_patch` fails with the
  expected unsupported message.

The stub is cheap, prevents accidental "looks supported on Linux" claims,
and gives Phase 5 a clear seam to implement the real probe behind.

### THR-007: Phase 3 schema migration is the last forward-only step before sealed (informational)

Phase 3 adds `patch_artifacts`, `verdict_review_targets`, and the `patch`
artifact kind (DESIGN_SYNTHESIS.md:88-92). RFC 0006 migrations are
forward-only by design, so a downgrade is not a supported recovery path.
Threat actors are not the concern; the concern is that any operator who
runs an older Striatum binary against a `.striatum/state.sqlite3` that
already saw Phase 3 will encounter unparseable artifact rows.

Required mitigation:

- Document the no-downgrade boundary in the synthesis's "Compatibility
  And Upgrade Risks" section.
- Encourage `evidence export` before any major version change so the
  durable provenance survives a state-store rebuild.

This is informational and does not block any phase.

## Non-Blocking Notes

- The `verdict_review_targets` recommendation as a side table is the right
  call. Recommend adding `UNIQUE (verdict_id, reviewed_artifact_id)` and a
  comment that one verdict may bind multiple reviewed artifacts only when
  policy explicitly opts in; default-deny is safer.
- Striatum's "no push, no merge, no rebase" promise (DESIGN_SYNTHESIS.md:74,
  221) survives even with sealed-mode local signed commits because the
  push surface is not a Striatum verb. Worth restating in the SPEC carve-
  out so downstream tools do not interpret a Striatum-signed commit on
  `origin/main` as proof of in-tool apply on that machine.
- The Linux-first staging stance is the right threat-model posture. macOS
  and Windows do not have a comparable user-vs-user write-denial story
  without selecting a specific containment technology and shipping a real
  test for it.
- The synthesis's `attested_at_record_time` audit-only flag
  (DESIGN_SYNTHESIS.md:131-132) is OK if it is never rendered in
  human-facing surfaces. Recommend storing it in `verdicts` only, never
  exposing it through `status`, `evidence export`, or the web UI, to
  avoid the same downstream-overclaim risk THR-002 calls out.
- Workflow JSON edits to `lanes.<>.env` between supervised spawns are the
  same shape as THR-003. The snapshot fix covers both.

## Verdict

`accept_with_findings`. The synthesis is implementation-ready for the
threat-model posture. The findings above are tightening contracts the
implementer must satisfy inside the existing phased plan; they do not
require another synthesis revision pass beyond the one the security
reviewer's `needs_revision` is already triggering. THR-001 (PID
identity), THR-002 (machine-readable advisory label), and THR-004 (no
crypto signing pre-containment) should be promoted into the
implementer's acceptance criteria for Phases 1, 3, and 4 respectively.
