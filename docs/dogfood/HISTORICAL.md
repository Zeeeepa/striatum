# Historical Dogfood Runs

> **These are historical incubation artifacts, not the active
> dogfood cadence.** Runs 001 through 013 belong to the era
> between RFC 0001 and the v1.4.0 line, when each dogfood was
> partly an exploration of the harness itself. The current
> cadence — one dogfood per RFC step, lighter ceremony, well-
> defined research/synthesis/review/implement/review jobs — is
> visible in runs 014 onward (each linked from
> `docs/DECISION_LOG.md` and `docs/rfcs/<NNNN>` index entries).
> Read the runs here only when you need to understand a
> specific historical pattern or harness friction; do not copy
> their workflow shape into a new dogfood without checking a
> recent run (e.g. run 020) first.

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

## Subsequent runs (current cadence)

Dogfood runs **006 onward** are part of the current cadence —
one per RFC step, with the standard five-job shape (research →
synthesize → review-design → implement → review-build). They
land via the `Land RFC NNNN ... (dogfood-NNN)` commit pattern
in `git log` and are referenced from the matching `D###` row
in `docs/DECISION_LOG.md`. The full per-run material lives
under `docs/dogfood/<id>/`.

Recent runs and what they shipped:

| Run | RFC | Tag | Highlights |
|---|---|---|---|
| 020 | RFC 0022 V1 | v1.11.0 | Server-rendered Jinja2 multi-page UI, refreshed CSS palette + dark mode, layered SVG dependency graph. |
| 019 | RFC 0021 V1.5 | v1.10.0 | `--ddd-layout-force` + `--ddd-layout-dry-run` on `striatum init --with-ddd-layout`. |
| 018 | RFC 0018 step 3 | v1.9.0 | `verdicts.posture` column + introspection across status / run-summary / evidence / run-graph / dashboard / web UI. |
| 017 | RFC 0021 V1 | v1.8.0 | `striatum init --with-ddd-layout` scaffolds the seven canonical human-facing DDD docs. |
| 016 | RFC 0018 V1 | v1.7.0 | `review_posture` field + `required_review_postures` reachability gate. |
| 015 | RFC 0020 step 3 | v1.6.0 | `recovery watch` long-lived sweeper daemon. |
| 014 | RFC 0020 V1 | v1.5.0 | `recovery auto` one-shot sweeper + `recovery_policy` workflow block + escalation hooks. |

For the index of every run (including the historical 001–013
above), see `docs/dogfood/`.
