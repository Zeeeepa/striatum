---
schema_version: "striatum.harness_improvement_proposal.v1"
artifact_kind: "harness_improvement_proposal"
target: "defaults"
expected_benefit: "Make the default supervised lane actually able to execute work packets, so dogfood-001 (and any other Striatum-on-Striatum run) can drive draft -> review -> apply end to end without a human stepping in to manually run the agent CLI."
risk: "Lane command shape changes affect any operator that already wired a Striatum workflow against `claude --model opus -p`. RFCs around supervised stdin format may require packet schema or supervisor adapter changes."
rollback: "Revert the lane command in `docs/dogfood/001/workflow.json` and continue running dogfood-001 manually (operator drives the work) until the supervised lane is fixed."
---

# HARNESS-001 — Default `claude_code` lane (`claude --model opus -p`) cannot execute work packets

Status: proposed
Run: dogfood-001
Reporter: author-claude-opus-001
Surface: supervise

## Observed friction

The dogfood-001 workflow (`docs/dogfood/001/workflow.json`) configures the
`claude_code` lane with:

```
"command": ["claude", "--model", "opus", "-p"]
```

The runbook prescribes `striatum supervise start --session-id $AUTHOR` followed
by `striatum claim-next --session-id $AUTHOR`. We did exactly that:

1. `supervise start` reported `pid=1860356`, `state=attached`,
   `stdin_pipe_path=.striatum/scratch/sup_.../stdin.pipe`.
2. `claim-next` reported
   `supervisor_delivery: {bytes_written: 3720, ...}` and the job moved to
   `claimed`.
3. **14 seconds later** the supervised process was already gone.
   `striatum supervise status --json` reported
   `state: "lost"`, `stop_reason: "pid is gone observed by status"`,
   `liveness: "gone"`.

`striatum why <run_id>` confirms the sequence: `supervisor.starting` ->
`supervisor.started` -> `queue.claimed` -> `supervisor.packet_delivered`
(3720 bytes) -> `supervisor.lost` (PID gone). No artifact was published, no
`heartbeat`/`ack`/`complete` was called, and the lease
`lease_7e8c03e41d734c88ae0a81246f5de112` is still held against a dead
process. Stdout/stderr are routed to `DEVNULL` per RFC 0009, so there is no
visible error.

What we expected: the supervised agent reads the JSON packet from its stdin
pipe, follows `task_prompt.path`, makes the change, calls
`striatum publish-artifact` and `striatum complete`, and the runner advances
to the review job.

What actually happened: `claude -p` (Claude Code in print mode) is a
single-prompt, single-response CLI. It reads stdin as one prompt body,
prints a single reply to stdout (which is `DEVNULL` in supervised mode), and
exits. It is not aware of the Striatum work-packet protocol; it has no tool
permissions in this invocation; it cannot call back into `striatum` because
it never saw an instruction to. The packet JSON is just delivered as raw
text and consumed as if it were a user prompt.

## Supporting runner evidence

- run_id: `run_a04880660517480a95438fcc0368d2e0`
- job_id: `job_run_a04880660517480a95438fcc0368d2e0_draft_change`
- packet_id: `wp_3c1fc153e2cd40f6ab7ef1447ae2a5e7`
- supervisor_id: `sup_917d33bf5fe649688c862d0f89f81591`
- relevant event types from `striatum why <id>`:
  `supervisor.starting`, `supervisor.started`, `queue.claimed`,
  `supervisor.packet_delivered`, `supervisor.lost`

## Proposed change

Pick at least one of the following; ideally all three:

1. **Document the lane contract.** The `lanes.<id>.command` value must point
   at an agent invocation that (a) stays alive across packets, (b) reads
   newline-delimited JSON packets from stdin, and (c) knows the Striatum
   protocol (calls `ack`/`heartbeat`/`publish-artifact`/`complete`). Today
   that contract is implicit; the dogfood scaffold ships with an invocation
   that violates all three. Add a `docs/SPEC.md` section "Supervised lane
   command contract" and link it from `workflow.json` schema docs.

2. **Ship a working default for `claude_code`.** Replace
   `["claude", "--model", "opus", "-p"]` with an invocation that:
   - launches a long-running session (not `-p` print mode),
   - is started with the `striatum-dogfood-001` skill (or equivalent
     project skill) loaded so the agent recognizes the packet and protocol,
   - has the permissions it needs to edit files and run `striatum`
     subcommands (e.g. `--dangerously-skip-permissions` in a sandboxed
     workflow, or a permissions allowlist).
   The shape that's most likely to work today is something like
   `claude --dangerously-skip-permissions --model opus` invoked with the
   skill on its `~/.claude/skills` path, not `-p`.

3. **Fast-fail when the supervisor dies before completing the packet.**
   Today, `supervisor.lost` is recorded but the lease stays held until
   expiry and `claim-next` reports success. The runner should either
   automatically requeue (for review jobs only — repo_write needs operator
   review per RFC 0008) or surface a `next_action` like "supervisor died
   before publish/complete; investigate or run `recovery requeue-stale`".
   Right now an operator has to read `striatum why` and notice the
   `supervisor.lost` event themselves.

## Risk

- Changing the lane command in dogfood-001 changes who/what runs the work,
  but the workflow itself is unchanged.
- Adding a `next_action` for orphaned leases changes operator UX; existing
  scripts that grep `next_actions` may need updating.
- Documenting the lane contract is pure addition.

## Rollback

- Lane command: revert `workflow.json`.
- `next_action` change: feature-flag or revert; the existing recovery path
  still works.
- Doc addition: revert.

## Notes

This is the friction predicted by `RUNBOOK.md` section "Things I expect to
break" item 1 ("Supervisor pipe vs TTY-required CLIs"). The variant we hit
is not TTY-required — `claude -p` accepts the pipe just fine — it's that
print mode fundamentally exits after one prompt and is the wrong shape for
a supervised lane. The conceptual gap is the same: the supervised-lane
contract has only ever been exercised by tests, and no real agent CLI in
the dogfood scaffold actually satisfies it.

Operator path forward for this run: stop the dead supervisor, requeue or
abandon the lease, drive the draft/review/apply work as the operator
(myself, the Claude Code instance you are talking to) instead of via a
supervised CLI. That's a manual workaround; HARNESS-001 captures the
underlying gap.
