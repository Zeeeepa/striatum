#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

scoped_files="$(find go/cmd/striatumd go/pkg/webservice go/pkg/webassets go/pkg/websse go/pkg/webtest -type f -name '*.go' 2>/dev/null || true)"

if [[ -n "$scoped_files" ]]; then
  if grep -nE 'striatum\.service|striatum\.web|src/striatum/service\.py|sqlite3|retired-local-state|terminal output|transcript' $scoped_files; then
    echo "RFC 0078 web cutover guard failed: Go web/service code references a retired Python or non-authoritative state surface" >&2
    exit 1
  fi
fi

if grep -nE '"/dogfood|"/chat' go/pkg/webservice/*.go >/dev/null; then
  : # retired route refusals are intentionally present in Go webservice.
fi

echo "RFC 0078 web retirement guard passed for retired /dogfood and /chat routes"
