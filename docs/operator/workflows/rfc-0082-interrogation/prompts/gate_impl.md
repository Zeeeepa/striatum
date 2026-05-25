# Gate — Implement RFC 0082 interrogation sessions (impl + tests)

Implement RFC 0082 in full. Read `docs/rfcs/0082-interrogation-sessions.md`
first — it is the contract, and its **Required Tests** section is the
acceptance bar. The implementation MUST match the intention: a reviewer
iteratively interrogates a builder's PRESERVED context and gets context-aware
answers. Also read `go/pkg/mutations/claim.go` (`HandleAwaitPacket`),
`go/pkg/mutations/work_send_message.go`, `go/pkg/agentloop/`,
`go/pkg/reads/trajectory.go`, `contracts/daemon_methods.json`, and
`go/pkg/pgtest/` (the live-PG harness from RFC 0080).

## Build

1. **Migration** (new file under `go/pkg/db/sql/`, bump `LatestDaemonDBVersion`):
   create `striatumd.interrogations` (lifecycle + correlation) and ensure
   `queue_messages` already carries `target_session_id` (it does) +
   `interrogation_id` via `payload_json`. CRITICAL ownership rule (RFC 0079 §5 /
   the RFC 0081 incident): the runtime role `striatumd_rw` does NOT own existing
   tables, so the migration MUST NOT `ALTER` owner tables and MUST NOT create
   foreign keys referencing owner-held tables (`repositories`, `runs`). Create a
   plain new table the runtime role can apply, enforce referential integrity in
   Go, and `GRANT` DML to the runtime role. Required Test 9 guards this.
2. **Daemon methods**: `interrogation.open|ask|answer|close|list|show` in
   `go/pkg/mutations` + `go/pkg/reads`; register in
   `contracts/daemon_methods.json` and regenerate (`cd go && go generate ./...`,
   keep the round-trip byte-identical); append `events` rows; capability-gate
   (interrogator may open/ask; only target session may answer; daemon-written).
3. **Delivery**: extend `HandleAwaitPacket` to return the typed envelope
   (`work_packet` | `interrogation_question` | `none`), delivering questions
   addressed to the awaiting session and preferring a pending question over new
   work. Do NOT regress plain work delivery.
4. **Context window**: add the `awaiting_interrogation` session phase + an
   `interrogable` job flag; after `work.complete` an interrogable job's session
   re-enters `await_packet` instead of closing, until `interrogation.close` or an
   idle timeout; relax `fresh_session_required` for interrogable builders.
5. **CLI + MCP**: `striatum interrogation {open,ask,answer,close,list,show}` and
   the matching MCP tools; update the skill bundle docs.
6. **Trajectory**: surface interrogation turns in the RFC 0081 `dialogue`
   profile (curated; D028 — never provider stdout/stderr).
7. **Docs**: `docs/WORKFLOW_TYPES.md` review-by-interrogation + daemon runbook note.

## Tests (ALL of RFC 0082 §"Required Tests" — non-negotiable)

Write them with `go/pkg/pgtest` for live-PG coverage. The end-to-end intention
test (RFC 0082 test 7) is the bar: a builder completes an interrogable job,
enters `awaiting_interrogation`, a reviewer opens+asks, the builder answers from
PRESERVED context (assert the answer reflects a builder-only fact seeded into the
build packet and never republished), close, then session closes; `trajectory
export --profile dialogue` reproduces the thread. Plus tests 1-6, 8, 9.

## Validate
```bash
cd go && go generate ./... && git diff --exit-code   # generator round-trip clean
cd go && go build ./... && go vet ./... && go test ./... && go test -race ./...
```
Stay strictly in `write_scope.allowed_paths`. Do not write to `src/striatum`.
Do not restart the operator's daemon.

## Artifact
Publish `docs/operator/artifacts/rfc-0082-interrogation/impl/SUMMARY.md`
(`synthesis`): methods/table added, the await_packet envelope, the context
window mechanism, every Required Test with its file:func, and validation output
(including the e2e intention test result). Use your packet byline.
