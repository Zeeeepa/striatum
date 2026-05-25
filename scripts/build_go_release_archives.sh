#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$ROOT/dist"
TARGETS=("linux-amd64" "linux-arm64" "darwin-amd64" "darwin-arm64")

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dist)
      DIST="$2"
      shift 2
      ;;
    --target)
      TARGETS=("$2")
      shift 2
      ;;
    *)
      echo "usage: $0 [--dist DIR] [--target os-arch]" >&2
      exit 2
      ;;
  esac
done

VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
GIT_SHA="$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
GIT_DIRTY="$(
  if git -C "$ROOT" diff --quiet --ignore-submodules HEAD -- 2>/dev/null; then
    echo clean
  else
    echo dirty
  fi
)"

rm -rf "$DIST"
mkdir -p "$DIST"

for target in "${TARGETS[@]}"; do
  os="${target%-*}"
  arch="${target#*-}"
  case "$os/$arch" in
    linux/amd64|linux/arm64|darwin/amd64|darwin/arm64) ;;
    *) echo "unsupported release target: $target" >&2; exit 2 ;;
  esac

  (
    cd "$ROOT/go"
    GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
      go build -ldflags "-X main.version=$VERSION" \
      -o "bin/striatum-$target" ./cmd/striatum
    GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
      go build -ldflags "-X main.daemonVersion=$VERSION -X main.buildGitSHA=$GIT_SHA -X main.buildDirty=$GIT_DIRTY" \
      -o "bin/striatumd-$target" ./cmd/striatumd
    GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
      go build -ldflags "-X main.version=$VERSION" \
      -o "bin/striatum-supervisor-helper-$target" ./cmd/striatum-supervisor-helper
  )

  stage="$DIST/striatum_${VERSION}_${target}"
  mkdir -p "$stage/bin"
  cp "$ROOT/go/bin/striatum-$target" "$stage/bin/striatum"
  cp "$ROOT/go/bin/striatumd-$target" "$stage/bin/striatumd"
  cp "$ROOT/go/bin/striatum-supervisor-helper-$target" "$stage/bin/striatum-supervisor-helper"
  chmod 0755 "$stage/bin/striatum" "$stage/bin/striatumd" "$stage/bin/striatum-supervisor-helper"
  cp "$ROOT/LICENSE" "$ROOT/README.md" "$ROOT/VERSION" "$stage/"
  cat > "$stage/INSTALL.txt" <<EOF
Install Striatum $VERSION by copying bin/striatum, bin/striatumd, and
bin/striatum-supervisor-helper to a directory on PATH.

Striatum live state requires daemon-owned PostgreSQL. Configure
STRIATUM_DAEMON_DB_URL or the daemon config before starting striatumd.
EOF
  tar -C "$DIST" -czf "$DIST/striatum_${VERSION}_${target}.tar.gz" "striatum_${VERSION}_${target}"
  rm -rf "$stage"
done

(
  cd "$DIST"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 ./*.tar.gz > SHA256SUMS
  else
    sha256sum ./*.tar.gz > SHA256SUMS
  fi
)

echo "built Go release archives in $DIST"
