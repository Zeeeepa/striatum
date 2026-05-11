---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["threat_model", "rfc-0030", "rfc-0031", "daemon", "build", "round-2"]
---

author: reviewer-claude-opus-002

# Threat-Model Review of RFC 0030 + RFC 0031 V2 Build — Round 2

Run: run_4e95a7c06d1e414cba6765f5045d4d07
Date: 2026-05-11
Posture: threat_model (over-eager AI + operator-mistake; malicious-local-root is out of scope)
Prior review: round-1, reviewer-claude-opus-001, verdict `needs_revision` (severity high, F1–F12).

## Summary

The implement_a2 pass closes every must-fix item the round-1 review raised
(F1–F7) with code changes that visibly carry the invariant rather than
shifting the responsibility into doc text. Sealed-apply denial vocabulary is
now driven by `daemon_apply.signing_key.key_loaded()` rather than by client-
supplied params; capability `repository_id` is resolved against
`striatumd.repositories` and refused unless it matches the router's bound
root; both the audit row and the request-log row are written on every
terminal response including denied calls and successful or malformed
handshakes; duplicate `request_id` is detected before audit append; handshake
state is per-connection; and the repo-local supervisor-pointer helper refuses
to record `attached` state at all, so a foundation slice cannot mark a
zombie or PID-reused process as supervised before the reattach loop ships.
SPEC.md no longer claims "stronger supervision" — only "V2 schema/API
foundation for future stronger supervision."

The should-fix list is largely closed too. `daemon.welcome.sealed_apply`
fields now reflect daemon state. The workflow validator accepts/rejects
`require_daemon`, `sealed_patch_provider`, and `apply_gate` with positive
and negative tests. `signing_key.py` has a real keyring branch with a
`0600`-mode fallback, the env var is renamed
`STRIATUM_DAEMON_SIGNING_KEY_PATH`, and the UBIQUITOUS_LANGUAGE entry
matches code behavior. SPEC.md is honest that V2 client-side routing has not
yet replaced direct repo-local CLI mode.

Two should-fix items remain as observations rather than blockers. (1) The
closed denial-vocabulary list (`base_tree_mismatch`, `patch_digest_mismatch`,
`verdict_digest_mismatch`, `verdict_run_mismatch`, `verdict_not_accepting`,
`write_scope_violation`, `apply_token_missing`) is still untested at the
route boundary; the stub only emits `sealed_key_missing` and
`apply_gate_unsatisfied`, so a table-driven assertion would not catch much
today, but the test debt should land with the full apply path so the
vocabulary cannot drift silently. (2) The duplicate-request guard does its
pre-check outside the audit/request-log insert transaction, leaving a
narrow race window once the accept loop ships concurrent handlers; today's
single-thread skeleton does not exercise it, but it is worth tightening
when the transport lands.

No new must-fix surface emerges from the round-2 revisions.

Verdict: `accept_with_findings`.

## Round-1 Findings Status

### F1 — Client-controlled denial vocabulary on `apply.reviewed_patch` — CLOSED

`src/striatum/daemon_apply/apply_service.py:12,17-20` now imports
`key_loaded` from `daemon_apply.signing_key` and calls it directly:
`if not key_loaded(): raise RpcError("sealed_key_missing", ...)`. The
client-supplied `signing_key_loaded` param is no longer read. Coverage:
`tests/test_daemon_rpc.py:97-103` passes `{"signing_key_loaded": True}`
with `monkeypatch` returning `key_loaded=False` and asserts the response is
`sealed_key_missing` regardless of client input.

### F2 — Capability `repository_id` not bound to mutation target — CLOSED

`src/striatum/daemon_rpc/server.py:91-92,124-139` resolves `repo_root` via
`SELECT repo_root FROM striatumd.repositories WHERE repository_id = %s AND
state = 'active'` and refuses (`repo_not_registered`) when the registered
path differs from `self.repo_root`. The dispatcher now uses the daemon-
resolved path on line 92 rather than the constructor default. Coverage:
`tests/test_daemon_rpc.py:224-258` exercises the wrong-repo refusal path
with a faked cursor and an `authorize` stub that admits a `repo_a`
capability against a router bound to `repo-a` while the registry rows for
`repo_a` point to `other-repo`. Token-stripping by `_params_to_args`
(`server.py:192-206`) prevents the capability token from leaking into the
forwarded CLI argv.

### F3 — Request-log not appended on refused requests — CLOSED

The terminal flow funnels through `_record_and_return` on both branches:
`src/striatum/daemon_rpc/server.py:93` for the success response and
`server.py:96` for the `except RpcError` response. `_record_and_return`
(server.py:99-122) writes `append_audit_row` first, then
`append_request_log` unconditionally, with the response's `audit_id`
backfilled. Coverage: `tests/test_daemon_rpc.py:138-164` issues
`daemon.describe` without a prior hello and asserts the denied response
carries both an audit call (`auth.decision == "denied"`,
`denial_reason == "version_incompatible"`) and a request_log call
(`decision == "denied"`, `audit_id == 41`).

### F4 — Duplicate-request-id detection runs after audit append — CLOSED

`src/striatum/daemon_rpc/server.py:55-57` calls `request_id_seen` before
entering the try block and returns an error response directly without
invoking `_record_and_return`. The replayed `request_id` therefore writes
no new audit row and no new request_log row, preserving the
audit↔request_log pairing from the first attempt. Coverage:
`tests/test_daemon_rpc.py:167-183` monkeypatches `request_id_seen` to
return True and `pytest.fail`s both `append_audit_row` and
`append_request_log`, asserting the response carries `duplicate_request`.

(See N1 below for a residual concurrency observation when the accept loop
ships.)

### F5 — Handshake state shared across all callers — CLOSED

`src/striatum/daemon_rpc/server.py:51,53,59,78` replaces the prior
`_handshaken_request_ids` set with `_handshaken_connections: set[str]`
keyed on a `connection_id: str = "default"` parameter passed into
`handle()`. The gate at server.py:59 checks
`connection_id not in self._handshaken_connections`. Coverage:
`tests/test_daemon_rpc.py:106-135` handshakes connection `a`, issues a
post-handshake call on `a` (succeeds past the gate), then issues a
`daemon.describe` on connection `b` and asserts the response is
`version_incompatible`.

### F6 — `daemon.hello` unaudited even on malformed attempts — CLOSED

The hello branch (`src/striatum/daemon_rpc/server.py:69-78`) now falls
through to `_record_and_return` on success (line 93) and through the
shared `except RpcError` handler (line 94-96) when `build_welcome` raises
`schema_invalid` / `version_incompatible`. Both paths emit audit + request-
log rows. Coverage: `tests/test_daemon_rpc.py:186-221` asserts the
successful hello path writes both. (See N2 — a malformed-hello regression
test would strengthen this; the audit path is unconditional in code, but
no test pins the malformed branch.)

### F7 — Supervisor reattach across daemon restart not implemented — CLOSED

The slice ships option (a) from the round-1 fix list: no code path can
mark a daemon-owned supervisor `attached` before reattach verification.
`src/striatum/daemon_supervisor/pointer.py:22-25` rejects any state outside
`{starting, detached, lost, stopped}`, which includes `attached`. A grep
across `src/` confirms no module besides the migration SQL touches
`striatumd.daemon_supervisors`, so the daemon-DB table also has no live
writers. `docs/SPEC.md:1100-1104` now reads "V2 schema/API foundation for
future stronger supervision" — the round-1 "stronger supervision" framing
is gone. Coverage: `tests/test_daemon_rpc.py:306-317` confirms the
repo-local pointer table migrates; the absence of a test for `attached`
follows from the helper rejecting it before INSERT.

### F8 — `daemon.welcome.sealed_apply.supported=true` hardcoded — CLOSED

`src/striatum/daemon_rpc/server.py:70-77` calls
`loaded = key_loaded()` and passes both `sealed_apply_supported=loaded`
and `key_loaded=loaded` into `build_welcome`. Coverage:
`tests/test_daemon_rpc.py:73-94` monkeypatches `key_loaded` to False and
asserts the welcome reports `supported=False` and `key_loaded=False`.

### F9 — Sealed-apply refuse-on-mismatch not exercised by tests — PARTIAL

`tests/test_daemon_rpc.py:97-103` covers `sealed_key_missing`. The other
codes named in DESIGN_SYNTHESIS — `base_tree_mismatch`,
`patch_digest_mismatch`, `verdict_digest_mismatch`,
`verdict_run_mismatch`, `verdict_not_accepting`, `write_scope_violation`,
`apply_token_missing` — are not yet emitted anywhere in
`apply_service.py:16-23`, which only reaches `sealed_key_missing` and
`apply_gate_unsatisfied`. Table-driven coverage of the closed vocabulary
must land with the full apply path. Tracking as F9-residual; not blocking
this slice because the stub cannot reach the unsigned codes today.

### F10 — Workflow schema additions not wired into validator — CLOSED

`src/striatum/workflow.py:566` calls `_validate_apply_gate` for each job;
`workflow.py:952-1009` validates `require_daemon` (boolean type, refused
explicit `False` under `provenance_mode=sealed_patch`), validates
`sealed_patch_provider` against `frozenset({"daemon","refuse"})` and
rejects it outside `sealed_patch` mode; `workflow.py:1011-1033` confines
`apply_gate=true` to `build`/`handoff` job types and requires a
`patch_summary` expected artifact. Coverage:
`tests/test_workflow_field_errors.py:125-195` exercises the positive
`build + patch_summary` path and the negative paths for each field.

### F11 — Signing-key OS keyring path not implemented — CLOSED

`src/striatum/daemon_apply/signing_key.py:22-38` ships a
`keyring_key_loaded` branch that lazily imports `keyring` (returning False
on ImportError) and consults the `striatum.daemon.signing_key` service.
The `0600`-mode `fallback_key_loaded` (lines 33-38) is the documented
fallback path. The env var rename to `STRIATUM_DAEMON_SIGNING_KEY_PATH`
(line 9) lands the round-1 ask. `docs/UBIQUITOUS_LANGUAGE.md:157` reads
"loaded from an OS keyring when available or a `0600` fallback file" —
matches code behavior.

### F12 — CLI dispatcher not yet routing through daemon RPC — CLOSED (TEXT)

A grep for `DaemonRpcRouter|daemon_rpc.server` across `src/` returns only
`server.py` itself; no CLI or daemon-process call site instantiates the
router. The round-1 demand was honesty in the SPEC, not premature routing.
`docs/SPEC.md:1040,1059-1062` now reads "RFC 0030 adds the daemon V2 RPC
server-side foundation" and "This foundation does not yet make the
installed CLI route ordinary operator commands through the daemon by
default." The BUILD_HANDOFF echoes that scope. The latent router is
acceptable for this slice given the explicit doc text.

## New Findings (Round 2)

### N1 — Duplicate-request guard not transactionally atomic with audit/request-log inserts (SHOULD-FIX, deferrable)

File: `src/striatum/daemon_rpc/server.py:55-57`, `request_log.py:39-40`.

The fast-path `request_id_seen` check at `server.py:55` runs outside the
audit/request-log insert path. The internal re-check in
`append_request_log` (lines 39-40) raises `RpcError("duplicate_request")`
after `append_audit_row` has already written a row, because
`_record_and_return` (`server.py:102-121`) calls audit-first then
request-log. In today's single-threaded skeleton the two concurrent
requests this race needs cannot arise. Once the accept loop ships with
real concurrency, two simultaneous requests with the same `request_id`
can both pass the outer `request_id_seen` check, both insert audit rows
(via successive calls), and only one will land in `rpc_request_log`. The
losing audit row then has no paired request-log row — the exact pairing
defect round-1 F4 chased, only race-conditioned.

Fix shape: wrap the duplicate check, audit insert, and request-log insert
inside a single SERIALIZABLE transaction (or take a lightweight advisory
lock keyed on `request_id`) so the audit row only appears when the
request-log row will appear with it. Track with the accept-loop
milestone; not blocking this slice.

### N2 — Malformed `daemon.hello` audit branch is unit-asserted only by inference (SHOULD-FIX, low cost)

File: `tests/test_daemon_rpc.py:186-221`.

The success path of `daemon.hello` is pinned, and the code at
`server.py:69-77` plus the shared `except RpcError` handler at
`server.py:94-96` makes the malformed branch reach `_record_and_return`
by construction. A regression test that drives a malformed-client hello
(missing `supported_envelope`, unsupported framing) and asserts both an
audit row and a request-log row carry `denial_reason=version_incompatible`
or `schema_invalid` would lock the audit invariant against future
refactors. The current shape relies on visual inspection of the shared
`_record_and_return` call. Easy to add; not blocking.

### N3 — `record_pointer` has a dead "`attached` without `pid_start_time`" branch (NIT, code hygiene)

File: `src/striatum/daemon_supervisor/pointer.py:22-25`.

```python
if state == "attached" and pid_start_time is None:
    raise ValueError("daemon supervisor pointers cannot attach without pid_start_time verification")
if state not in {"starting", "detached", "lost", "stopped"}:
    raise ValueError("daemon supervisor pointer state is not enabled before reattach verification")
```

The second check refuses every `attached` value outright, so the first
check is unreachable. This is correct from a threat-model standpoint
(F7's option-a gate holds), but it leaves a misleading guard the next
maintainer is likely to "tidy up" by dropping the second check while
trusting the first — which would re-open F7. Delete the dead first
branch or, when the reattach loop ships, replace the second check with
the first. Non-blocking.

### N4 — `_repo_root_for` short-circuits when `pg_conn is None` (DEFENSIVE NIT)

File: `src/striatum/daemon_rpc/server.py:125-127`.

```python
if self.pg_conn is None or repository_id is None:
    return self.repo_root
```

For methods with a non-None `required_capability`, the earlier
`token_missing` gate at `server.py:80-82` already refuses when
`pg_conn is None`, so this branch is unreachable for any route that
mutates. The only methods that reach `_repo_root_for` without a
`pg_conn` are unauthenticated (currently only `daemon.hello`, which
takes its own branch). The early return is therefore safe today but
will silently widen the boundary if any future route declares
`required_capability=None` + `repository_scope=True`. Consider asserting
`self.pg_conn is not None` (or raising `token_missing` defensively) at
the top of `_repo_root_for`. Non-blocking.

## Threat-Model Verdict

`accept_with_findings`. The round-2 revisions materially close every
must-fix item from round-1 with code-resident invariants and regression
tests, and no new must-fix surface emerges. Residual items are
race-window tightening (N1) and three test/code-hygiene refinements
(F9-residual, N2, N3, N4) that the BUILD_HANDOFF can track as follow-ups
without blocking this foundation slice.
