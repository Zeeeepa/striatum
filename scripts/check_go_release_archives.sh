#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT/scripts/smoke_common.sh"

DIST="${1:-$ROOT/dist}"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"

test -f "$DIST/SHA256SUMS" || {
  echo "missing checksum manifest: $DIST/SHA256SUMS" >&2
  exit 1
}

(
  cd "$DIST"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 -c SHA256SUMS
  else
    sha256sum -c SHA256SUMS
  fi
)

found=0
host_triple="$(smoke_host_triple 2>/dev/null || true)"
executed_host=0
for archive in "$DIST"/striatum_"$VERSION"_*.tar.gz; do
  [[ -f "$archive" ]] || continue
  found=$((found + 1))
  base="$(basename "$archive")"
  target="${base#striatum_${VERSION}_}"
  target="${target%.tar.gz}"
  tmp="$(mktemp -d)"
  root="$(smoke_extract_archive "$archive" "$tmp")"
  test -x "$root/bin/striatum"
  test -x "$root/bin/striatumd"
  test -x "$root/bin/striatum-supervisor-helper"
  if [[ -n "$host_triple" && "$target" == "$host_triple" ]]; then
    "$root/bin/striatum" --version | grep -Fx "$VERSION" >/dev/null
    "$root/bin/striatumd" --describe | grep -F "daemon_version=$VERSION" >/dev/null
    "$root/bin/striatumd" --describe | grep -E "migration_count=[1-9][0-9]*" >/dev/null
    "$root/bin/striatum-supervisor-helper" --version | grep -Fx "$VERSION" >/dev/null
    executed_host=1
  fi
  rm -rf "$tmp"
done

if [[ "$found" -eq 0 ]]; then
  echo "no Go release archives found in $DIST" >&2
  exit 1
fi

if [[ -n "$host_triple" && "$executed_host" -eq 0 ]]; then
  echo "no host-compatible archive found for runtime smoke: $host_triple" >&2
  exit 1
fi

echo "checked $found Go release archive(s)"
