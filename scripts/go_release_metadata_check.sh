#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/go"

version="${STRIATUM_RELEASE_VERSION:-dev}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

go build -ldflags "-X main.version=$version" -o "$tmp/striatum" ./cmd/striatum
go build -ldflags "-X main.daemonVersion=$version" -o "$tmp/striatumd" ./cmd/striatumd

cli_version="$("$tmp/striatum" --version)"
if [[ "$cli_version" != "$version" ]]; then
  echo "striatum version mismatch: got $cli_version want $version" >&2
  exit 1
fi

describe="$("$tmp/striatumd" --describe)"
if [[ "$describe" != *"daemon_version=$version"* ]]; then
  echo "striatumd describe missing daemon_version=$version: $describe" >&2
  exit 1
fi
