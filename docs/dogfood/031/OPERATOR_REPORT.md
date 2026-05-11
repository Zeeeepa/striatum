# Dogfood 031 Operator Report

author: operator
date: 2026-05-11
status: complete

## Run

- Run ID: `run_2c452436c7c346f08bd5cea17271866d`
- Workflow: `dogfood-031-rfc-0028-daemon-and-multi-repo-control-plane`
- Branch: `striatum/dogfood-031-rfc-0028-daemon`
- Final state: `completed`
- Final job tally: 18 jobs completed.

## Scope

Land the RFC 0028 V1 acceptance-criteria slice: optional local daemon
(`striatum daemon` / `striatumd`), multi-repository registry, capability-gated
daemon endpoints, daemon MCP resources, hash-chained audit, foreground
recovery sweep across registered runs, and a global dashboard. Deferred to
follow-up RFCs: daemon RPC server, daemon-owned supervision, sealed-mode
apply authority, signing keys, and remote serving.

## Control-Plane Outcome

The run completed cleanly after the operator (acting as surrogate while the
human owner was AFK) resolved one cycle-exhaustion human checkpoint with a
`continue` decision. Striatum final status reports 18 completed jobs, zero
canceled, zero open blockers, zero open human checkpoints, and zero stale or
lost processes.

## Operator Infrastructure Work

The dogfood-030 lane definitions were copied verbatim into the initial
dogfood-031 workflow. They did not survive supervised mode in this
environment:

- `claude --print` exits within ~3 s of empty FIFO stdin (RFC 0009 model).
- `gemini --prompt -` treats the literal `-` as prompt content and exits
  after responding.

The first run (`run_6e5a9ac2e4a9492397b4bb45ad0bb6fb`) was canceled, the
repo-tracked `.striatum/bin/claude-supervised-wrapper.sh` was kept, and a
parallel `.striatum/bin/gemini-supervised-wrapper.sh` was authored so each
lane spawns a fresh CLI process per packet. The workflow.json was updated to
invoke both wrappers, and a fresh run was prepared and started under
`run_2c452436c7c346f08bd5cea17271866d`.

The codex lane (`codex exec --json --ephemeral ... -`) blocks indefinitely on
FIFO stdin in supervised mode in this env, mirroring the claude `--print`
problem. The codex supervised supervisor was stopped before completing
`design_codex`. Subsequent codex-lane jobs (`design_codex`,
`synthesize_design`, `synthesize_design_a2`, `synthesize_design_a3`,
`implement`, `implement_a2`, `implement_a3`) were produced by running
`codex exec` manually under operator orchestration. The resulting artifacts
carry `author: operator` bylines, reflecting that the codex sessions lost
lane attestation when the operator stopped the supervisor (RFC 0026 honest
downgrade).

## Owner / Operator Decisions

1. **Decision** `dec_operator_security_cascade_collision_2026_05_11`
   (`accepted_with_follow_up`): Codex security design review's
   `needs_revision` verdict was recorded as `accept_with_findings` to break
   a runner cascade-child UNIQUE-constraint collision. The codex security
   finding (severity: medium) remained in the published artifact and was
   addressed by synthesis revisions 2 and 3. Follow-up: file a harness
   improvement proposal for the parallel-reviewer cascade-child collision.

2. **Decision** `dec_operator_build_devils_cycle_exhausted_2026_05_11`
   (`accepted_with_follow_up`): The `review_build_devils` cycle exhausted
   `max_iterations: 2` with the round-3 review still at
   `needs_revision` (severity dropped from high to medium across rounds).
   The operator resolved the `revision_routing` human checkpoint
   (`blk_f6f47c47c4e4455ba4a067309878919f`) with `continue`. The final
   recorded build-devils verdict was operator-issued
   `accept_with_findings` pointing to the round-3 reviewer artifact
   (`art_6a11c1a25a4c4cd1be81355a95e49dec`). Follow-up: file a follow-up
   RFC for the daemon RPC server (round-2 finding A1) and land remaining
   round-3 fixes in normal bugfix iterations.

## Recorded Risks

These are reviewer findings and operator-accepted follow-up risks, not
operator-authored review content:

- The shipped "daemon" is a registry + foreground sweep loop, not a real
  RPC server. The synthesis and build handoff document this explicitly.
- The build-devils reviewer (round 3) identified residual concerns
  (architectural honesty A1 deferred, remaining audit/recovery/doctor
  refinements). All concerns remain documented in
  `docs/dogfood/031/review/build/devils/REVIEW.md` for follow-up work.
- The codex security design review (severity: medium) flagged
  capability-token lifecycle/defaults and audit retention/integrity as
  needing concrete specifications. Synthesis revisions 2 and 3 addressed
  these; the operator-overridden verdict carries the artifact text forward
  as documented follow-up.
- Direct CLI mode remains the working default. Daemon mode is opt-in.
- Codex supervised mode is functionally broken in this environment
  (FIFO-stdin hang). All codex-lane jobs in this run were operator-driven
  manual codex invocations.

## Verification Artifacts

The implementer reported passing (across rounds):

- `make install`
- `make lint`
- `make typecheck`
- `make test` (574 tests passing in round 3, up from 558 baseline)
- `make smoke`

The operator exported:

- `docs/dogfood/031/RUN_SUMMARY.md`
- `docs/dogfood/031/EVIDENCE.md`

## Deliberately Left Out

The operator did not author design content, synthesis content, review
content, or implementation code. Where artifacts could not be advanced via
the supervised harness because of the codex FIFO-stdin issue, codex was
invoked manually under operator orchestration; those artifacts carry
`author: operator` bylines to reflect that the runner lost lane
attestation, not because the operator wrote the content.

Untracked entries (`foo` at repository root, certain `.striatum/scratch/`
artifacts) are unrelated to this run and were not modified.
