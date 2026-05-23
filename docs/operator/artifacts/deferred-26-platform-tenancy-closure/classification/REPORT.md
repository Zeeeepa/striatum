---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/ROADMAP.md", "docs/SPEC.md", "docs/DECISION_LOG.md", "docs/rfcs/0028-long-running-daemon-and-multi-repository-control-plane.md", "docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md", "docs/rfcs/0039-go-daemon-core.md", "docs/CLI_REFERENCE.md", "docs/POSTGRES_TRANSITION.md", "docs/USING_STRIATUM.md", "src/striatum/day_zero.py", "src/striatum/daemon_runtime.py", "src/striatum/_daemongo/__init__.py", "src/striatum/cli/parser.py", "go/pkg/admin/bootstrap.go", "tests/test_day_zero.py", "tests/test_daemon_runtime.py", "tests/exit_codes/test_rfc0043_refusals.py", "go/pkg/admin/bootstrap_test.go"]
---

# Deferred 26 Platform Tenancy Classification
author: deferred-26-platform-tenancy-codex-gpt-5-001
date: 2026-05-23
status: split-closure

## Verdict

Deferred item 26 should be closed as a split classification, not reopened as
one implementation task.

Service-manager install/start/status is already current product for Linux
systemd user services and macOS launchd agents. Windows daemon support remains
out of current product. Local multi-operator or multi-OS-user tenancy remains
out of current product under D083 and requires a dedicated RFC before any
implementation.

## Classification

| Surface | Current Status | Evidence | Next Action |
|---|---|---|---|
| Linux/macOS service-manager install | Current product | TODO 58 says `daemon service install/start/status` renders and controls systemd-user or launchd services; CLI reference documents `--manager auto|systemd|launchd`; `src/striatum/day_zero.py` implements the manager resolver, unit/plist rendering, and command wrappers; `tests/test_day_zero.py` covers systemd dry-run and manager command wrapping. | No RFC needed. Keep focused tests green. |
| Windows daemon support | Not current product | SPEC says Windows daemon support is not claimed in V1; RFC 0039 and RFC 0035 list Windows daemon mode as a non-goal/deferred; current runtime path code is Linux/macOS-shaped; the package-data binary resolver documents linux/darwin examples; CLI service-manager choices omit Windows. | Requires a Windows local daemon RFC. |
| Local multi-operator tenancy | Not current product | D083 accepts one OS user per machine for daemon V2 and defers multi-user to a dedicated RFC; RFC 0030 lists multi-user/multi-OS-user access as a non-goal; RFC 0035 says the harness is one OS user per instance; current runtime token is one owner-only local fallback file. | Requires a local multi-operator tenancy RFC. |

## Windows RFC Boundary

A Windows daemon RFC should be deliberately narrow and local-first. It needs
to answer at least:

- transport replacement or compatibility for Unix-socket RPC, including token
  handling and loopback/MCP behavior;
- runtime directory, token-file, pid/lock, and permission semantics on
  Windows;
- Windows Service install/start/status, or an explicit decision to support
  foreground-only Windows first;
- process supervision, PTY/named-pipe behavior, process identity, and
  restart/liveness semantics;
- release packaging and platform slug behavior for Windows binaries;
- CI coverage shape and what can be tested without interactive agent CLIs.

Do not fold this into an incidental service-manager patch. The transport,
permissions, packaging, and supervision semantics are platform boundary work.

## Multi-Operator Tenancy RFC Boundary

A local tenancy RFC should not imply hosted accounts or remote persistence. It
needs to answer at least:

- whether operator identity maps to OS users, local token clients, or both;
- repository-level ACLs for read/write/review/claim/apply/admin/recovery;
- token issuance, expiry, revocation, lockout, and compromised-client recovery;
- audit fields that identify clients without recording transcripts or secrets;
- runtime-token storage when multiple local operators share a machine;
- service-manager and daemon ownership assumptions on shared workstations.

Do not retrofit these semantics into the existing owner-only runtime token.
D083 intentionally kept this out of daemon V2 so the daemon/RPC/Postgres
cutover could finish under a single-user trust boundary.

## Commands Run

```bash
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python -m striatum.cli workflow validate --json docs/operator/workflows/deferred-26-platform-tenancy-closure/workflow.json
```

Result:
`{"data":{"valid":true,"workflow_id":"deferred-26-platform-tenancy-closure"},"ok":true}`.

```bash
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python -m striatum.cli workflow plan --json docs/operator/workflows/deferred-26-platform-tenancy-closure/workflow.json
```

Result: valid plan; 2 claim steps, 2 jobs, 1 edge, 0 cycles.

```bash
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python -m striatum.cli workflow lint --json docs/operator/workflows/deferred-26-platform-tenancy-closure/workflow.json
```

Result: valid, `warning_count: 0`, coverage level `strong`.

```bash
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python - <<'PY'
from pathlib import Path
from striatum.artifact_contracts import validate_artifact_front_matter
items = [
    ('work_plan', Path('docs/operator/plans/deferred-26-platform-tenancy-closure.md')),
    ('synthesis', Path('docs/operator/artifacts/deferred-26-platform-tenancy-closure/classification/REPORT.md')),
    ('synthesis', Path('docs/operator/artifacts/deferred-26-platform-tenancy-closure/final/SUMMARY.md')),
]
for kind, path in items:
    validate_artifact_front_matter(kind=kind, path=path, payload=path.read_bytes())
    print(f'{kind} {path}: ok')
PY
```

Result: work-plan and synthesis front matter valid.

```bash
PYTHONDONTWRITEBYTECODE=1 PYTEST_ADDOPTS='-p no:cacheprovider' PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_day_zero.py tests/test_daemon_runtime.py tests/exit_codes/test_rfc0043_refusals.py::test_daemon_unreachable_message_lists_remediation
```

Result: 17 passed in 0.08s.

```bash
go test ./pkg/admin
```

Result: passed.

```bash
git diff --check -- docs/operator/plans/deferred-26-platform-tenancy-closure.md docs/operator/workflows/deferred-26-platform-tenancy-closure docs/operator/artifacts/deferred-26-platform-tenancy-closure
```

Result: passed.

## Shared-Doc Follow-Up

No shared TODO, roadmap, brief, RFC, decision-log, source, or test edit is
required from this scoped pass. Existing current docs already describe the
landed service-manager surface and the Windows/tenancy non-goals accurately.
