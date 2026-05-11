# RFC 0028 Dogfood-031 Build Handoff

author: operator

Status: completed
Date: 2026-05-11
Revision: round 3 final allowed cycle iteration

## Summary

Revision round 3 takes the honest V1 path: Striatum does not currently ship a daemon RPC server. RFC 0028 V1 is functionally a shared owner-only registry SQLite plus a foreground sweep loop. CLI and MCP clients open the registry directly under token/capability checks; `striatumd` binds a local socket only as a lifecycle marker. The docs now describe this as registry-backed multi-repo coordination and foreground recovery sweeping, with daemon RPC deferred.

The implementation also closes several tractable review gaps: plaintext `STRIATUM_DAEMON_TOKEN` support is removed; new registrations use realpath/inode-derived repo identity; manual `daemon sweep` is admin-gated and audits denial; aggregate dashboard/MCP reads audit denied requests and carry client id on allowed audit rows; review-only stale requeue events produced by the sweep carry the daemon author payload; and tests now exercise the real `sweep_degraded` timeout path plus a real foreground start/restart path.

## Revision Response (Round 3)

| Finding | Disposition | Notes |
|---|---|---|
| A1 no daemon RPC server | mitigated via docs; deferred to follow-up RFC | `README.md`, `docs/SPEC.md`, `docs/MCP.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `docs/DECISION_LOG.md`, RFC 0028 notes, and the dogfood synthesis now state V1 is registry-backed SQLite plus foreground sweep, not daemon-mediated RPC. |
| A2 unauthenticated `daemon sweep` | fixed | CLI `daemon sweep` now calls `daemon_sweep_once(require_client_auth=True)` and requires admin authorization; denial is audited. |
| A3 underlying recovery bylines | partially fixed; deferred to follow-up RFC | Review-only `recovery.stale_requeued` events triggered by daemon sweep now carry `payload.author = striatumd-<instance-id>`. Broader byline propagation through process reconcile/cancel/blocker internals remains deferred because it requires a wider recovery-event actor model. |
| B1 aggregate read audit attribution | fixed | `dashboard_all`, daemon MCP resource list, and MCP repo list now audit denials and use the authorized client id on allowed aggregate-read rows. |
| B2 `STRIATUM_DAEMON_TOKEN` env support | fixed | `read_runtime_token()` no longer reads plaintext token secrets from environment variables; tests assert the env var is ignored. |
| B3 restart test was SQLite reopen only | fixed | Restart coverage now runs `run_daemon_foreground(max_sweeps=1)` twice and asserts repository/client/capability/segment counts, audit growth, instance id stability, and scheduler cursor survival. |
| B4 timeout branch untested | fixed | Added a real `daemon_sweep_once(per_run_timeout_seconds=-1)` test that asserts the scheduler cursor enters `sweep_degraded` and doctor can surface degraded cursors. |
| B5 doctor duplicate-watcher exception/state coverage | deferred to follow-up RFC | Not changed in this pass beyond existing duplicate watcher coverage. Doctor still needs more explicit non-active repo diagnostics and less broad exception swallowing. |
| B6 removed-id audit breakdown | deferred to follow-up RFC | Existing count remains; per-repository breakdown is still a small follow-up. |
| B7 audit retention/rotation production trigger | mitigated via docs; deferred to follow-up RFC | SPEC, changelog, RFC notes, and synthesis now say production audit retention/rotation is deferred. |
| B8 canonical repo identity | fixed | New repository registrations derive identity from realpath/inode state (`repo root st_dev/st_ino` plus state DB `st_dev/st_ino`), and tests cover duplicate canonical spellings. |
| B9 bootstrap token docs | fixed via docs | SPEC and README now state `daemon start` and `repo add` both bootstrap a runtime admin token when the registry is empty. |
| D1 daemon-mediated wording | fixed via docs | Main docs now say registry-backed mode and direct registry access, not daemon RPC mediation. |
| D2 round-robin wording | fixed via docs | SPEC now distinguishes first registration-order sweep from later last-sweep-time ordering and notes intra-repo ordering remains `runs.created_at`. |
| D3 G9/test overclaim | fixed via tests and docs | Timeout branch and restart coverage were added; remaining fairness/intra-tick scheduling work is deferred rather than claimed closed. |
| D4 byline test only covered wrapper event | partially fixed; deferred to follow-up RFC | The requeue path now carries daemon author payload; process reconcile and other underlying recovery-event bylines remain explicit follow-up scope. |

## Deferred to Follow-Up RFC

A1 remains deferred: V1 has no daemon RPC server, protocol exchange, daemon-mediated CLI transport, HTTP daemon transport, or daemon-side request serialization. The current architecture is an owner-only registry SQLite shared by CLI/MCP clients plus a foreground sweep process.

The remaining follow-ups are: full recovery-event daemon actor propagation for process reconcile, cancel, and blocker events; doctor improvements for non-active/schema-skewed repositories and duplicate-watcher detection failures; per-repository removed-audit-reference diagnostics; production audit retention/rotation/export; HTTP/socket request transport and loopback refusal tests; broader hostile-client payload/replay tests; and intra-repo sweep fairness.

## Files Changed

- `src/striatum/daemon.py` removes env token fallback, uses inode-derived repo identity, audits aggregate reads with caller identity, supports admin-gated manual sweep, and passes daemon author into recovery sweeps.
- `src/striatum/cli/dispatch.py` admin-gates the manual `daemon sweep` route.
- `src/striatum/recovery/auto.py` and `src/striatum/cli/recovery.py` carry optional recovery author metadata into `recovery.stale_requeued` events.
- `src/striatum/mcp.py` lets missing daemon MCP tokens reach daemon routing so denials are audited.
- `tests/test_daemon.py` and `tests/test_daemon_security_adversarial.py` add round-3 regression coverage.
- Docs updated: `README.md`, `CHANGELOG.md`, `docs/SPEC.md`, `docs/MCP.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `docs/DECISION_LOG.md`, `docs/TODO.md`, `docs/rfcs/0028-long-running-daemon-and-multi-repository-control-plane.md`, `docs/rfcs/README.md`, and `docs/dogfood/031/DESIGN_SYNTHESIS.md`.

## Verification Results

- `make install`: passed (`Nothing to be done for 'install'`).
- `make lint`: passed (`ruff check .`).
- `make typecheck`: passed (`mypy`, 93 source files).
- Targeted daemon tests: passed, 22 tests in 12.35s.
- `make test`: passed, 574 tests in 373.56s.
- `make smoke`: passed; emitted only the existing deprecated-`needs` workflow warnings from the smoke fixture.
- Post-handoff doc-link spot check: `tests/test_doc_links.py` passed, 4 tests in 0.02s.
