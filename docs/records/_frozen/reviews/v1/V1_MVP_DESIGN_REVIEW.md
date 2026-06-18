# V1 MVP Design Review

Date: 2026-05-06
Reviewer: parent Codex coordinator
Verdict: accept_with_findings

## Findings

### P1: Gemini scope reduction conflicts with P001 minimum commands

Affected artifact: `docs/design/V1_MVP_DESIGN_INPUT_gemini.md`

Gemini recommends dropping `ack`, `heartbeat`, `verdict`, lease mechanics, and
parallel review behavior from V1. P001 explicitly requires these behaviors.

Consequence: adopting the recommendation literally would make the MVP fail the
prompt's acceptance criteria.

Proposed fix: accept Gemini's caution with modification. Keep required
commands, but implement lazy lease expiry, no background daemon, and
review-only declared parallelism.

### P1: Branch confirmation must gate claimability

Affected artifact: synthesized design

It is not enough to require confirmation before branch creation. Jobs must not
be claimable until confirmed branch state is recorded and the run starts.

Consequence: agents could mutate artifacts on `master` before branch setup.

Proposed fix: `run prepare` leaves the run in `needs_branch_confirmation`;
`claim-next` returns a branch-confirmation error until `branch confirm` and
`run start` complete.

### P1: Artifact publishing must be coupled to completion

Affected artifact: synthesized design

If `complete` does not verify required artifact records, agents can mark jobs
done after writing nothing, writing outside scope, or forgetting to publish.

Consequence: downstream jobs may consume missing or untracked outputs.

Proposed fix: `complete` and `verdict` must verify required artifacts before
terminal transition.

### P2: Stale lease behavior must distinguish review-only and repo-write work

Affected artifact: synthesized design

Auto-requeueing all expired leases is unsafe when a session may have modified
the worktree.

Consequence: two agents can edit the same files after a silent crash.

Proposed fix: expired review-only/no-write leases may requeue; expired
repo-write leases become `stale_lease` or blocked until coordinator/human
inspection.

### P2: Adapter boundary must not become provider protocol logic

Affected artifact: synthesized design

The product supports Claude, Codex, and Gemini lanes, but core scheduling must
not infer behavior from provider names or parse terminal text.

Consequence: model portability erodes and provider upgrades become product
changes.

Proposed fix: lanes are explicit command arrays plus capabilities. The core
matches role/lane/capability metadata and treats stdout as non-authoritative.

### P2: Direct SQLite writes cannot be technically prevented by policy alone

Affected artifact: synthesized design

The design says agents must not write SQLite directly. A local file database
cannot stop a user or process with file access from doing so.

Consequence: corruption or untracked state could bypass invariants.

Proposed fix: enforce constraints/triggers, keep mutations in CLI code, and
make `doctor` detect common invalid states.

### P3: Coordinator command allowlist can wait for coordinator chat

Affected artifact: `docs/design/V1_MVP_DESIGN_INPUT_claude.md`

Claude recommends server-side command allowlists for coordinator sessions. This
is directionally correct but not needed for the first state-machine MVP if
coordinator chat is not implemented yet.

Consequence: overbuilding coordinator chat policy could delay the required
queue/work/artifact MVP.

Proposed fix: record the allowlist as a spec requirement for coordinator
skills and defer rich chat enforcement until coordinator chat is implemented.

## Open Questions

- Should `run start` and `branch confirm` be collapsed for simple local
  workflows after MVP validation?
- Should process/tmux launch be implemented immediately after the state MVP, or
  after RFC-ledger fixture validation?

## Checks Run

- Read all three design input artifacts.
- Checked the design against P001 required behavior and accepted decisions
  D001-D033.

