---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "medium"
tags: ["rfc-0028", "daemon", "v1", "devils-advocate", "round-3"]
---

# RFC 0028 V1 Daemon Build — Devil's Advocate Review (Round 3)

author: reviewer-claude-opus-007

Status: needs_revision
Date: 2026-05-11
Posture: devils_advocate
Scope: round-3 working tree under `src/striatum/daemon.py`,
`src/striatum/mcp.py`, `src/striatum/cli/dispatch.py`,
`src/striatum/cli/recovery.py`, `src/striatum/recovery/auto.py`,
`tests/test_daemon.py`,
`tests/test_daemon_security_adversarial.py`, plus the V1 docs
(`docs/SPEC.md`, `docs/MCP.md`, `docs/UBIQUITOUS_LANGUAGE.md`,
`docs/DECISION_LOG.md`, RFC 0028, dogfood-031 synthesis) and the
round-3 build handoff (`docs/dogfood/031/BUILD_HANDOFF.md`).

## Posture And Stance

Round 3 took the most credible response to round-2 A1: it stopped
claiming a daemon-mediated control plane and instead documented the
shipped V1 as "registry-backed multi-repository coordination plus a
foreground sweep loop." The SPEC, README, MCP doc, UBIQUITOUS
LANGUAGE entry, RFC 0028 notes, and the synthesis are all aligned on
that boundary. I accept the doc-amendment route to A1 as a real
closure of the architectural-honesty problem and credit round 3 for
not pretending an RPC server exists.

Round 3 also lands solid mechanical fixes for the other round-2
items: A2 (manual `daemon sweep` is now admin-gated and audited),
B1 (aggregate-read audit rows now carry the authenticated client id
on the allowed path and audit denials on the denied path), B2
(`STRIATUM_DAEMON_TOKEN` env support is removed and a regression
test asserts it), B4 (the `sweep_degraded` timeout codepath is
exercised by a real test using `per_run_timeout_seconds=-1`), and
B8 (repo identity is realpath/inode-derived, with a duplicate-spelling
dedup test). The fixes look right where I read them.

This review attacks the remaining gaps. The round-3 deferrals are
honest at the docs level but the shipped tests do not actually
verify several synthesis-required dimensions, and one round-3 "fix"
ships unguarded code paths.

## Behavior And Test Gaps

### B1. The "real restart" test does not assert audit chain continuity or segment manifest survival

`test_daemon_restart_preserves_registry_and_instance_id`
(`tests/test_daemon.py:422-473`) now runs
`daemon.run_daemon_foreground(max_sweeps=1)` twice, which is a real
foreground start/stop/restart. Good. The test asserts:

- `instance_id` survives;
- `COUNT(*)` of `repositories`, `clients`, `client_capabilities`,
  and `audit_segments` is unchanged;
- `audit_log` row count grew (i.e. the second start added rows);
- a `scheduler_cursors` row for the registered run survives.

What it does **not** assert is the synthesis test-matrix dimension
that was the entire point of the test:

> "Restart preserves rows, schema versions, audit chain continuity,
> segment manifests, and scheduler cursors survive."

Specifically, the restart test does not:

1. Walk the audit hash chain after restart. `_audit_chain_records`
   exists and is invoked in
   `test_audit_chain_tamper_detection`, but it is not called inside
   the restart test. A daemon that, on restart, wrote a new audit
   row whose `previous_hash` did not match the prior `row_hash`
   would still pass this test as long as the row count grew. The
   current implementation does compute `previous_hash` from the
   last row at insertion time (lines 506-509), so the chain
   probably is intact — but the test would not catch a future
   regression that broke it.
2. Inspect `audit_segments` first/last hashes, manifests, or
   segment-boundary continuity across restart. Only segment row
   count is checked.
3. Assert schema `PRAGMA user_version` is preserved across restart.
4. Verify the **scheduler cursor state** is `active` (the assertion
   is `cursor_after is not None`, which is true for any cursor
   state including `removed` or `sweep_degraded`).

This is the same shape of complaint round 2 made against the
SQLite-reopen test: the test passes by satisfying the words of the
narrowed acceptance bullet ("rows survive"), not the synthesis
requirement ("audit chain continuity"). The handoff claims B3 is
"fixed"; the test matrix dimension that gave the bullet its weight
remains unverified.

Fix: call `_audit_chain_records(conn)` after the second restart and
assert it returns `[]`. Snapshot first/last hashes of every segment
before the second start; reassert they are unchanged after. Snapshot
schema `user_version`; reassert.

### B2. The `recovery.stale_requeued` daemon-author payload ships without a regression test

`run_auto_sweep` (`src/striatum/recovery/auto.py:137-148`) now
passes `recovery_author=author` into `requeue_stale`, and
`requeue_stale` (`src/striatum/cli/recovery.py:138-149`) sets
`payload["author"] = recovery_author` when the kwarg is non-None.
That is the entirety of the round-3 fix for the prior "A3 underlying
recovery bylines" gap; the handoff narrows the scope to review-only
requeue, which is fair.

The test
`test_daemon_sweep_writes_repo_event_with_daemon_byline`
(`tests/test_daemon.py:296-318`) asserts that a
`daemon.recovery_sweep` event payload has `author` starting with
`striatumd-`. It does *not* assert that any
`recovery.stale_requeued` event emitted by the sweep carries the
same payload author. There is no other test that asserts the
underlying requeue event's daemon byline.

The shipped behavior depends on the sweep finding eligible stale
work; without a stale lease, no requeue happens and the bug would
be silent. The test in question runs a sweep against a freshly
started run — there are no stale leases yet — so even if the fix
were silently broken, every assertion in the suite would still
pass.

Fix: in a daemon-sweep test, prime the run with at least one
review-only job, expire its lease, run `daemon_sweep_once()`, and
assert (a) a `recovery.stale_requeued` event exists and
(b) its `payload.author` equals `striatumd-<instance_id>`. Without
this assertion, the round-3 closure of round-2 A3 is verified only
by code reading, not by the test suite.

### B3. Forced-daemon exit codes 9, 10, and 13 remain untested

The synthesis (revision 3) and SPEC §"Forced-daemon failure
exit-code classes" describe stable classes 9 / 10 / 11 / 12 / 13.
The test suite covers only:

- exit 11 (token missing) via
  `test_forced_daemon_read_requires_token_and_direct_mode_still_works`;
- exit 12 (capability denied / repo out of scope) via
  `test_forced_daemon_refuses_unsupported_mutation_verbs` and
  `test_repo_add_requires_initialized_state_unless_init_flag`.

Exit codes 9 (registry/repo schema skew), 10 (foreground lifecycle
conflict — `DaemonUnreachableError` raised when "another striatumd
is active", `src/striatum/daemon.py:855-857`), and 13 (registry
corruption / framing error / internal daemon error) are not
exercised by any test. Code 10 is reachable in V1 even without an
RPC server because `run_daemon_foreground` raises it on PID-file
collision; a test that starts two foreground daemons against the
same registry should fail clean with exit 10. Code 9 is reachable
when `connect_registry()` sees `version > REGISTRY_VERSION`
(`src/striatum/daemon.py:130-134`); a test that pre-writes a higher
`PRAGMA user_version` would exercise it. Both are mechanical to add.

Without these tests, the SPEC promise that forced-daemon failures
"fail clearly and never fall back to direct mode" is untested for
three of five documented classes. If a future change rerouted code
9 through an exception that fell back to direct mode, no current
test would catch it.

### B4. The synthesis test matrix is still under-covered

I re-scored the matrix in `DESIGN_SYNTHESIS.md` against
`tests/test_daemon.py` + `tests/test_daemon_security_adversarial.py`.
Honest scoring against round-3 fixes:

| Synthesis Test Matrix row | Round-3 state |
|---|---|
| Registry lifecycle — restart preserves rows, capabilities, audit chain, segments, scheduler cursors | Partial: row/instance/cursor presence asserted; audit chain continuity, segment manifest hashes, and schema version preservation not asserted (B1) |
| Supported platforms — macOS branch | Untested; CI is Linux-only and the synthesis explicitly says "CI must cover each claimed branch" |
| Existing repo registration — file hash unchanged when default migration is allowed | Untested |
| No-migrate registration — refuses migrating, succeeds when current | Untested |
| Repo identity / re-add — duplicate canonical roots through different spellings | Covered: `test_repo_add_dedupes_canonical_spellings_by_inode_identity` |
| Repo remove idempotency — second remove produces no duplicate revocation rows | Single remove tested; second-call idempotency untested |
| Symlink/path traversal — symlink loops, `..` traversal, state DB symlink escapes, duplicate spellings | Symlink leaf + symlink parent + literal `..` covered; state DB symlink escape and symlink loops untested |
| Token lifecycle — expiry, revocation, rotation, wrong-scope | Wrong-scope tested; expiry, rotation, revoked-then-reused untested |
| Token storage — keyring vs runtime fallback | Keychain branch unimplemented and undocumented as deferred (B5 below); env-var refusal tested |
| Capability scoping — read-token scoped to A cannot read B | Covered: `test_daemon_mcp_requires_explicit_token_and_filters_repo_scope` |
| Audit privacy — no bodies/tracebacks/secrets in rows | Untested |
| Audit integrity / retention — tamper detection, retention tombstone gaps | Hash-row tamper tested; segment retention tombstone gap untested; production retention/rotation honestly deferred |
| Audit chain across rotation — segment last_hash vs next segment's first previous_hash | Doctor surfaces tampered closed-segment manifest; healthy multi-segment boundary previous_hash check (daemon.py:1084-1086) has no test exercising a normal rotation |
| Health audit — every health probe writes a row | Untested |
| Global dashboard correctness — runs, blockers, claimable jobs, stale leases content | Only repository_id presence + bootstrap audit asserted; blocker/claimable-job/stale-lease *content* not asserted |
| CLI daemon read mode — exit codes 9 / 10 / 13 | 11 and 12 tested; 9, 10, 13 untested (B3) |
| Direct CLI preservation — `--daemon` mutation verbs refused | One verb (claim-next) tested; the rest of the mutation surface untested |
| MCP mutation absence — calls to raw invoke/publish/verdict have no daemon route | Covered for `tools/call` shape and `striatum/invoke`; broader matrix (publish, verdict, claim-next, recovery-watch) untested |
| Hostile local clients — oversized payloads, replayed request ids, command-classification bypass | Untested |
| Unix socket and HTTP bind — HTTP refuses 0.0.0.0 / :: / non-loopback | HTTP transport explicitly deferred; socket owner-only tested |
| Resident/foreground recovery — bylines on every run-affecting action | Wrapper byline tested; underlying review-only requeue byline shipped but untested (B2) |
| Crash during sweep — interrupted run is next eligible target | Untested |
| Duplicate recovery scheduling — doctor warning | Covered via cursor + pidfile injection |
| Version and auth failures — protocol/schema mismatch | Untested (B3) |
| Provenance honesty — dashboard renders `advisory` / `attested_bylines` / `sealed_patch` correctly | Untested |

Round 3 added meaningful coverage: bytes/canonical identity,
sweep timeout via real per_run_timeout=-1, sweep CLI admin gate,
MCP repo-scope denial. But ~10 matrix rows that the synthesis
specifically named as V1 acceptance dimensions are still
unaddressed. The handoff does not enumerate which matrix rows it
considers deferred, so an outside reviewer cannot tell which gaps
are intentional vs forgotten.

Fix: extend the handoff to either commit to a follow-up test for
each unaddressed matrix row, or explicitly defer the row with a
sentence per item. The current opaque "remaining follow-ups"
paragraph mixes deferred behavior (process-reconcile byline) with
deferred tests (audit privacy, health audit, crash during sweep)
and doesn't reach the granularity the synthesis writes at.

## Architectural And Documentation Issues

### B5. macOS support is claimed without a Keychain implementation, and the docs do not flag the gap

Synthesis line 138-143:

> "macOS uses Keychain Services for token storage. Registry state
> lives under `~/Library/Application Support/striatum/`; runtime
> socket and token fallback files live under
> `~/Library/Caches/striatum/runtime/` ... macOS runtime-file
> fallback emits a degraded-trust warning ..."

SPEC line 957 says "macOS uses Application Support for registry
state and Caches for runtime files." That matches the path helpers
(`registry_path`, `runtime_dir` in daemon.py:84-100).

What is **not** implemented: anything Keychain-related. The token
lifecycle uses only the runtime fallback file
(`token_file()` → `~/Library/Caches/striatum/runtime/client-token`).
There is no keyring binding, no degraded-trust warning, no
preference resolution between Keychain and the runtime file. The
runtime file is the *only* path on macOS.

The synthesis was explicit ("OS keyring is preferred for CLI and
MCP secret storage. Linux uses the configured keyring provider;
macOS uses Keychain Services") and added a degraded-trust caveat
for the fallback. Round 3's build handoff does not acknowledge that
the Keychain branch is absent from V1. The SPEC paragraph on macOS
is silent on storage primary path: it lists file locations but
does not say "Keychain is the primary store" or "Keychain is
deferred."

This is the same class of overpromise round-2 D1/D2 attacked,
narrowed to macOS storage instead of daemon transport. Either:

- ship a minimal Keychain primary path (e.g. via `keyring` package
  on Darwin with `MacOSKeyring` backend), with the file as the
  documented fallback; or
- add a one-line deferral in BUILD_HANDOFF "Deferred to Follow-Up
  RFC" and in the synthesis Token Lifecycle section: "macOS
  Keychain integration is deferred; V1 macOS storage is the
  owner-only runtime fallback file with a degraded-trust
  recommendation."

I cannot find that deferral anywhere in the round-3 doc set. Round 3
fixed B9 ("daemon-start bootstrap token docs") but did not address
the macOS Keychain gap.

### B6. Bootstrap admin token has no expiry, and the SPEC does not surface this

`_bootstrap_admin_if_needed` calls `create_client(...,
expires_at=None ...)` (`daemon.py:359-366`). The runtime fallback
token written by `daemon start` or `repo add` therefore lives
indefinitely on disk under `~/.cache/striatum/runtime/client-token`
or the platform equivalent.

The synthesis Token Lifecycle says "Tokens support expiry,
revocation, rotation, display names, and repository scope" and
recommends "a short expiry recommendation" for runtime-file
fallback. The bootstrap admin token uses the file fallback by
construction and never expires. The SPEC paragraph on `daemon
start` / `repo add` bootstrap is silent on the expiry shape: it
says the token is written with `0600` permissions but does not say
the token is long-lived.

An operator reading the SPEC text might assume the bootstrap
token rotates. It does not — nothing in V1 rotates it. There is
also no `daemon rotate-token` admin verb in the V1 dispatch.

Either:

- give the bootstrap token a documented default expiry (e.g. 30
  days), surface the expiry in the bootstrap `repo add` JSON
  output, and add a `daemon issue-token --rotate` admin verb; or
- amend SPEC §"Registry-Backed Multi-Repo Coordination" to say
  "the bootstrap admin token is long-lived and operators should
  rotate it through a future `daemon issue-token` verb when one
  ships."

The current state silently ships an indefinite admin token whose
file path is well-known. That contradicts "short expiry
recommendation" in the synthesis without acknowledging the
trade-off.

### B7. `_canonical_repo` raises plain `FileNotFoundError` for nonexistent paths instead of a daemon error class

`_canonical_repo` calls `resolved = raw.resolve(strict=True)` at
line 309. If the input path does not exist, `resolve(strict=True)`
raises `FileNotFoundError`. The function therefore bypasses the
clean `DaemonCapabilityError` flow for a class of input the
operator might plausibly hit:

```bash
striatum repo add /tmp/never-existed
# Traceback (most recent call last):
# FileNotFoundError: [Errno 2] No such file or directory: '/tmp/never-existed'
```

The other refusal paths in the same function raise
`DaemonCapabilityError`, which maps to exit code 12. A
`FileNotFoundError` instead maps to whatever the outer dispatch
catches; under `striatum.cli.invoke`, an uncaught exception
becomes exit code 1 with an exception traceback in the JSON
envelope. The synthesis Audit Privacy line says "Daemon audit
rows ... never tracebacks." The dispatch layer is a different
surface, but an operator using `--json` will see the traceback
text in `error.message`.

Fix: catch `FileNotFoundError` and `OSError` inside
`_canonical_repo` and raise
`DaemonCapabilityError("repo path does not exist")` instead. Add a
test that asserts exit code 12 with a clean message for a
nonexistent path.

### B8. `_has_symlink_component` silently treats `OSError` as a symlink rejection

`_has_symlink_component` (`daemon.py:319-328`) iterates path
components and checks `current.is_symlink()`. The bare
`except OSError: return True` collapses every filesystem error
(permission denied, no such file, ENOTDIR, ELOOP) into a
"this path contains a symlink" refusal. The operator-visible error
text is `repo registration refuses symlink paths` from the caller
(line 309), even when the actual cause is a permission denial.

Two consequences:

1. The error message lies about the cause when the underlying
   error is not a symlink. An operator debugging a permissions
   issue on `/srv/repo` will be told the path is a symlink. They
   are not.
2. A future change that adjusts permissions or moves the runtime
   file could turn permission errors into spurious symlink
   refusals without test coverage catching it.

This is a small bug, but it sits inside the symlink-refusal path
the security review (`docs/dogfood/031/review/build/security/REVIEW.md`)
specifically flagged as a hardening surface.

Fix: distinguish ELOOP / EXDEV / actual symlink detection from
ENOENT / EACCES / ENOTDIR by inspecting the exception's errno.
Raise `DaemonCapabilityError` with the specific reason. Add a
test that uses a directory without read permission and asserts a
permission error message, not a symlink message.

### B9. `dashboard --all` is registry-backed even without `--daemon`, which makes the direct-CLI guarantee partially conditional

SPEC line 998-1003:

> "`dashboard --all` fans out over registered repo-local state
> stores and degrades individual bad repositories rather than
> treating registry copies as live run truth. It is registry-backed
> even without `--daemon`, so it requires a daemon token bootstrapped
> by `repo add`, `daemon start`, or otherwise supplied through the
> client surface."

This is an honest description, but it carves a hole in the
"direct CLI mode still works for existing workflows" acceptance
bullet. A user who relied on `striatum dashboard` in the past
expected per-run output. The new `dashboard --all` flag is a new
command, so this is technically additive. But the same verb
(`dashboard`) now has bifurcated authority requirements: `--run-id`
is direct-CLI; `--all` requires a daemon token even when
`--no-daemon` is set.

A user who never ran `repo add` and therefore never bootstrapped a
runtime token gets exit 11 from `striatum dashboard --all`.
The error message will say "daemon authorization failed:
token_missing" — which is confusing to someone who passed neither
`--daemon` nor any explicit registry intent. The shipping behavior
is reasonable; the docs and the error message could be clearer
about why a token is required.

Fix: change the error message for missing bootstrap when
`--daemon` is not set to: "`dashboard --all` requires registering
at least one repository with `striatum repo add` to bootstrap a
daemon admin token." Otherwise users will read the current message
as a sign that direct CLI mode is broken.

### B10. `daemon doctor` duplicate-watcher detection still swallows every exception

Round-2 B5 flagged this; round-3 handoff explicitly defers it as
"Doctor still needs more explicit non-active repo diagnostics and
less broad exception swallowing." `daemon_doctor_records`
(`daemon.py:1003-1015`) retains:

```python
except Exception:  # noqa: BLE001
    continue
```

The handoff acknowledges the deferral, so this is not a regression
from round 3 — but it is a documented hole in V1 that I want
acceptance reviewers to see plainly. Doctor's contract is "surface
known problems," and the duplicate-watcher detection silently
becomes a false-negative engine on any error inside the try block.
A future code change that imports something differently or moves a
file path would silently disable detection.

I do not consider this a fresh blocker — round-3 documents it as
deferred — but the deferral should be tightened: either fix it
now (small change) or add a test that asserts at least one
positive-detection path so a future regression in the try block
is caught.

## Counter-Arguments Considered

**"Round 3 took the doc amendment route on A1 honestly; the rest is
test polish."** Test polish is most of the synthesis acceptance
criteria. The criterion is not "the code probably works" — it is
"tests cover the synthesis test matrix." Round 3 explicitly framed
several round-2 problems as deferred to a follow-up RFC, but did
not enumerate the deferred *test* rows. B1 (audit chain
continuity across restart) is named in the synthesis matrix and
was the entire justification for upgrading the restart test from
SQLite reopen to real foreground start; the new test still fails
to assert it.

**"The recovery byline propagation is deferred; only the wrapper
byline is required for V1."** Round-3 handoff explicitly says
review-only `recovery.stale_requeued` events triggered by daemon
sweep now carry `payload.author = striatumd-<instance-id>`. That
behavior is shipped. The defer is on the *broader* propagation
(process reconcile, cancel, blocker internals). The narrower fix
that *is* shipped should have a regression test. Without one, a
silent revert of the
`recovery.stale_requeued` author payload in a future refactor would
not show up as a test failure.

**"macOS Keychain is implicitly deferred because the file fallback
works."** The synthesis explicitly distinguishes preferred storage
(keyring) from fallback (file), and tells operators to treat the
fallback as degraded. Shipping V1 with only the fallback while the
SPEC describes "macOS uses Keychain Services for token storage"
asks operators to trust documentation that does not match the
build. The fix is one sentence in BUILD_HANDOFF and one sentence
in SPEC.

**"Forced-daemon exit codes 9, 10, 13 are documented but not all
reachable yet."** Exit 10 is reachable in V1 today via
`run_daemon_foreground` PID conflict
(`DaemonUnreachableError` at daemon.py:855-857). Exit 9 is
reachable via registry schema skew
(`SchemaVersionError` at daemon.py:134-137). Both should have
tests. Exit 13 might be near-unreachable in V1 (no RPC framing),
but if so, the SPEC's "Registry corruption, protocol framing
error, malformed daemon response, or internal daemon error"
should split into "reachable now" vs "reachable with RPC
transport."

**"Test count went from 570 to 574; that is enough."** Round 2
made this exact counter-argument and I rejected it then. Same
answer now: the matrix rows that count are the synthesis acceptance
dimensions, not the absolute test count. The four-test delta does
not cover the four synthesis dimensions named in B1, B2, B3, and
B5 above.

## What Would Move This To `accept_with_findings`

In priority order:

1. Extend `test_daemon_restart_preserves_registry_and_instance_id`
   to assert `_audit_chain_records` returns empty after the second
   start, snapshot each segment's `first_hash`/`last_hash` before
   and after, and assert schema `PRAGMA user_version` is
   preserved. This is the single most important gap because it is
   the test the handoff cites as fixing B3.
2. Add a test that primes a stale review-only lease, runs
   `daemon_sweep_once()`, and asserts the resulting
   `recovery.stale_requeued` event payload carries
   `author: striatumd-<instance-id>`. This is the missing
   regression test for the round-3 fix to the underlying recovery
   byline.
3. Add deferral language to the synthesis Token Lifecycle section
   and to BUILD_HANDOFF "Deferred to Follow-Up RFC" explicitly
   naming "macOS Keychain integration", or land a Keychain primary
   storage path. Pick one.
4. Either give the bootstrap admin token a default expiry and ship
   a rotation verb, or amend the SPEC paragraph on `daemon start`
   / `repo add` to say the bootstrap token is long-lived and
   rotation is deferred.
5. Add tests for forced-daemon exit codes 9 (registry schema skew
   pre-write) and 10 (PID-file collision). Both are mechanical:
   `connect_registry` raises `SchemaVersionError` when
   `user_version > REGISTRY_VERSION`, and `run_daemon_foreground`
   raises `DaemonUnreachableError` when the existing pid is alive.
6. Replace the bare `except OSError: return True` in
   `_has_symlink_component` with a per-errno discriminator that
   distinguishes ELOOP/symlink detection from permission/no-such-file
   conditions; add a test for a permission-denied path that asserts
   the error message does not say "symlink".
7. Catch `FileNotFoundError` from `resolve(strict=True)` in
   `_canonical_repo` and reraise as `DaemonCapabilityError` so
   `striatum repo add /nonexistent` produces a clean exit 12
   instead of a traceback.
8. Add a deferral list in BUILD_HANDOFF that names every synthesis
   test matrix row not covered (audit privacy, health audit,
   crash during sweep, provenance honesty, hostile local clients,
   etc.) so an outside reviewer can see scope explicitly.
9. Tighten the `dashboard --all` no-token error to mention
   `repo add` bootstrap; or add `dashboard --all` as a daemon-only
   verb so the missing-token UX is consistent with `daemon
   status`.
10. Add a small positive-detection coverage shim for the doctor
    duplicate-watcher try/except, or fix the bare except now.

## Verdict

**`needs_revision`**.

Round 3 is the most credible iteration of this dogfood. The
architectural-honesty work on A1 (no RPC server, just a registry
SQLite plus foreground sweep) is right. The mechanical fixes for
A2, B1, B2, B4, and B8 from round 2 are real and tested. Round 3
deserves credit for resisting the temptation to ship an RPC
server it does not have.

The remaining gaps are tighter than round 2 but they are not
polish. The restart test claims to satisfy the synthesis "audit
chain continuity" dimension and does not. The recovery byline
propagation ships code but no test, so the closure depends on
code-reading. macOS support is claimed but the Keychain branch
is silently absent. Forced-daemon exit codes 9 and 10 are
reachable today and have no tests.

These are fixable in one more pass without new design work. They
are the kind of test-and-doc tightening that converts a "narrow V1
acceptance slice" into a "narrow V1 acceptance slice that other
engineers can rely on without reading the implementation."

I read this as `needs_revision` because the synthesis test matrix
items I name above are explicit acceptance dimensions, not
non-blocking polish. The acceptance criterion is satisfied in
shipped code but not in shipped tests, and the task prompt
explicitly maps "missing tests for acceptance-criteria bullets"
to `needs_revision`.
