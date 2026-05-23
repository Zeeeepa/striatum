---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/ROADMAP.md", "docs/DECISION_LOG.md", "docs/rfcs/0049-interactive-claude-lane-mcp-control-plane.md", "docs/rfcs/0050-go-daemon-http-sse-mcp.md", "docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md", "docs/MCP.md", "contracts/daemon_methods.json", "src/striatum/scaffold/templates/bin/claude-supervised-wrapper.sh", "go/pkg/agentloop/bootstrap.go", "go/pkg/agentloop/loop.go"]
---

# RFC 0049 Deferred Item 14 Classification
author: rfc0049-classifier-codex-gpt-5-001
date: 2026-05-23
status: shelved

## Verdict

Deferred item 14 remains **shelved**. It is not reopened and not closed as
implemented.

D106 is still the controlling decision: RFC 0049 is a capability experiment,
not active backlog. Current RFC 0050 and RFC 0075 work has landed generic MCP,
agent-loop, tmux, and liveness prerequisites, but it does not satisfy RFC
0049's Claude-specific acceptance criteria: a measured real interactive Claude
lane, billing attribution proof, `long_lived` lifecycle support, or a
documented `fresh_strategy` policy.

## Evidence

| Question | Evidence | Result |
|---|---|---|
| Does current status already say shelved? | `docs/TODO.md` item 37, `docs/ROADMAP.md` section 5.5, `docs/rfcs/README.md`, and RFC 0049 all cite D106 and mark the item shelved. | Status text is current. |
| Did newer MCP work reopen the item? | RFC 0050 Phase D/E and `docs/MCP.md` describe generic daemon HTTP MCP, `work.await_packet`, `session.report`, and agent-loop PTY bootstrap. | Generic prerequisites landed, not a Claude-specific reopen. |
| Did newer tmux work reopen the item? | RFC 0075 remains proposed; the plan says tmux metadata and liveness are partial. RFC 0077 liveness and tmux attach metadata help observability but do not prove interactive Claude billing or behavior. | No reopen condition met. |
| Is `await_packet` implemented? | `contracts/daemon_methods.json` and `src/striatum/daemon_rpc/daemon_methods.json` register `work.await_packet`; Go mutations register the handler. | Implemented generically. |
| Is pre-work session reporting implemented? | The same method contracts register `session.report`; Go lifecycle code validates ready/heartbeat/question/escalate reports. | Implemented generically. |
| Is the Claude lane now long-lived? | Source search found no `long_lived`, `fresh_strategy`, `claude_code.lifecycle`, or exact `"per_packet"` lifecycle fields in `src/`, `tests/`, `go/`, or `contracts/`. | Not implemented. |
| Does the committed Claude wrapper still use print mode? | `src/striatum/scaffold/templates/bin/claude-supervised-wrapper.sh` and `.striatum/bin/claude-supervised-wrapper.sh` still document and invoke fresh per-packet `claude --print` with the v1.48.1 auth flags. | Per-packet wrapper remains the current path. |
| Have external billing terms materially changed? | Official Claude Help Center article checked on 2026-05-23 still describes June 15, 2026 Agent SDK credits covering `claude -p`, Max 20x at $200/month, and interactive Claude Code in terminal/IDE staying on subscription usage. It also recommends API-key billing for shared production automation. | Economic motivation remains, but billing intent for supervised PTY automation is still not local product evidence. |

External billing source checked:
<https://support.claude.com/en/articles/15036540-use-the-claude-agent-sdk-with-your-claude-plan>

## Classification

Use **shelved** for the product item.

Do not mark RFC 0049 closed/done, because the implementation-specific fields
and real Claude spike evidence are absent. Do not mark it reopened, because
the D106 revisit triggers are not satisfied: no explicit operator-funded
spike, no supported PTY-supervised billing answer, and no material billing
change that removes the need for a spike.

RFC 0050 / RFC 0075 should continue independently as generic MCP/tmux
infrastructure. If an operator later funds the Claude-specific spike, it should
start from RFC 0049 Phase A with measurable success criteria and should not
change core docs from this closure pass alone.

## Commands Run

```bash
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python -m striatum.cli workflow validate --json docs/operator/workflows/deferred-14-rfc0049-closure/workflow.json
```

Result:
`{"data":{"valid":true,"workflow_id":"deferred-14-rfc0049-closure"},"ok":true}`.

```bash
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_claude_supervised_wrapper.py tests/architecture/test_tmux_authority_boundary.py tests/test_dashboard_rfc0075.py
```

Result: 16 passed in 1.24s.

```bash
go test ./pkg/agentloop ./pkg/supervisor ./pkg/sessionliveness ./pkg/mcp
```

Result: passed for all four packages.

```bash
PYTHONDONTWRITEBYTECODE=1 PYTEST_ADDOPTS='-p no:cacheprovider' PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_mcp_fake_agent_loop_e2e.py::test_fake_mcp_agent_completes_work_packet_loop
```

Result: 1 passed in 2.03s.

```bash
rg -n '"long_lived"|fresh_strategy|claude_code\.lifecycle|lifecycle.*long_lived|"per_packet"' src tests go contracts || true
```

Result: no matches.

```bash
rg -n 'claude --print|--permission-mode acceptEdits|--allowedTools "Bash"' src/striatum/scaffold/templates/bin/claude-supervised-wrapper.sh .striatum/bin/claude-supervised-wrapper.sh tests/test_claude_supervised_wrapper.py
```

Result: wrapper template and installed wrapper still show `claude --print`
with the v1.48.1 auth flags.

```bash
rg -n '"method": "session.report"|"method": "work.await_packet"' contracts/daemon_methods.json src/striatum/daemon_rpc/daemon_methods.json
```

Result: both generic MCP methods are registered in both method-contract copies.

```bash
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python - <<'PY'
from pathlib import Path
from striatum.artifact_contracts import validate_artifact_front_matter
items = [
    ('work_plan', Path('docs/operator/plans/deferred-14-rfc0049-closure.md')),
    ('synthesis', Path('docs/operator/artifacts/deferred-14-rfc0049-closure/classification/REPORT.md')),
    ('synthesis', Path('docs/operator/artifacts/deferred-14-rfc0049-closure/final/SUMMARY.md')),
]
for kind, path in items:
    validate_artifact_front_matter(kind=kind, path=path, payload=path.read_bytes())
    print(f'{kind} {path}: ok')
PY
```

Result: work-plan and synthesis front matter valid.

```bash
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_doc_links.py
```

Result: 7 passed in 0.11s.

```bash
git diff --check -- docs/operator/plans/deferred-14-rfc0049-closure.md docs/operator/workflows/deferred-14-rfc0049-closure docs/operator/artifacts/deferred-14-rfc0049-closure
```

Result: passed.

## Shared-Doc Follow-Up

No shared status update is needed from this scoped pass. The existing TODO,
roadmap, RFC index, RFC 0049, and brief posture are consistent with the source
and current external billing evidence.
