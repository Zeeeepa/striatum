author: implementer-codex-gpt-5.5-001

# Build Handoff: RFC 0030 + RFC 0031 V2 Foundation

Status: implemented
Date: 2026-05-11
Run: run_4e95a7c06d1e414cba6765f5045d4d07

## Summary

Implemented the reviewed dogfood-034 V2 foundation for RFC 0030 and RFC 0031.
The landed slice adds a daemon RPC package with envelope-v1 validation,
newline JSON framing helpers, owner-local transport guards, version handshake,
method registry, capability authorization helpers, and PostgreSQL
request/audit helper wiring. It also adds daemon-owned supervision/apply
foundation state: daemon DB migration v2 for RPC methods, daemon supervisors,
and apply receipts; repo-local migration v13 for supervisor pointers; and
fail-closed sealed-apply helpers.

The threat-model revision pass closed the build review's blocking findings:
sealed-apply denial state now comes from daemon-side key lookup instead of
client params, request ids are checked before audit append, denied and
handshake requests write both audit and request-log rows, handshake state is
per connection, repo-scoped RPC routes bind to the registered repository root,
and supervisor pointers no longer mark a process `attached` before reattach
verification exists.

The implementation keeps repo-local `.striatum/state.sqlite3` as workflow
truth. Direct repo-local CLI mode remains the compatibility path while daemon
routing moves method by method. Daemon MCP remains resources-only; mutation
MCP expansion stays deferred to RFC 0032. Sealed apply is documented as an AI
guardrail and fail-closed authority boundary, not third-party cryptographic
non-repudiation or malicious-local-operator resistance.

## Code Changes

New daemon RPC modules:

- `src/striatum/daemon_rpc/envelope.py`
- `src/striatum/daemon_rpc/handshake.py`
- `src/striatum/daemon_rpc/registry.py`
- `src/striatum/daemon_rpc/capability.py`
- `src/striatum/daemon_rpc/request_log.py`
- `src/striatum/daemon_rpc/server.py`
- `src/striatum/daemon_rpc/framing.py`
- `src/striatum/daemon_rpc/transport_unix.py`
- `src/striatum/daemon_rpc/transport_http.py`
- `src/striatum/daemon_rpc/client.py`

New daemon supervision/apply foundation modules:

- `src/striatum/daemon_supervisor/pointer.py`
- `src/striatum/daemon_apply/apply_service.py`
- `src/striatum/daemon_apply/signing_key.py`

Schema and packaging changes:

- `src/striatum/daemon_pg/sql/0002_rpc_supervision_apply.sql` adds
  `striatumd.rpc_methods`, `striatumd.daemon_supervisors`, and
  `striatumd.apply_receipts`.
- `src/striatum/daemon_pg/migrations.py` advances the daemon DB schema to
  version 2.
- `src/striatum/migrations.py` adds repo-local migration v13 for
  `process_supervisor_pointers`.
- `pyproject.toml` and `src/striatum/__init__.py` bump the package version to
  `1.23.0`.

## Documentation

Updated `README.md`, `CHANGELOG.md`, `docs/SPEC.md`, `docs/MCP.md`,
`docs/CLI_REFERENCE.md`, `docs/HOW_TO_HUMAN.md`,
`docs/UBIQUITOUS_LANGUAGE.md`, `docs/DECISION_LOG.md`, `docs/TODO.md`,
`docs/rfcs/README.md`, RFC 0030, and RFC 0031. The docs mark RFC 0030 and
RFC 0031 as accepted V2 foundation work while preserving the product boundary:
no hosted services, no MCP mutation expansion, no cross-repo workflow mutation,
no Windows daemon claim, and no overclaiming sealed apply.

## Tests

Added `tests/test_daemon_rpc.py` covering:

- envelope-v1 round trip and version-skew refusal;
- `daemon.hello` / `daemon.welcome` handshake behavior;
- method-registry capability declarations and `methods_etag`;
- loopback-only HTTP and owner-only Unix socket guards;
- daemon DB v2 migration contents;
- repo-local supervisor pointer migration.
- request-log/audit behavior for denied calls and `daemon.hello`;
- duplicate request-id refusal before audit append;
- per-connection handshake isolation;
- repo-scope binding refusal;
- sealed-apply refusal using daemon key state, not request input.

Updated `tests/test_workflow_field_errors.py` covering `require_daemon`,
`sealed_patch_provider`, and job-level `apply_gate` validation.

Verification run by the parent session:

- `make install` passed (`Nothing to be done for 'install'` after editable
  install).
- `make lint` passed.
- `make typecheck` passed.
- `make test` passed: 603 tests.
- `make smoke` passed. It emitted the existing deprecation warnings for
  fixture jobs using the `needs` field; no smoke failure.
- Focused checks passed:
  `pytest -q tests/test_daemon_rpc.py tests/test_workflow_field_errors.py`
  and the sealed-patch CLI regression tests.

## Delegation

Used two native explorer sub-agents for read-only parallel inspection:

- workflow-validator/schema placement for `require_daemon`,
  `sealed_patch_provider`, and `apply_gate`;
- RPC/audit/supervision/apply threat-review findings and test placement.

The parent session owned implementation, integration, verification, scope
discipline, and this handoff artifact.

## Deferred Scope

Deferred scope remains as reviewed:

- cross-repository workflow mutation and MCP mutation capability expansion
  remain RFC 0032 scope;
- Python-to-Go daemon core replacement remains future D084 follow-up work;
- bundled or Dockerized PostgreSQL distribution remains deferred;
- direct repo-local CLI retirement remains a future compatibility decision;
- service-manager install and Windows daemon support remain future work.
