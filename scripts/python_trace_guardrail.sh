#!/usr/bin/env bash
set -euo pipefail

mode="report"
format="markdown"
limit=40

usage() {
  cat <<'USAGE'
Usage: scripts/python_trace_guardrail.sh [--report|--strict] [--format markdown|tsv] [--limit N]

Scans tracked repository files for RFC 0078 Python traces.

Modes:
  --report   Print classifications and exit 0 even when blocked traces exist.
  --strict   Exit 1 when blocked or unclassified traces exist.

Formats:
  markdown   Human-readable summary and bounded finding samples.
  tsv        One row per finding:
             classification<TAB>class<TAB>path<TAB>line<TAB>evidence
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --report)
      mode="report"
      shift
      ;;
    --strict)
      mode="strict"
      shift
      ;;
    --format)
      format="${2:-}"
      shift 2
      ;;
    --limit)
      limit="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "$format" != "markdown" && "$format" != "tsv" ]]; then
  echo "--format must be markdown or tsv" >&2
  exit 2
fi

if ! [[ "$limit" =~ ^[0-9]+$ ]]; then
  echo "--limit must be a non-negative integer" >&2
  exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

declare -a rows=()
declare -A counts=()
declare -A class_counts=()

add_row() {
  local classification="$1"
  local class="$2"
  local path="$3"
  local line="${4:-}"
  local evidence="${5:-}"
  if [[ -z "$line" ]]; then
    line="-"
  fi

  rows+=("${classification}"$'\t'"${class}"$'\t'"${path}"$'\t'"${line}"$'\t'"${evidence}")
  counts["$classification"]=$(( ${counts["$classification"]:-0} + 1 ))
  class_counts["$classification|$class"]=$(( ${class_counts["$classification|$class"]:-0} + 1 ))
}

is_current_guidance_path() {
  local path="$1"
  case "$path" in
    README.md|AGENTS.md|Makefile)
      return 0
      ;;
    .github/workflows/*|scripts/*)
      return 0
      ;;
    docs/ADOPTER_READING_PATH.md|docs/DOC_MAP.md|docs/FRONTEND_DEVELOPMENT.md)
      return 0
      ;;
    docs/INDEX.md|docs/SPEC.md|docs/TODO.md|docs/ROADMAP.md|docs/RELEASING.md)
      return 0
      ;;
    docs/DECISION_LOG.md|docs/UBIQUITOUS_LANGUAGE.md)
      return 0
      ;;
    docs/USING_STRIATUM.md|docs/GETTING_STARTED.md|docs/POSTGRES_TRANSITION.md)
      return 0
      ;;
    docs/CLI_REFERENCE.md|docs/MCP.md|docs/HOW_TO_AGENT.md|docs/HOW_TO_HUMAN.md)
      return 0
      ;;
    docs/WORKFLOW_TYPES.md|docs/WORKFLOW_CATALOG.md|docs/WRITING_WORKFLOWS.md)
      return 0
      ;;
    docs/operator/BRIEF.md)
      return 0
      ;;
    docs/architecture/COMMAND_AUTHORITY_MATRIX.md|docs/architecture/DAEMON_METHOD_TABLES.md)
      return 0
      ;;
    src/striatum/skills/templates/*|src/striatum/plugins/templates/*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

is_historical_or_provenance_path() {
  local path="$1"
  case "$path" in
    docs/rfcs/*|docs/reviews/*|docs/dogfood/*|docs/operator/artifacts/*)
      return 0
      ;;
    docs/operator/plans/*|docs/operator/workflows/*|docs/handoffs/*|docs/research/*)
      return 0
      ;;
    docs/ENGRAM_INCUBATION_CONTEXT.md|docs/RFC_0014_DOGFOOD_FIX_SPEC.md)
      return 0
      ;;
    prompts/*|examples/rfc-0014-operational-artifact-home/*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

is_target_workload_path() {
  local path="$1"
  case "$path" in
    examples/*|docs/issues/*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

while IFS= read -r path; do
  case "$path" in
    src/*.py|src/*.pyi)
      add_row "blocked" "active_striatum_python_source" "$path" "" "Striatum runtime source must be ported, retired, or deleted before RFC 0078 closure."
      ;;
    tests/*.py|tests/*.pyi)
      add_row "blocked" "active_pytest_surface" "$path" "" "Active Python tests keep pytest as a required validation surface."
      ;;
    scripts/*.py|scripts/*.pyi)
      add_row "blocked" "active_python_script" "$path" "" "Tracked Python scripts are active Striatum tooling until replaced or retired."
      ;;
    pyproject.toml)
      add_row "blocked" "active_python_packaging" "$path" "" "pyproject.toml still owns Python packaging, console scripts, and pytest/ruff/mypy configuration."
      ;;
    *.pyc|*__pycache__*)
      add_row "blocked" "tracked_python_cache" "$path" "" "Python cache artifacts must never be tracked."
      ;;
  esac
done < <(git ls-files)

guidance_pattern='(pytest|mypy|ruff|pip install|python[0-9]? -m striatum|python[0-9]? -m pip|venv|wheel|sdist|striatum-orchestrator)'

while IFS= read -r match; do
  path="${match%%:*}"
  rest="${match#*:}"
  line="${rest%%:*}"
  evidence="${rest#*:}"
  if is_current_guidance_path "$path"; then
    add_row "blocked" "active_python_runtime_guidance" "$path" "$line" "$evidence"
  elif is_historical_or_provenance_path "$path"; then
    add_row "historical_provenance" "archived_python_reference" "$path" "$line" "$evidence"
  elif is_target_workload_path "$path"; then
    add_row "target_workload_allowed" "target_repo_python_reference" "$path" "$line" "$evidence"
  else
    add_row "unclassified" "unclassified_python_reference" "$path" "$line" "$evidence"
  fi
done < <(
  git grep -I -n -i -E "$guidance_pattern" -- \
    README.md AGENTS.md Makefile .github scripts docs examples prompts \
    src/striatum/skills/templates src/striatum/plugins/templates \
    ':(exclude)scripts/python_trace_guardrail.sh' \
    2>/dev/null || true
)

blocked_count="${counts["blocked"]:-0}"
unclassified_count="${counts["unclassified"]:-0}"

if [[ "$format" == "tsv" ]]; then
  printf 'classification\tclass\tpath\tline\tevidence\n'
  printf '%s\n' "${rows[@]}"
else
  printf '# Python Trace Guardrail Report\n\n'
  printf 'Mode: `%s`\n\n' "$mode"
  printf '| Classification | Count |\n'
  printf '|---|---:|\n'
  printf '| blocked | %d |\n' "$blocked_count"
  printf '| unclassified | %d |\n' "$unclassified_count"
  printf '| historical_provenance | %d |\n' "${counts["historical_provenance"]:-0}"
  printf '| target_workload_allowed | %d |\n\n' "${counts["target_workload_allowed"]:-0}"

  printf '## Class Counts\n\n'
  printf '| Classification | Class | Count |\n'
  printf '|---|---|---:|\n'
  for key in "${!class_counts[@]}"; do
    printf '%s\t%s\t%d\n' "${key%%|*}" "${key#*|}" "${class_counts[$key]}"
  done | sort | while IFS=$'\t' read -r classification class count; do
    printf '| %s | %s | %d |\n' "$classification" "$class" "$count"
  done
  printf '\n'

  printf '## Finding Samples\n\n'
  printf '| Classification | Class | Path | Line | Evidence |\n'
  printf '|---|---|---|---:|---|\n'
  printed=0
  for row in "${rows[@]}"; do
    if [[ "$printed" -ge "$limit" ]]; then
      break
    fi
    IFS=$'\t' read -r classification class path line evidence <<< "$row"
    evidence="${evidence//|/\\|}"
    printf '| %s | %s | `%s` | %s | %s |\n' "$classification" "$class" "$path" "${line:-}" "$evidence"
    printed=$(( printed + 1 ))
  done
  if [[ "${#rows[@]}" -gt "$limit" ]]; then
    printf '\n_Showing %d of %d findings. Use `--format tsv` for the complete list._\n' "$limit" "${#rows[@]}"
  fi
fi

if [[ "$mode" == "strict" && ( "$blocked_count" -gt 0 || "$unclassified_count" -gt 0 ) ]]; then
  echo "python-trace guardrail failed: blocked=${blocked_count} unclassified=${unclassified_count}" >&2
  exit 1
fi

exit 0
