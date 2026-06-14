# RFC 0126/0127/0128 implementation campaign

Date: 2026-06-14 · Author: operator-claude-opus-4-8 · Repo: `/home/halbritt/git/striatum` (`main`)

Companion to the provenance-durability campaign closeout
(`docs/operator/plans/provenance-durability-campaign-2026-06-14.md`). The three
follow-up RFCs were **accepted** (D194/D195/D196 — see the decision log and
`docs/rfcs/README.md`); this plan scaffolds their **implementation as striatum
dogfoods**, one `code_change` run per RFC's P0 slice.

## Why dogfoods (not hand-implementation)

Per the standing convention the operator scaffolds dogfoods rather than
hand-implementing role artifacts. The two ancillary issues (#285, #286) were
direct fixes (landed: `579c1c34`, `d9ae5411`); the RFCs — which are phased daemon
changes touching schema and the completion/verdict/workspace machinery — are
routed through striatum so each implementation is reviewed and provenance-tracked.

## The scaffolds

Each scaffold is a `code_change` shape (`draft → review → apply`, one bounded
revision cycle) on a single `claude` agent lane with per-job worktree isolation.
The `draft` prompt carries the focused P0 brief and the load-bearing gotchas; the
lane writes the implementation onto the run's **feature branch** (never `main`).

| RFC | Slice | Scaffold | Workflow id |
| --- | --- | --- | --- |
| 0126 (D194) multi-reviewer revision coherence | P0: `verdicts.review_generation` column + build counter; stamp in `applyVerdict`; same-tx bump in `reopenJobForAttempt`; stop the verdict DELETE | `docs/campaigns/rfc-0126/` | `rfc-0126-p0-review-generation` |
| 0127 (D195) retire the lane git identity | P0: opt-in `workspace_kind: plain_dir` in `workspace.create` + base-tree-sha recorded | `docs/campaigns/rfc-0127/` | `rfc-0127-p0-plain-dir-workspace` |
| 0128 (D196) cross-repo run boundary | P0: validate-time cross-repo lint (write-scope outside repo root ⇒ fail exit 7; foreign prompt slug ⇒ warn) | `docs/campaigns/rfc-0128/` | `rfc-0128-p0-crossrepo-lint` |

0126 is the highest value (it unblocks reliable multi-reviewer dogfoods) and is
launched first.

## Launch runbook

The daemon is a hard prerequisite. From the repo root, per RFC:

```bash
# 1. snapshot + prepare (branch.mode=auto creates the branch and reaches ready)
striatum --repo /home/halbritt/git/striatum run prepare \
  --workflow docs/campaigns/rfc-0126/workflow.json --json
# 2. start (RFC 0124 auto-drive is default-on: a detached `run drive` runs the loop)
striatum run start --run-id <id>
# 3. watch
striatum dashboard --run-id <id> --once
```

### Lane host provisioning (hardened repo)

`striatum` is a hardened target repo: supervised lanes run as a sandbox OS user
(e.g. `striatum-lane`), so the per-job worktree path and `.striatum/scratch` need
ACLs or `supervise start` fails at the mcp-config write. Before launching, ensure
the lane user has `rwx` + a DEFAULT ACL on the repo's `.striatum/` scratch and the
worktrees root (see `reference_lane_user_host_provisioning` and
`docs/how-to/lane-sandbox.md`). A stale `STRIATUM_DAEMON_SOCKET` in the lane env
is noisy-but-nonfatal.

## Load-bearing gotchas (encoded in each draft prompt)

- **Owner-table migration hazard.** `verdicts` (0126) and any new `job_workspaces`
  column (0127) may be owner-held; a runtime `ALTER` on an owner table
  **crash-loops the daemon** (RFC 0081 incident). Verify ownership first and route
  owner DDL through an owner bundle (`go/pkg/db/sql/owner/`).
- **Migrations are append-only + numbered**; bump `LatestDaemonDBVersion` for a
  new runtime migration.
- **Scope discipline.** Each run implements ONLY its P0 slice; P1–P3 are separate
  runs. RFC 0106 graduated-shape freeze holds (no new artifact kinds).
- **Don't break #285's reconstructability gate** or RFC 0118's verdict path.

## Operator review gate (do NOT auto-merge)

A dogfood-produced daemon change lands on its **feature branch**, not `main`.
Before integrating: re-run `make -C go build`, the CI lint
(`golangci-lint run --default=none --enable=govet --enable=staticcheck
--enable=errcheck --enable=ineffassign ./...`), and the P0 pgtests on the branch;
read the diff (especially the migration ownership decision); then FF to `main`.
Autonomous daemon code is never trusted on a completion claim alone.

## Status

- Scaffolds committed and validated (`workflow validate` → `valid` for all three).
- 0126 P0 launch: see the campaign log appended below / the run dashboard.
- 0127, 0128: scaffolded and ready; launch after 0126 lands (sequence by value).
