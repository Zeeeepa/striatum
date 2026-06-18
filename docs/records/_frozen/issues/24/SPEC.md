# GH #24 — supervise send: packet_id from claim-next not accepted; release --requeue lands jobs in 'blocked'

Source: https://github.com/halbritt/striatum/issues/24

## Summary

Two operator-discoverability + state-machine bugs surfaced while driving a fresh kayak-gen RFC-0057 stage-4 run on striatum 1.55.0:

1. **Operators can't discover the `packet_id` claim-next returns.** The `packet_id` field IS present in `claim-next` JSON output at `data.packet.packet_id` (deep in the structure, near the end), but it is not at any top-level position and is invisible to operators inspecting the first 2 KB of output. Operators reach for the obvious candidates — the `message_id` parsed from `commands.ack`, the `lease_id`, the `job_id` — and every one returns `not_found: could not find work packet for packet_id="..."` from `supervise send`. The bug filer's complete operator runbook (workflow.json validated, run prepared, branch confirmed, 7 sessions registered, 7 supervisors started, 7 packets claimed) stalled with all 7 lanes in `pipe_read` for the full lease window because there was no usable verb-side path to deliver the packet bytes.

2. **`release --requeue` parks repo_write jobs in `blocked`, not back to `queued`.** After `release --requeue --reason "..." --message-id <msg> --lease-id <lease>` returns `{"job_state":"blocked","status":"released"}`, subsequent `claim-next` (on the same session AND on freshly-registered sessions) returns `{"data":{"status":"no_work"}}`. The job is still in `striatum list jobs` with the same `workflow_job_id`, but it has no claimable path. Released 2 of 7 claims and ended up with `claimable_jobs: []` and `jobs: {"blocked": 9, "claimed": 5}` — run dead in the water; the remaining 5 supervisors sat in `claimed` until their leases expired.

Combined effect: an operator following the 1.55.0 skill-bundle runbook hits both bugs in sequence (first they can't deliver; then their attempts to undo the bad claim makes the job permanently unreachable). The bug filer cancelled the run and proceeded "cowboy-mode."

## Repro

```bash
striatum workflow validate <workflow.json> --allow-same-model-pairing
striatum run prepare --workflow <workflow.json>
striatum branch confirm --run-id <run> --branch <branch> --create
striatum run start --run-id <run>
SESS=$(striatum register-session --run-id <run> --role <r> --lane codex --fresh --json | jq -r .data.session_id)
striatum supervise start --session-id $SESS
# THE FAILURE — operator parses the message_id out of commands.ack because
# packet_id is not at any visible top-level position:
MSG=$(striatum claim-next --session-id $SESS --json | jq -r '.data.packet.commands.ack | capture("--message-id (?<m>msg_[0-9a-f]+)").m')
striatum supervise send --session-id $SESS --packet-id $MSG
# => not_found: could not find work packet for packet_id="msg_<...>"

# (For the second bug: release --requeue → job state = blocked → no claimable path.)
```

## Acceptance / Definition of done

A solution must satisfy each of:

1. **`packet_id` is operator-discoverable from `claim-next` output without spelunking.** Options (pick one): surface `packet_id` at `data.packet_id` (top-level alongside `status`); add a `data.next_steps.supervise_send` field with the literal command line; or unify so `supervise send` accepts `--session-id` alone and looks up the active packet for that session. The chosen path is documented in the `striatum-supervise` and `striatum-claim-loop` skill bundles.
2. **`supervise send` provides a useful error message.** When `--packet-id` is a `msg_*` / `lease_*` / `job_*` value, the error names the wrong-kind ID and points at the correct field path (`data.packet.packet_id` in the claim-next response, or whichever path lands).
3. **`release --requeue` either re-queues the job or fails loudly.** If repo_write jobs cannot be requeued (per D036), `release --requeue` must refuse the call with exit code != 0 and a message naming the recovery verb. Silently parking the job in `blocked` state with no recovery path is the bug.
4. **A worked end-to-end example exists in `striatum-supervise` / `striatum-claim-loop`.** Goes from "fresh session" to "supervised codex agent reads a packet on stdin" without any undocumented IDs. The example uses only the JSON paths from real CLI output.
5. **Tests cover both bugs.** (a) An integration test that follows the worked example from claim-next through supervise send to a written packet on the FIFO. (b) A test that release --requeue on a repo_write job either re-queues or refuses, never silently blocks.

## Suggested fixes (proposals; pick one per bug)

### Bug 1: packet_id discoverability

1. **Surface at top level**: `claim-next` returns `data.packet_id` alongside `data.status` and `data.packet`. Cheapest.
2. **next_steps field**: `claim-next` returns `data.next_steps = {"supervise_send": "striatum supervise send --session-id X --packet-id Y"}`. Mirrors the existing `commands` block in the packet.
3. **Implicit lookup**: `supervise send --session-id $SID` (no `--packet-id`) looks up the session's active claimed-but-undelivered packet and sends it. Requires only one in-flight packet per session.

(2) is the most discoverable; (3) is the most ergonomic; (1) is the smallest change.

### Bug 2: release --requeue blocks

1. **Honor --requeue for repo_write**: re-queue the job back to `ready`. Requires reasoning about whether work-in-progress on the repo (uncommitted writes by the prior lane) is safe to claim again.
2. **Refuse the call**: `release --requeue` on a repo_write job returns exit code 5 (or similar), names the constraint, and points at `recovery requeue-stale` / `recovery operator-publish` / whichever verb DOES recover.
3. **Add `release --abandon`**: explicit verb that parks the job in `blocked` with an operator rationale; `--requeue` is reserved for jobs that actually re-queue.

## Provenance

Filed by the kayak-gen RFC-0057 stage-4 operator on 2026-05-18. Full repro at https://github.com/halbritt/striatum/issues/24. I (the GH #22/#23 operator) hit the same `supervise send` discoverability problem on this very repo on 2026-05-19 — the workaround (`python3 -c "...json.load(sys.stdin)['data']['packet']['packet_id']"`) is what I'm using in the ad-hoc launch scripts in this issue's sibling workflows. The fix should make that python-spelunk unnecessary.
