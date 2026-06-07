# Unattended DoD driver — the "no human intervention" standing acceptance

`driver.py` is the **outside-CI live acceptance** for Striatum's central claim:
the runner carries a multi-lane, review-gated, revision-capable build to
completion with **no operator rescue**, repeatably.

It complements the hermetic RFC 0105 harness (`go/pkg/adapterconformance/`): the
harness proves the recovery/lifecycle state machine deterministically with
fake/in-process agents; this driver proves the same property **live**, against
the real daemon and the workflow's configured lane CLIs.

## What it does

For each of N iterations it `run prepare` + `run start`s a fresh run on a
registered target repo, then delegates the foreground drive loop to
`striatum run drive --run-id <id>`:

- `run drive` registers and supervises one fresh session per queued role/lane as
  the DAG unblocks;
- it adopts already-active matching sessions, so re-running the wrapper does not
  duplicate lanes;
- it stops terminal or superseded launched lanes before fresh-reviewer registration, so
  downstream reviewers do not require a manual close-author-before-reviewer
  step.

It **never** calls a rescue verb (`recovery requeue-stale`, `override-verdict`,
`run retry-job`, `--force`). A run that reaches `completed` this way is a clean
pass; a run that escalates (`needs_operator` / `waiting_human`) is a FAIL — that
is precisely the "a human would be needed" condition the DoD forbids.

## Usage

```bash
python3 scripts/dod/driver.py <repo_root> <workflow.json> <N>
# e.g. (the floor dogfood: agent-loop lane, draft -> review -> revision cycle)
python3 scripts/dod/driver.py /tmp/striatum-floor-acceptance workflow.json 10
```

Prerequisites: the daemon is running; the target repo is registered
(`striatum repo add --path <repo> --init`) and trusted; every lane command in
the workflow is installed. The driver exits 0 iff all N runs passed.

## Assumptions / scope

- The example target is the **floor dogfood**: an agent-loop lane
  (`adapter_capabilities.agent_loop`, `pty_helper`, `require_tmux`) with a
  bounded `needs_revision` cycle. Lane resolution comes from the prepared
  workflow snapshot and live job rows; no lane id is hard-coded in this wrapper.
- Runs are sequential (one at a time); each uses `branch.mode: auto`, so each
  lands on its own branch and repeated runs do not collide.

## Result of record

2026-06-03, against v2.14.0 (the deployed daemon, post-#162/#163): **10/10
consecutive clean unattended passes** — every run `completed`, every review
`accept`, **zero** rescue-type events, **zero** escalations (independently
verified in the daemon). This was unblocked by the live-acceptance fixes #162
(session-bound token lacked `read`, so lanes could not claim) and #163 (claude
parked on the workspace-trust dialog on a fresh repo).
