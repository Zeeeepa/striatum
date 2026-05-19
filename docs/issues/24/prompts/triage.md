# Triage -- GH #24 scope

You are the triager for this issue workflow. Produce only the declared
scope artifact for this workflow. Do not implement source changes.

## Read

1. `docs/issues/24/SPEC.md`
2. `src/striatum/cli/dispatch.py` -- the `claim-next` and
   `supervise send` dispatch paths. Look especially at how the
   `claim_next` daemon response is formatted before printing to stdout
   (is the operator-facing structure shaped differently from the
   in-process packet object?).
3. `go/pkg/mutations/claim.go` -- the canonical packet structure
   (note `packet_id` at line ~217 lives inside the packet map). Decide
   whether the right fix is at the daemon RPC layer (claim returns
   `packet_id` as a top-level data field as well) or at the CLI
   formatter layer.
4. `go/pkg/mutations/supervision_control.go:HandleSuperviseSend` --
   the `not_found` error path (line ~227+).
5. `go/pkg/mutations/release.go` (or wherever `HandleRelease` lives) --
   the `--requeue` semantics for repo_write jobs.
6. `~/.claude/skills/striatum-supervise/SKILL.md` and
   `~/.claude/skills/striatum-claim-loop/SKILL.md` -- the skill
   bundles that need a worked example.
7. `docs/HOW_TO_AGENT.md` -- the operator-facing workflow doc.
8. `docs/DECISION_LOG.md` -- specifically D036 (lazy-lease semantics)
   to know whether repo_write requeue is supposed to refuse.

## Output

Write `docs/issues/24/SCOPE.md` with `striatum.synthesis.v1` front matter
and the exact `author:` line from the work packet. Include:

- the exact files in scope for the fix (CLI dispatch / packet
  formatting, Go claim handler if surfacing packet_id at top level,
  Go release handler, skill SKILL.md files, tests);
- the exact files out of scope (do NOT touch unrelated supervisor
  pidfile work, unrelated migration paths, supervisor stop logic);
- an acceptance checklist with one numbered check per bullet in
  `docs/issues/24/SPEC.md` "Acceptance / Definition of done";
- the chosen approach for each of the two bugs, with justification
  rooted in the actual CLI output / Go handler structure;
- verification commands (ruff + mypy + targeted Python tests + Go
  test ./pkg/mutations + Go test ./pkg/rpc + a manual claim-next →
  supervise send round-trip);
- risks and conflicts with parallel issue workflows (#22 touches
  `src/striatum/cli/dispatch.py`, `src/striatum/daemon_pg/`; #23
  touches `src/striatum/cli/`, `go/cmd/striatumd/`, `go/pkg/`).
  Name the specific functions / lines that are likely to merge-conflict
  and recommend an order (this workflow should run AFTER #22 and #23
  have shipped, unless the triager has a sharper carve-out).
