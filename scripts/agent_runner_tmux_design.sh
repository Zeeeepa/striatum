#!/usr/bin/env bash
set -euo pipefail

AGENT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "$AGENT_ROOT/.." && pwd)"
SESSION="${AGENT_RUNNER_SESSION:-agent-runner-design}"
RUN_MODE="${AGENT_RUNNER_RUN_MODE:-print}"
WAIT_SECONDS="${AGENT_RUNNER_WAIT_SECONDS:-20}"
PROMPT="prompts/P001_design_review_build_v1_mvp.md"

usage() {
  cat <<'EOF'
Usage:
  agent-runner/scripts/agent_runner_tmux_design.sh start
  agent-runner/scripts/agent_runner_tmux_design.sh start-pipe
  agent-runner/scripts/agent_runner_tmux_design.sh run-job <job>
  agent-runner/scripts/agent_runner_tmux_design.sh print-prompt <job>
  agent-runner/scripts/agent_runner_tmux_design.sh status
  agent-runner/scripts/agent_runner_tmux_design.sh next

Jobs:
  design_claude
  design_codex
  design_gemini
  synthesis_ready

Environment:
  AGENT_RUNNER_SESSION       tmux session name (default: agent-runner-design)
  AGENT_RUNNER_RUN_MODE      print | pipe (default: print)
  AGENT_RUNNER_WAIT_SECONDS  poll interval for synthesis_ready (default: 20)

  CODEX_CMD                  stdin-taking command for Codex GPT-5.5
                             (default: codex -a never exec --model gpt-5.5
                              --sandbox danger-full-access -)
  CLAUDE_CMD                 stdin-taking command for Claude Opus
                             (default: claude --model opus
                              --dangerously-skip-permissions)
  GEMINI_CMD                 stdin-taking command for Gemini Pro 3.1
                             (default: gemini --model
                              gemini-3.1-pro-preview --yolo)

Modes:
  print  Print a lane-specific prompt handoff in each tmux pane.
  pipe   Pipe the lane-specific prompt into the configured model command.

The design input Markdown files are the watched completion artifacts for this
bootstrap pass. This runner is temporary orchestration for the agent_runner MVP
design, not the product architecture.
EOF
}

jobs() {
  cat <<'EOF'
design_claude
design_codex
design_gemini
synthesis_ready
EOF
}

design_jobs() {
  cat <<'EOF'
design_claude
design_codex
design_gemini
EOF
}

job_config() {
  MODEL=""
  LABEL=""
  OUTPUT=""
  EMPHASIS=""

  case "$1" in
    design_claude)
      MODEL="claude_opus"
      LABEL="Claude lane"
      OUTPUT="docs/design/V1_MVP_DESIGN_INPUT_claude.md"
      EMPHASIS="product boundary, workflow ergonomics, coordinator attention, and adversarial review of process risks"
      ;;
    design_codex)
      MODEL="codex_gpt5_5"
      LABEL="Codex lane"
      OUTPUT="docs/design/V1_MVP_DESIGN_INPUT_codex.md"
      EMPHASIS="Python implementation shape, SQLite schema, CLI contract, test strategy, and migration from bootstrap tmux"
      ;;
    design_gemini)
      MODEL="gemini_3_1_pro"
      LABEL="Gemini lane"
      OUTPUT="docs/design/V1_MVP_DESIGN_INPUT_gemini.md"
      EMPHASIS="model portability, workflow config, artifact policy, hidden provider assumptions, and failure modes"
      ;;
    synthesis_ready)
      MODEL="coordinator"
      LABEL="Synthesis handoff"
      OUTPUT=""
      EMPHASIS=""
      ;;
    *)
      printf "Unknown job: %s\n" "$1" >&2
      return 1
      ;;
  esac
}

command_for_model() {
  case "$1" in
    codex_gpt5_5)
      local cmd="${CODEX_CMD:-}"
      if [[ -z "$cmd" || "$cmd" == "codex --model gpt-5.5 --yolo" ]]; then
        cmd="codex -a never exec --model gpt-5.5 --sandbox danger-full-access -"
      fi
      printf "%s" "$cmd"
      ;;
    claude_opus)
      printf "%s" "${CLAUDE_CMD:-claude --model opus --dangerously-skip-permissions}"
      ;;
    gemini_3_1_pro)
      printf "%s" "${GEMINI_CMD:-gemini --model gemini-3.1-pro-preview --yolo}"
      ;;
    *)
      return 1
      ;;
  esac
}

shell_quote() {
  printf "%q" "$1"
}

tmux_env_prefix() {
  local prefix
  local var
  prefix="AGENT_RUNNER_RUN_MODE=$(shell_quote "$RUN_MODE")"
  prefix="$prefix AGENT_RUNNER_WAIT_SECONDS=$(shell_quote "$WAIT_SECONDS")"
  for var in CODEX_CMD CLAUDE_CMD GEMINI_CMD; do
    if [[ -n "${!var:-}" ]]; then
      prefix="$prefix $var=$(shell_quote "${!var}")"
    fi
  done
  printf "%s" "$prefix"
}

artifact_exists() {
  [[ -f "$AGENT_ROOT/$1" ]]
}

all_design_outputs_exist() {
  local job
  while IFS= read -r job; do
    job_config "$job"
    artifact_exists "$OUTPUT" || return 1
  done < <(design_jobs)
  return 0
}

lane_prompt() {
  local job="$1"
  job_config "$job"

  cat "$AGENT_ROOT/$PROMPT"
  cat <<EOF

---

agent_runner tmux bootstrap assignment:

The base prompt above describes the complete one-shot. This tmux pane is only
one design-lane input for that one-shot.

Assignment:
- Lane: $LABEL
- Model slug: $MODEL
- Required output artifact: $OUTPUT
- Primary emphasis: $EMPHASIS
- Project root for this assignment: $AGENT_ROOT
- Engram repo root: $REPO_ROOT

Scope override:
- Do not synthesize the three-lane design.
- Do not review the synthesized design.
- Do not implement source code.
- Do not create or switch branches. The one-shot coordinator owns branch setup.
- Do not update docs/SPEC.md, docs/DECISION_LOG.md, or docs/UBIQUITOUS_LANGUAGE.md.
- Do not edit files outside the single required output artifact.

Required reading:
- Read the files named in the base prompt from the agent_runner project root.
- Also read docs/ENGRAM_INCUBATION_CONTEXT.md.
- For Engram repo-root paths listed there, prefix ../ when running from this
  agent_runner directory.
- Inspect ../scripts/phase3_tmux_agents.sh enough to understand the bootstrap
  pain, but do not treat it as target architecture.

Before writing:
- Run git status --short.
- If the required output artifact already exists, read it and decide whether it
  fully satisfies this lane. Do not overwrite unrelated human changes.

Write exactly this Markdown artifact:

  $OUTPUT

Use this structure:

1. Title
2. Executive recommendation
3. MVP boundary
4. Work packet schema
5. SQLite schema and event/queue semantics
6. CLI command surface
7. JSON workflow config shape
8. Persistent sessions, fresh roles, and tmux/process adapter behavior
9. Artifact and decision policy
10. RFC-ledger validation fixture
11. Test strategy
12. Risks, blockers, and deferred work
13. Concrete recommendations for synthesis

Keep the output actionable. Prefer schemas, command shapes, state transitions,
and explicit tradeoffs over prose gloss. If you find a real blocker, record it
inside the required output artifact with a clear "Blocker" section.
EOF
}

print_synthesis_handoff() {
  cat <<'EOF'
=== agent_runner design inputs complete ===

All three required design input artifacts exist:

  docs/design/V1_MVP_DESIGN_INPUT_claude.md
  docs/design/V1_MVP_DESIGN_INPUT_codex.md
  docs/design/V1_MVP_DESIGN_INPUT_gemini.md

Next coordinator work:

1. Read all three inputs.
2. Create/update:

   docs/design/V1_MVP_DESIGN.md
   docs/reviews/v1/V1_MVP_DESIGN_REVIEW.md
   docs/reviews/v1/V1_MVP_FINDINGS_LEDGER.md
   docs/reviews/v1/V1_MVP_SYNTHESIS.md

3. Update docs/SPEC.md into the implementation contract.
4. Record any accepted product/architecture decisions in docs/DECISION_LOG.md.
5. Update docs/UBIQUITOUS_LANGUAGE.md if the design sharpens terms.

Use prompts/P001_design_review_build_v1_mvp.md as the controlling process
prompt and continue from the design synthesis/review sections. The synthesis
must cite all three design inputs and explicitly mark recommendations as
accepted, modified, deferred, or rejected.
EOF
}

wait_for_design_outputs() {
  until all_design_outputs_exist; do
    printf "[%s] waiting for design input artifacts\n" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    status_artifacts
    sleep "$WAIT_SECONDS"
  done
}

run_job() {
  local job="$1"
  job_config "$job"

  cd "$AGENT_ROOT"
  mkdir -p docs/design docs/reviews/v1

  if [[ "$job" == "synthesis_ready" ]]; then
    wait_for_design_outputs
    print_synthesis_handoff
    return 0
  fi

  printf "\n=== agent_runner design job ready ===\n"
  printf "Job:      %s\n" "$job"
  printf "Lane:     %s\n" "$LABEL"
  printf "Model:    %s\n" "$MODEL"
  printf "Prompt:   %s\n" "$PROMPT"
  printf "Output:   %s\n" "$OUTPUT"
  printf "Run mode: %s\n\n" "$RUN_MODE"

  if artifact_exists "$OUTPUT"; then
    printf "Output already exists; leaving it untouched: %s\n" "$OUTPUT"
    printf "Remove or revise it manually before rerunning this job.\n"
    return 0
  fi

  case "$RUN_MODE" in
    pipe)
      local cmd
      cmd="$(command_for_model "$MODEL")"
      if [[ -z "$cmd" ]]; then
        printf "No command configured for %s. Set CODEX_CMD, CLAUDE_CMD, or GEMINI_CMD.\n" "$MODEL" >&2
        return 2
      fi
      lane_prompt "$job" | bash -lc "$cmd"
      ;;
    print)
      printf "Print mode. To pipe this job now, run:\n\n"
      printf "  AGENT_RUNNER_RUN_MODE=pipe %s run-job %s\n\n" "scripts/agent_runner_tmux_design.sh" "$job"
      printf "To inspect/copy the full lane prompt, run:\n\n"
      printf "  %s print-prompt %s\n\n" "scripts/agent_runner_tmux_design.sh" "$job"
      printf "Configured command for this model would be:\n\n"
      printf "  %s\n\n" "$(command_for_model "$MODEL")"
      printf "=== BEGIN LANE PROMPT ===\n\n"
      lane_prompt "$job"
      printf "\n=== END LANE PROMPT ===\n"
      ;;
    *)
      printf "Unknown AGENT_RUNNER_RUN_MODE: %s\n" "$RUN_MODE" >&2
      return 2
      ;;
  esac
}

print_prompt() {
  local job="$1"
  job_config "$job"
  if [[ "$job" == "synthesis_ready" ]]; then
    print_synthesis_handoff
    return 0
  fi
  lane_prompt "$job"
}

start_session() {
  cd "$AGENT_ROOT"
  mkdir -p docs/design docs/reviews/v1

  if ! command -v tmux >/dev/null 2>&1; then
    printf "tmux is required for start/start-pipe but was not found on PATH.\n" >&2
    printf "You can still run individual jobs with:\n" >&2
    printf "  %s run-job design_claude\n" "scripts/agent_runner_tmux_design.sh" >&2
    printf "  %s run-job design_codex\n" "scripts/agent_runner_tmux_design.sh" >&2
    printf "  %s run-job design_gemini\n" "scripts/agent_runner_tmux_design.sh" >&2
    return 127
  fi

  if tmux has-session -t "$SESSION" 2>/dev/null; then
    printf "tmux session already exists: %s\n" "$SESSION" >&2
    printf "Attach with: tmux attach -t %s\n" "$SESSION" >&2
    return 1
  fi

  local env_prefix
  env_prefix="$(tmux_env_prefix)"

  tmux new-session -d -s "$SESSION" -c "$AGENT_ROOT" -n coordinator \
    "$env_prefix scripts/agent_runner_tmux_design.sh status; printf '\nagent_runner design bootstrap session: %s\nRun mode: %s\n\n' '$SESSION' '$RUN_MODE'; exec ${SHELL:-/bin/bash} -l"

  local job
  while IFS= read -r job; do
    tmux new-window -t "$SESSION" -c "$AGENT_ROOT" -n "${job//_/-}" \
      "$env_prefix scripts/agent_runner_tmux_design.sh run-job '$job'; exec ${SHELL:-/bin/bash} -l"
  done < <(jobs)

  printf "Started tmux session: %s\n" "$SESSION"
  printf "Attach with: tmux attach -t %s\n" "$SESSION"
}

status_artifacts() {
  cd "$AGENT_ROOT"
  local job
  printf "Design input artifacts:\n"
  while IFS= read -r job; do
    job_config "$job"
    if artifact_exists "$OUTPUT"; then
      printf "[x] %s -> %s\n" "$job" "$OUTPUT"
    else
      printf "[ ] %s -> %s\n" "$job" "$OUTPUT"
    fi
  done < <(design_jobs)

  printf "\nNext synthesis artifacts:\n"
  for artifact in \
    docs/design/V1_MVP_DESIGN.md \
    docs/reviews/v1/V1_MVP_DESIGN_REVIEW.md \
    docs/reviews/v1/V1_MVP_FINDINGS_LEDGER.md \
    docs/reviews/v1/V1_MVP_SYNTHESIS.md; do
    if artifact_exists "$artifact"; then
      printf "[x] %s\n" "$artifact"
    else
      printf "[ ] %s\n" "$artifact"
    fi
  done
  printf "[~] docs/SPEC.md (exists now; update after synthesis)\n"
}

next_artifact() {
  cd "$AGENT_ROOT"
  local job
  while IFS= read -r job; do
    job_config "$job"
    if ! artifact_exists "$OUTPUT"; then
      printf "%s -> %s\n" "$job" "$OUTPUT"
      return 0
    fi
  done < <(design_jobs)

  if ! artifact_exists "docs/design/V1_MVP_DESIGN.md"; then
    printf "synthesis_ready -> docs/design/V1_MVP_DESIGN.md\n"
    return 0
  fi

  printf "design bootstrap complete\n"
}

main() {
  case "${1:-}" in
    start)
      start_session
      ;;
    start-pipe)
      RUN_MODE="pipe"
      start_session
      ;;
    run-job)
      [[ $# -eq 2 ]] || { usage >&2; exit 2; }
      run_job "$2"
      ;;
    print-prompt)
      [[ $# -eq 2 ]] || { usage >&2; exit 2; }
      print_prompt "$2"
      ;;
    status)
      status_artifacts
      ;;
    next)
      next_artifact
      ;;
    ""|-h|--help|help)
      usage
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
}

main "$@"
