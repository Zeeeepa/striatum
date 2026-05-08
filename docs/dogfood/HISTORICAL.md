# Historical Dogfood Runs

The dogfood runs collected here are the incubation history of
striatum-on-striatum work between RFC 0001 and the v1.0.0 line.
Each run's per-id directory under `docs/dogfood/<id>/` carries
the canonical material (`workflow.json`, `RUNBOOK.md` if present,
prompts, roles, research, reviews, build handoffs, run summaries).
This page is the pointer.

## Dogfood 001 — first striatum-on-striatum scaffold

`docs/dogfood/001/` is the first striatum-on-striatum dogfood
scaffold. It drives a small code change — adding Graphviz DOT
export to `workflow graph` — but the real purpose was to exercise
the runner with real agent CLIs and capture harness friction as
durable `harness_improvement_proposal` artifacts.

For a human-run session, start with the runbook:

```bash
less docs/dogfood/001/RUNBOOK.md
```

For an agent handoff, give the agent the repo-local skill:

```text
Use the skill at /path/to/striatum/docs/dogfood/001/SKILL.md to
start and drive dogfood-001.
```

Starting a dogfood run does not launch an interactive orchestrator
chat. The runner creates SQLite workflow state, makes jobs
claimable, and can supervise an agent process. Humans drive the
run with `striatum` commands and watch it through `dashboard`,
`status`, `why`, and the artifacts the agents publish.

## Dogfood 003 — RFC 0010 V1 (tool harness profiles)

`docs/dogfood/003/` scaffolds the RFC 0010 tool-harness-profile
dogfood run: verify the existing Codex, Claude Code, and Gemini
CLI research; synthesize an implementation design from the
concrete profile candidates; review it; record human acceptance;
implement the first slice; review the build.

For an agent handoff:

```text
Use the skill at /path/to/striatum/docs/dogfood/003/SKILL.md to
start and drive dogfood-003.
```

The workflow intentionally asks research agents to try native
sub-agents or equivalent delegation for independent research
subtasks while keeping the parent striatum session accountable
for final artifacts and state changes. It also carries RFC 0010's
proposed `harness_profiles` map as a fixture for the
implementation job to validate and expose in work packets.

## Dogfood 004 — RFC 0010 V2 (Claude Code supervised wrapper)

`docs/dogfood/004/` scaffolds the V2 follow-up of dogfood-003's
HARNESS-001 finding: author
`.striatum/bin/claude-supervised-wrapper.sh` so workflows that
declare a supervised Claude Code lane (per RFC 0009) actually run.
The workflow is research → design → review → human acceptance →
implementation → build review.

For an agent handoff:

```text
Use the skill at /path/to/striatum/docs/dogfood/004/SKILL.md to
start and drive dogfood-004.
```

`striatum workflow validate docs/dogfood/004/workflow.json --json`
intentionally surfaces a single V1.5 lint warning naming the
missing wrapper path; landing the wrapper is the goal of the run,
after which the warning goes away.

## Dogfood 005 — RFC 0014 V1 (process adapter completion guarantees)

`docs/dogfood/005/` scaffolds the run for RFC 0014 V1
(process-adapter completion guarantees). The workflow is research
→ design synthesis → review → human acceptance → implementation
→ build review. Targets closure of
[issue #1](https://github.com/halbritt/striatum/issues/1).

For an agent handoff:

```text
Use the skill at /path/to/striatum/docs/dogfood/005/SKILL.md to
start and drive dogfood-005.
```

## Bootstrap tmux harness

The temporary design-bootstrap runner remains available for
historical design-fixture work:

```bash
scripts/striatum_tmux_design.sh start
tmux attach -t striatum-design
```

This script is not the product control plane. It exists to
bootstrap MVP design/build work until the generic process adapter
(`adapter run`) and supervised sessions (`supervise`) cover that
workflow end-to-end. With v1.0.0 they do; the script is retained
as provenance.

## Subsequent runs

Dogfood runs 006 onward (RFC 0012 service, RFC 0013 web UI, RFC
0016 dashboard graph, RFC 0015 skill bundles, RFC 0017 docs
reorganization) follow the same scaffold shape under
`docs/dogfood/<id>/`. See [the dogfood index](.) for the full
list.
