---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
---

author: reviewer-unknown-model-001

# GH #24 Verification Review

Final verdict: `accept_with_findings`.

No license, attribution, telemetry, or compliance issue was found in
the reviewed artifact set. The fix is implemented entirely in the
in-tree Go daemon path and the source-of-truth skill templates; no
third-party code was added, no proprietary content was imported, no
license header was changed, and the commit carries the standard
`Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>` attribution
line. The author byline `implementer-unknown-model-001` on the
handoff matches the work-packet shape, and no transcripts or other
forbidden artifacts were written. The runtime change correctly
closes GH24 Bug 1, Bug 1b, and Bug 2 with file:line evidence below.
The acceptance set is met by code and unit tests; two follow-up
findings are recorded against the SCOPE deltas (Python PG handler
parity gap, and the absence of a single end-to-end test that goes
from `claim-next` through `supervise send` to FIFO bytes). Neither
gap blocks correctness on the production path running on this repo
(Go daemon `go/bin/striatumd`).

## Acceptance Verification

1. **DoD-1 (`packet_id` operator-discoverable from `claim-next`):
   accepted.** `HandleClaimNext` now returns the new top-level shape
   via the helper at `go/pkg/mutations/claim.go:170-179`
   (`claimNextResult` returns `status`, `packet_id`, `packet`,
   `next_steps.supervise_send`). The claimed return is wired through
   at `go/pkg/mutations/claim.go:164` (`return
   claimNextResult(sessionID, packetID, packet), nil`). The
   `next_steps.supervise_send` value is the literal command line
   `striatum supervise send --session-id <S> --packet-id <P>`
   (`go/pkg/mutations/claim.go:175-176`). The unit shape pin lives
   at `go/pkg/mutations/claim_test.go:8-24`
   (`TestClaimNextResultSurfacesPacketIDAndSuperviseSend`). Skill
   bundles surface the path: `data.packet_id` is named in
   `src/striatum/skills/templates/claude_code/claim-loop.md.tmpl:39`
   and `src/striatum/skills/templates/claude_code/supervise.md.tmpl:40`,
   regenerated into `.claude/skills/striatum-claim-loop/SKILL.md`
   and `.claude/skills/striatum-supervise/SKILL.md`, and the
   `docs/HOW_TO_AGENT.md:157-161` paragraph names the new field.

2. **DoD-2 (`supervise send` useful wrong-kind error): accepted.**
   The detector at `go/pkg/mutations/supervision_control.go:622-637`
   (`wrongKindPacketID`) covers `msg_`, `lease_`, `job_`, `sess_`,
   and `sup_` prefixes, returning the kind label (`message`,
   `lease`, `job`, `session`, `supervisor`). `loadWorkPacket`
   short-circuits at `go/pkg/mutations/supervision_control.go:601-603`
   before any DB read, returning `not_found` whose message names
   the wrong kind and points at `data.packet_id` /
   `data.packet.packet_id` as the correct field. The unit assertion
   pins the message at
   `go/pkg/mutations/supervision_control_test.go:122-147`
   (`TestSuperviseSendWrongKindPacketIDPointsAtClaimNextPacketID`).

3. **DoD-3 (`release --requeue` refuses repo_write loudly):
   accepted.** The refusal at `go/pkg/mutations/lifecycle.go:360-362`
   runs after the active-lease lookup at
   `go/pkg/mutations/lifecycle.go:357-359` and before any state
   mutation. The error code is `invalid_transition` and the message
   names `striatum recovery requeue-stale` as the recovery verb.
   The unit test `TestReleaseRequeueRefusesRepoWriteWithoutMutating`
   at `go/pkg/mutations/lifecycle_test.go:15-49` asserts (a) the
   error code, (b) the message contents (`release --requeue is not
   supported for repo_write jobs` AND `striatum recovery
   requeue-stale`), (c) `len(tx.execs) == 0` -- no mutations were
   issued, (d) the transaction did NOT commit, and (e) the
   transaction DID roll back. This satisfies the SPEC requirement
   that the call exits non-zero AND leaves state unchanged.

4. **DoD-4 (worked supervised example): accepted.** Both source
   templates were updated and regenerated:
   `src/striatum/skills/templates/claude_code/supervise.md.tmpl:31-50`
   shows the `register-session -> supervise start -> claim-next ->
   parse data.packet_id -> supervise send -> supervise stop` flow
   using only real JSON paths; `claim-loop.md.tmpl:36-44` adds the
   same `PACKET_ID=$(... jq -r .data.packet_id)` pattern and the
   `data.next_steps.supervise_send` copy-paste alternative.
   Generated outputs at `.claude/skills/striatum-supervise/SKILL.md`
   and `.claude/skills/striatum-claim-loop/SKILL.md` reflect the
   updates. The parity templates
   `src/striatum/skills/templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`
   and
   `src/striatum/skills/templates/generic/STRIATUM_AGENT_GUIDE.md.tmpl`
   carry the same flow. No `python3 -c "...json.load(...)..."`
   spelunking remains; no `--packet-json` example remains.

5. **DoD-5a (integration test from claim-next through supervise send
   to FIFO): `accept_with_findings`.** No single test exercises the
   full chain in one assertion. The Go unit tests cover the response
   shape (`TestClaimNextResultSurfacesPacketIDAndSuperviseSend`,
   `go/pkg/mutations/claim_test.go:8`) and the FIFO delivery path
   (the pre-existing `TestSuperviseSendDeliversPacketUnacknowledged`
   in `go/pkg/mutations/supervision_control_test.go`), but no test
   in this commit takes a `claim-next` response, parses
   `data.packet_id`, hands it to `supervise send`, and asserts the
   packet bytes arrive on the supervisor's FIFO. The shape and the
   delivery are pinned independently; an explicit end-to-end pin
   would be a stronger regression net. See finding F1 below.

6. **DoD-5b (release --requeue test): accepted.**
   `TestReleaseRequeueRefusesRepoWriteWithoutMutating` at
   `go/pkg/mutations/lifecycle_test.go:15-49` directly asserts the
   refusal AND the no-mutation property -- the silently-blocked
   regression cannot recur on this code path.

## Adversarial Probes

- **Worked example actually works (running daemon caveat):
  observation logged.** The example in the regenerated skill bundle
  uses `data.packet_id` and `data.next_steps.supervise_send`, both
  of which the Go source returns at
  `go/pkg/mutations/claim.go:170-179`. A live probe against the
  in-process daemon (`/home/halbritt/git/striatum/go/bin/striatumd`,
  pid 3150637) is NOT a clean test because the daemon binary on
  disk has mtime `2026-05-19 10:23:08Z` and the fix commit `755cb80`
  is dated `2026-05-19 10:34:17Z`; the running daemon predates the
  fix. The source-on-disk is correct (`go test ./pkg/mutations/...
  ./pkg/rpc/...` passes from `go/`); operators must rebuild and
  restart `striatumd` before the new shape and the new error
  message will surface on the wire. This is an operator-deployment
  issue, not a code defect.

- **Wrong-kind ID error: accepted (code), behaviorally pending
  rebuild.** A live probe `striatum supervise send --session-id
  <S> --packet-id msg_fake_probe --json` against the currently
  running (pre-fix) daemon returned `not_found: could not find work
  packet for packet_id="msg_fake_probe"` -- the old message.
  Reading the post-fix code at
  `go/pkg/mutations/supervision_control.go:600-603` and the unit
  test at `supervision_control_test.go:122-147`, the wire response
  after the rebuild names the id kind (`msg_123 is a message id,
  not a work packet id`) and points at both `data.packet_id` and
  `data.packet.packet_id`. The code path is correct; the live
  observation is consistent with a stale binary, not a missing
  fix.

- **`release --requeue` regression: accepted.** Code path inspected
  (`go/pkg/mutations/lifecycle.go:360-362`); refusal precedes every
  `UPDATE` statement; unit test asserts zero `tx.Exec` calls. The
  silent-park-in-blocked dead-end is closed: the only post-fix
  outcomes for a `release --requeue` on a repo_write job are (1)
  active-lease lookup failure before the refusal (`return err`),
  which leaves state unchanged because no mutation precedes the
  lookup, or (2) the refusal itself with rollback.

- **Lease/message IDs preserved: accepted.** None of `HandleAckWork`
  (`go/pkg/mutations/lifecycle.go`), `HandleHeartbeat`, or
  `HandleCompleteWork` were touched in this commit. Verified by the
  diffstat (only `claim.go`, `supervision_control.go`, and
  `lifecycle.go`'s `HandleReleaseWork` were modified) and by the
  active session: `ack`, `heartbeat`, and `complete` continue to
  use `--message-id`, `--lease-id`, and `--job-id` exactly as
  before, including the message-id parsed out of
  `commands.ack` in the packet block. No behavioral drift on the
  preserved IDs.

## Test / Verification Assessment

`go test ./pkg/mutations/... ./pkg/rpc/...` passes from `go/` (cache
hit on a clean tree). The three new unit tests run in well under
ten milliseconds and exercise (a) the claim_next response shape,
(b) the wrong-kind supervise-send error, and (c) the repo-write
requeue refusal with explicit no-mutation + rollback assertions.
The pre-existing `TestSuperviseSendDeliversPacketUnacknowledged`
continues to cover the FIFO delivery path. The handoff documents
that `striatum skills install --profile all --scope project
--dry-run --json` was used to confirm template -> generated
artifact rendering without writing forbidden directories. No
Python-side unit tests were added; see finding F2.

## Findings

### F1 -- end-to-end claim-next -> supervise send -> FIFO pin missing (severity: low)

**Gap.** SCOPE acceptance bullet DoD-5a specifically calls for an
integration-style test that "follows the worked example" through
`claim-next` -> parse `data.packet_id` -> `supervise send` and
asserts the packet bytes arrive on the supervisor's FIFO. The
commit pins shape and delivery independently
(`go/pkg/mutations/claim_test.go:8` and the pre-existing
`TestSuperviseSendDeliversPacketUnacknowledged`) but does not
chain them.

**Why this matters.** A future refactor could rename
`data.packet_id` to `data.packet.packet_id` (or vice versa) in one
file and leave the FIFO delivery untouched; the unit tests would
still pass, but the worked example operators copy out of
`.claude/skills/striatum-supervise/SKILL.md` would silently break.
A chained test is the cheapest guardrail against the exact bug
class GH #24 surfaced.

**Remediation.** Add one Go test (or Python integration test
under `tests/integration/`) that calls `HandleClaimNext` (or the
CLI), reads `result["packet_id"]`, invokes `HandleSuperviseSend`
with that value, and asserts the FIFO receives the packet bytes.
Reuse the existing `superviseControlFakeRunner` plumbing in
`go/pkg/mutations/supervision_control_test.go` for fixture
fidelity. Estimate: one screen of code; under one hour.

### F2 -- Python PG claim_next handler still returns the old shape (severity: low)

**Gap.** SCOPE.md "Files in scope" lists
`src/striatum/daemon_pg/handlers/workflow_loop/claim_next.py` as
**EDIT** with the same `packet_id` + `next_steps.supervise_send`
additions. The handoff explicitly skipped the file citing
`write_scope.allowed_paths`. The current file at
`src/striatum/daemon_pg/handlers/workflow_loop/claim_next.py:153`
still returns `{"status": "claimed", "packet": packet}` without
the new top-level fields, and `register_pg_handler("work.claim_next",
"claim_next")` at line 21 is still active.

**Why this matters.** The production daemon on this repo is the
Go binary (verified: `go/bin/striatumd`, pid 3150637), so the
on-the-wire behavior on this repo is correct as soon as operators
rebuild. However:
- Test fixtures and any Python-mode `striatum serve` invocation
  (the in-tree Python daemon, e.g. the `pytest` harness at
  `pid 2175175 .../striatum.cli ... serve --token secret123`)
  will still return the old shape and break the worked example.
- An operator who follows the `striatum-supervise` skill with the
  Python-mode daemon hits the GH #24 Bug 1 symptom again.
- Parity drift between Go and Python handlers raises future audit
  cost.

**Why it is `low`, not `medium`.** The Go daemon is the documented
production path on this repo (`docs/POSTGRES_TRANSITION.md` and
`AGENTS.md` "Product Boundary" cite the daemon-owned PostgreSQL
instance via the Go binary). The Python handler is reachable in
test harnesses and in operator-side `serve` invocations, but it is
not the default production path. Risk is bounded.

**Remediation.** Land a follow-up commit that mirrors the
Go-side change in the Python handler: in
`src/striatum/daemon_pg/handlers/workflow_loop/claim_next.py:153`
return `{"status": "claimed", "packet_id": packet_id, "packet":
packet, "next_steps": {"supervise_send": f"striatum supervise
send --session-id {session_id} --packet-id {packet_id}"}}`. Add
the matching unit test under `tests/daemon_pg/test_claim_next.py`
(or the closest existing test file). The same packet
`write_scope` constraint that blocked this in-packet does not
apply to a fresh work packet scoped to the Python path.

### F3 -- operator deployment caveat (severity: info)

**Gap.** Not a code defect; an operator-deployment observation.
The running `go/bin/striatumd` binary on this checkout (mtime
`2026-05-19 10:23:08Z`) predates the fix commit `755cb80`
(`2026-05-19 10:34:17Z`). Until operators run `make
striatumd` (or whichever build target rebuilds the Go binary)
and restart the daemon, the on-the-wire behavior of `claim-next`
and `supervise send` will still match the pre-fix shape/messages.

**Remediation.** Surface the rebuild step in the GH #24 close
note or in `docs/POSTGRES_TRANSITION.md`. Optionally, add a
`daemon doctor` block that compares the running daemon's
`commit_sha` against the source-tree head and warns operators
when the daemon is stale. Out of scope for the GH #24 fix
itself.

## Notes for the Operator

- After landing this work, operators on a hosted daemon should
  rebuild `striatumd` and restart the daemon process before
  trusting the new `claim-next` shape or the new
  `supervise send` error message.
- The skill-bundle update is generation-driven; running
  `striatum skills install` regenerates
  `.claude/skills/striatum-*/SKILL.md` from the new templates.
  No hand-edits to the generated files are required.
- F2 (Python PG handler) is a parity follow-up, not a bug in
  this commit. If a parity sprint is queued, scope it as a
  separate dogfood with `write_scope.allowed_paths` that
  includes `src/striatum/daemon_pg/handlers/workflow_loop/`
  and `tests/daemon_pg/`.
