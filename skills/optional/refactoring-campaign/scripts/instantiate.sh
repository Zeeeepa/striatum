#!/usr/bin/env bash
# Instantiate the refactoring-campaign workflow chain from the bundled
# example into a target repository, with campaign-scoped artifact roots
# and unique workflow ids. Idempotence: refuses to overwrite an existing
# campaign directory.
set -euo pipefail

usage() {
  cat <<'EOF'
usage: instantiate.sh --slug <kebab-slug> [--target-repo <path>] [--source <path>]

Copies examples/refactoring-campaign/stage-* into
  <target>/striatum/workflows/refactoring-campaign-<slug>/
rewriting:
  examples/refactoring-campaign/artifacts/  ->  striatum/refactoring/<slug>/
  workflow_id                               ->  refactoring-campaign-<slug>-stage-N
  branch suggested_name                     ->  striatum/refactoring-campaign-<slug>
then validates each stage workflow.

--source defaults to the examples/refactoring-campaign directory of the
striatum checkout this script lives in (symlink-safe).
EOF
}

SLUG=""
TARGET="$PWD"
SOURCE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --slug) SLUG="${2:?--slug needs a value}"; shift 2 ;;
    --target-repo) TARGET="${2:?--target-repo needs a value}"; shift 2 ;;
    --source) SOURCE="${2:?--source needs a value}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[ -n "$SLUG" ] || { echo "error: --slug is required" >&2; exit 2; }
case "$SLUG" in
  *[!a-z0-9-]*) echo "error: slug must be kebab-case [a-z0-9-]" >&2; exit 2 ;;
esac
[ -d "$TARGET" ] || { echo "error: target repo not found: $TARGET" >&2; exit 2; }

if [ -z "$SOURCE" ]; then
  SCRIPT_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")" && pwd)"
  SOURCE="$(readlink -f "$SCRIPT_DIR/../../../../examples/refactoring-campaign")"
fi
[ -d "$SOURCE/stage-0-goal-selection" ] || {
  echo "error: source example not found at: $SOURCE" >&2; exit 2; }

DEST="$TARGET/striatum/workflows/refactoring-campaign-$SLUG"
[ ! -e "$DEST" ] || { echo "error: campaign already exists: $DEST" >&2; exit 2; }

mkdir -p "$DEST"
cp -R "$SOURCE"/stage-0-goal-selection "$SOURCE"/stage-1-plan-gate \
      "$SOURCE"/stage-2-execution "$DEST"/

# Campaign-scoped artifact root (workflow.json, prompts, and roles all
# reference artifact paths).
find "$DEST" -type f \( -name '*.json' -o -name '*.md' \) -exec sed -i \
  -e "s|examples/refactoring-campaign/artifacts/|striatum/refactoring/$SLUG/|g" {} +

# Unique workflow ids and campaign branch per stage.
for wf in "$DEST"/stage-*/workflow.json; do
  sed -i \
    -e "s|\"workflow_id\": \"refactoring-campaign-|\"workflow_id\": \"refactoring-campaign-$SLUG-|" \
    -e "s|\"suggested_name\": \"striatum/refactoring-campaign\"|\"suggested_name\": \"striatum/refactoring-campaign-$SLUG\"|" \
    "$wf"
done

FAIL=0
for wf in "$DEST"/stage-*/workflow.json; do
  printf '%s: ' "$wf"
  striatum --repo "$TARGET" workflow validate --allow-same-model-pairing "$wf" || FAIL=1
done
[ "$FAIL" -eq 0 ] || { echo "error: a stage failed validation" >&2; exit 1; }

cat <<EOF

Instantiated: $DEST
Artifact root: striatum/refactoring/$SLUG/
Next: bind real lanes in each stage workflow.json (REFERENCE.md), then
      validate again and 'run prepare' stage 0 (or author GOAL_DECISION.md
      directly if the operator already named the goal).
EOF
