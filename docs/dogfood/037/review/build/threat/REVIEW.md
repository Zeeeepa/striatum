---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["threat_model", "rfc-0035", "multi-repo-test-harness", "build"]
---

author: reviewer-claude-opus-001

# Threat-Model Review: RFC 0035 Multi-Repo Test Harness V1 Build

Scope: implementation under `tests/_harness/`, `tests/test_multi_repo_harness.py`, the five `tests/test_cross_repo_*_e2e.py` + `tests/test_mcp_capability_scope_e2e.py` + `tests/test_per_repo_write_scope_e2e.py` files, `tests/conftest.py`, `Makefile` `test-multi-repo` target, and the docs/TODO/CHANGELOG updates landed on `striatum/dogfood-037-rfc-0035-multi-repo-test-harness`.

This review is fresh-context and posture-restricted to threat-model: do the e2e tests actually exercise the threat surfaces RFC 0032 V2 introduced, and is the harness deterministic + cleanly isolated?

## Verdict

`accept_with_findings`. The V1 slice ships in coherent form: the harness module skeleton, the five e2e test files, the smoke test, `make test-multi-repo`, and the per-test reset semantics all line up with the design synthesis. Three non-blocking ergonomic / packaging findings recorded below.

## What the build gets right (threat-model lens)

- **Capability scope mismatch is exercised, not stubbed.** `tests/test_mcp_capability_scope_e2e.py:5` covers `test_repo_scoped_write_token_allows_repo_a_write_and_audits_allowed` + `test_repo_scoped_write_token_denied_against_repo_b_and_audits_scope_mismatch`. The denial test asserts both the refusal AND the audit row records the scope mismatch — exactly the threat surface the synthesis claimed.
- **Default-deny on unknown methods.** `test_unknown_method_denied_and_audited` asserts the standard unknown-method error + audit row with documented denial vocabulary.
- **Token lifecycle covered.** `test_revoked_token_denied_and_audited`, `test_expired_token_denied_and_audited`, and `test_read_only_token_cannot_call_write_tool` exercise the three documented denial paths.
- **`tools/list` capability filtering covered.** `test_read_only_token_lists_only_read_tools` asserts the filtering.
- **Audit chain continuity covered.** `test_audit_chain_continuous_across_allowed_and_denied_calls` asserts the hash chain links across mixed allow + deny calls — the strongest single test for the chain integrity claim.
- **Crash recovery covered.** `tests/test_cross_repo_crash_recovery_e2e.py` has 4 test functions covering SIGKILL-mid-prepare/start/cancel + one-repo-unreachable scenarios.
- **Per-repo write-scope covered.** `tests/test_per_repo_write_scope_e2e.py` has 4 test functions exercising validator-time and runtime refusal paths.
- **Per-test DB reset is honest.** `tests/_harness/pg.py:112` uses `TRUNCATE ... RESTART IDENTITY CASCADE` over every daemon DB table except the schema-version row. Audit chain rows are cleared cleanly between tests; chain continuity assertions cannot leak across functions.
- **No parallel production-code path.** Spot-checked `tests/_harness/multi_repo.py`, `mcp.py`, `tokens.py`, `audit.py` — each is a thin helper over the production daemon RPC + capability + audit code paths. Helpers compose calls to `DaemonRpcServer`, the existing token issuance path, and the existing audit-row reader; nothing bypasses capability gating or directly injects audit rows.
- **Skip-when-no-PG path is clean.** `tests/_harness/pg.py:33-53` calls `pytest.skip(...)` with two distinct messages: one for missing psycopg, one for unreachable PG. Both messages name the remediation (`install the daemon-pg extra and run make pg-test` / `STRIATUM_TEST_POSTGRES_URL` env var). I exercised the no-psycopg skip path locally; 6 tests in the smoke file skip cleanly with the clear remediation message.
- **Subprocess + Unix socket means no port collision.** `tests/_harness/daemon.py` boots the daemon with an ephemeral socket path under the harness scratch directory; back-to-back instances cannot interfere.
- **Deterministic cleanup.** `MultiRepoHarness.stop()` drops the ephemeral database, removes the scratch dir, and deletes the Unix socket. The `tests/test_multi_repo_harness.py` smoke covers the start/register/reset/stop lifecycle in 6 test functions including back-to-back instantiation.
- **Pytest marker registered.** The `multi_repo` marker is registered (no `PytestUnknownMarkWarning`), and `make test-multi-repo` filters by marker.
- **Scope discipline honest.** Deferred items (Go-client testing surface, examples workflow, Docker PG, Windows daemon, cross-machine, performance) are all documented in BUILD_HANDOFF.

## Non-blocking findings (severity low)

These do not block the V1 ship. Recorded so the next bugfix iteration can fold them in opportunistically.

1. **psycopg is not in dev extras by default.** A developer running `make install` followed by `make test-multi-repo` will see all 32 harness tests silently skip (with clear messages, to be fair). Concrete suggestion: add a `dev-pg` extras group in `pyproject.toml` that pulls `daemon-pg` so `pip install -e .[dev,dev-pg]` enables the harness path. Or document the two-extras install (`-e .[dev,daemon-pg]`) in the README "Install" section + `docs/HOW_TO_HUMAN.md`. Currently the install path is implicit.

2. **`make test-multi-repo` lacks a `--skip-on-no-pg` informational header.** When PG is unavailable, the operator sees `32 skipped` with no top-of-output guidance about how to enable it. Concrete suggestion: the Makefile target could check for psycopg+PG availability and print a clear "harness skipped because <reason>; remediation: <command>" before invoking pytest. Implementer choice.

3. **Crash-recovery tests use SIGKILL on the daemon subprocess.** The synthesis flagged the determinism caveat (SIGKILL can race with in-flight transactions). I read the four crash-recovery test bodies and they correctly poll daemon state after restart rather than racing for a specific transition timing, but the comments don't call out the polling pattern. Concrete suggestion: add a one-line `# polls daemon state after restart; SIGKILL races are inherent — see RFC 0035 §Open Questions` comment so future readers understand why the test isn't tight on timing.

## Out of scope per posture

The threat_model posture evaluates whether the harness will exercise RFC 0032 V2's threat surfaces end-to-end. **Out of scope** for this review:

- Performance / load characteristics (RFC 0035 §Non-Goals).
- Cross-machine testing (D083).
- Windows daemon harness (RFC 0030 V2 scope).
- Go-client testing surface (D084 future).
- Docker-based ephemeral Postgres (separate hardening RFC).
- Malicious-local-root resistance (RFC 0031 threat model).

## Implementation passes the bar

The five e2e test files match the threat-surface coverage matrix from the synthesis 1:1. The harness itself is well-isolated (subprocess + Unix socket + ephemeral DB), cleans up deterministically, and skips cleanly when PG is unavailable. The audit-row inspection helper composes the production audit-reader path; no test helper bypasses capability gating or audit append.

## Acceptance bar checklist

- [x] Capability scope mismatch refused with documented denial + audit row
- [x] Default-deny on unknown methods + missing capability
- [x] Token revoked + expired denial paths covered
- [x] `tools/list` capability-filtered
- [x] Audit chain continuity across mixed allow/deny calls
- [x] Crash recovery (SIGKILL mid-prepare/start/cancel) + one-repo-unreachable
- [x] Per-repo write-scope (validator-time + runtime)
- [x] Per-test DB reset truncates the right tables
- [x] Ephemeral DB + scratch dir + Unix socket cleaned up on stop
- [x] Subprocess + Unix socket means no port collisions
- [x] No parallel production-code path
- [x] Skip-when-no-PG path is clean with named remediation
- [x] `make test-multi-repo` filters by `multi_repo` pytest marker
- [x] Documentation honest: TODO Open item 19 marked done; SPEC + CHANGELOG updated; no overclaim of cross-machine / Windows / Docker / Go support
