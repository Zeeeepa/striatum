---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs:
  - "docs/dogfood/001/DRAFT_HANDOFF.md"
  - "docs/dogfood/001/review/FINDING.md"
  - "docs/dogfood/001/APPLY_HANDOFF.md"
  - "docs/dogfood/001/EVIDENCE.md"
  - "docs/dogfood/001/RUN_SUMMARY.md"
  - "docs/dogfood/001/findings/HARNESS-001.md"
  - "docs/dogfood/001/findings/HARNESS-002.md"
  - "docs/dogfood/001/findings/HARNESS-003.md"
  - "docs/dogfood/001/review/HARNESS-004.md"
---

# Synthesis — dogfood-001

Run: `run_a04880660517480a95438fcc0368d2e0`
Branch: `striatum/dogfood-001-graph-dot`
Outcome: completed in 20m46s; verdict `accept_with_findings`; 3/3 jobs
completed; tests 142 → 143; lint and mypy clean.

## TL;DR

The product change (Graphviz DOT export for `striatum workflow graph`) was
small and uneventful. The point of the dogfood was to discover V1 friction
by driving a real run, and the run delivered four concrete harness gaps
plus a recurring pattern that explains all of them.

The supervised lane that the dogfood scaffold ships with
(`claude --model opus -p`) cannot execute work packets. A stale editable
install silently hid migration v5. Reviewer-independence and byline
policies turned out to be advisory only, surfacing because the operator
had to drive both lanes manually after the supervisor friction. And the
reviewer role doc tells reviewers to write to a path their write_scope
forbids.

## What we shipped

`workflow_graph_dot()` in `src/striatum/workflow.py` (DOT mirror of
Mermaid's `workflow_graph_data` shape — same nodes, same dependency
edges, parallel groups as `cluster_<group>` subgraphs, dashed
`needs_revision` cycle edges). New `--format dot` choice in the parser,
dispatch wiring, a `tests/test_cli_mvp.py::test_workflow_graph_exports_dot`
covering structure plus a Graphviz `dot -Tsvg` arm that auto-skips when
Graphviz is not installed. SPEC, README, CHANGELOG entries.

## What we found

Four harness improvement proposals, three filed in the artifact table
(HARNESS-001, HARNESS-002 from the author lease; HARNESS-004 from the
review lease) and one on disk only (HARNESS-003 — the reviewer's
write_scope rejected the path before publish).

### HARNESS-001 (target: defaults) — Default `claude_code` lane cannot execute work packets

`claude --model opus -p` reads stdin as a single prompt, exits, and has
no permissions to call `striatum` subcommands. The supervisor process
died 14s after `claim-next`; the lease stayed held against a dead pid;
stdout/stderr are `DEVNULL` so the failure was silent. The runner's
`supervisor.lost` event fired but no `next_action` surfaces it to the
operator. **From this point on the operator drove every job manually
using the held leases.** Every other finding flows from this one.

Severity: blocker. Without this, dogfood-001 cannot run as a supervised
workflow. Recommended fix layers (in order of cost):
1. Document the supervised-lane command contract in `docs/SPEC.md` —
   "must stay alive, must read newline-delimited JSON packets from
   stdin, must call back via `striatum` CLI".
2. Surface dead-supervisor-with-held-lease as a `next_action` in
   `status` and a high-severity diagnostic in `doctor`.
3. Ship a working default lane invocation (probably a long-running
   `claude` session with a Striatum protocol skill loaded and the
   permissions it needs to run `striatum` subcommands and edit files).
   This is the hard one and probably warrants RFC 0010 (PTY
   supervisor) work first.

### HARNESS-002 (target: defaults) — Editable install can silently pin Striatum to a stale worktree copy

`pip show striatum` reported the editable install location as
`.claude/worktrees/agent-…/`, a temporary Claude Code worktree. Source
on disk had migration v5 (drops the artifact_kind SQL CHECK); the
installed copy did not. `striatum init` happily created a v4 DB; the
first `publish-artifact --kind harness_improvement_proposal` blew up
on the SQL CHECK with no useful guidance. `pip install -e
/home/halbritt/git/striatum` (canonical path) plus the next connect
silently upgraded the DB to v5. **This foot-gun is reasonably likely to
recur.** Recommended fix:
1. `striatum doctor` compares `striatum.__file__` against the repo
   argument; warns loudly when they diverge.
2. `striatum init` compares the on-disk `LATEST_VERSION` against the
   running install's `LATEST_VERSION`; refuses to initialize a fresh
   state DB if the install lags.
3. `Makefile install` resolves the install path explicitly so
   `make install` from a worktree doesn't silently pin.

### HARNESS-003 (target: spec) — Reviewer-independence and byline are advisory, not enforced

`fresh_session_required: true` and `reviewer_context_policy: fresh` are
recorded on the job row but the runner never actually enforces them
beyond session-id distinctness. The published `author:` line in a
review artifact only has to *match the workflow's declared expected
byline* — which is the workflow's claim about who *should* be reviewing,
not a check against who actually wrote the file. In dogfood-001 the
operator (the same Claude Code instance that authored the change) drove
the review, and the runner happily recorded `author:
reviewer-codex-gpt-5.5-001` in the run summary even though no codex
process ever touched the run. Recommended fix layers:
1. Document plainly in SPEC that fresh policies are advisory.
2. Doctor warning when author and reviewer sessions share parent pid,
   or when a reviewer registers without a supervisor adapter on a run
   where the author had one.
3. `register-session --role reviewer` policy: refuse same-parent-pid
   when the job declares `reviewer_context_policy: fresh`, with an
   explicit `--force-non-fresh --reason` escape hatch.
4. Byline integrity: when an artifact is published with no `author:`
   line and the packet declared one, the publisher should record
   "byline missing" rather than the declared expected byline.

### HARNESS-004 (target: documentation) — Reviewer role doc contradicts the review job's write scope

`docs/dogfood/001/roles/reviewer.md` tells reviewers to file harness
proposals under `docs/dogfood/001/findings/`. The review job's
`write_scope.allowed_paths` only includes `docs/dogfood/001/review/`.
The publisher correctly refuses (exit 6: "artifact path is outside the
job write scope"), but the operator only learns at publish time. Fix:
either change the role doc to point at `review/HARNESS-NNN.md` (cheap)
or widen the write scope (semantically wider) or introduce a
workflow-level harness scratch path (structural, RFC-shaped).

## Cross-cutting pattern

Every one of the four findings followed the same shape: **the dogfood
scaffold says one thing, the runner enforces another, and the operator
only learns the truth at command time.**

- Scaffold says `claude --model opus -p` is a working `claude_code`
  lane; runner cannot execute against it.
- `make install` says it installs Striatum; runner runs an old copy
  pinned to a temp worktree.
- Workflow says the reviewer is fresh and codex; runner accepts the
  same operator wearing two session ids.
- Role doc says "file harness under findings/"; runner refuses on
  scope.

The remediation pattern is symmetric: **make the runner refuse or warn
loudly when the scaffold's promise diverges from runner-enforced
reality.** Doctor warnings are the cheapest first step in every case;
hard refusal at command-time is the appropriate next layer for
HARNESS-001/002 (since proceeding past those silently corrupts later
artifacts).

## Recommendations for v2

dogfood-001 v2 should drive the four fixes themselves, on a fresh
branch, with the existing dogfood-001 friction already in mind. That
gives us a measurable answer to "did the fixes actually unblock a
supervised workflow", and it dogfoods the runner's own remediation
process.

Suggested v2 task split (one author job per fix, or one bundled
draft):
- **Fix 001**: doc + doctor + supervisor-orphan next_action. Ship the
  long-running supervised lane only if RFC 0010 (PTY) is also
  in-scope; otherwise leave the working-default-lane sub-task to a
  follow-up dogfood that depends on RFC 0010.
- **Fix 002**: doctor warning + init guard + Makefile path resolution.
- **Fix 003**: spec doc + doctor warnings + register-session policy
  with `--force-non-fresh` flag + byline-missing recording.
- **Fix 004**: doc fix to the dogfood-001 reviewer role (and audit any
  other role doc that mentions a path).

Acceptance for v2:
- `striatum supervise start` + `claim-next` actually runs the work to
  `complete` end-to-end, OR `doctor` surfaces the supervisor death as
  a `next_action` within 30s.
- `striatum init` against a stale install refuses with a clean error.
- `striatum register-session --role reviewer` with same-parent-pid
  refuses unless `--force-non-fresh --reason` is provided.
- All four reviewer role docs in `docs/dogfood/*/roles/reviewer.md`
  point at scope-valid paths.

## Open questions for the next operator

1. Should v2 land all four fixes in one bundle (one PR), or split per
   harness number (four PRs)? Bundling is simpler for the dogfood
   loop; splitting is cleaner for review and bisect.
2. Is HARNESS-001's "working default lane" sub-task in scope for v2,
   or does it block on RFC 0010 (PTY supervisor)? If the latter, v2's
   HARNESS-001 work is doc + doctor + next_action only, and a
   dogfood-002 covers the working-supervised-lane validation after
   RFC 0010 lands.
3. Should the reviewer for v2 be a real codex process this time? It
   would be the first signal that *any* supervised lane works, even
   imperfectly. If codex's `exec -` mode also cannot execute packets,
   that is itself a useful finding.
