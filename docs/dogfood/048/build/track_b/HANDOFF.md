---
schema_version: striatum.handoff.v1
artifact_kind: handoff
title: "Track B — RFC 0043 V1 CLI surface + RPC registry"
---
author: implementer-unknown-model-001

# Track B Handoff — RFC 0043 V1 CLI surface + RPC method registry

This handoff records Track B as shipped: the operator-facing CLI surface
changes (`--no-daemon` retirement, exit codes 11 and 12 with platform
remediation), the daemon-required dispatch helper, and the RFC 0030
method-registry expansion to cover every mutation in
`src/striatum/cli/mutations.py` per RFC 0043 §5.

Sister Track A (schema migration + `migrate-repo-local` body) lands in
parallel and owns `src/striatum/daemon_pg/sql/`,
`src/striatum/daemon_pg/cutover.py`, and `src/striatum/cli/daemon.py`.
Nothing in Track B touches those paths.

## Files touched

### Source

- `src/striatum/errors.py` — added `DaemonUnreachableError` (exit 11) and
  `RepoNotMigratedError` (exit 12); added `EXIT_*` integer constants for
  the stable exit-code table (1–15) including the renumbered V1 daemon
  codes (auth → 14, capability → 15).
- `src/striatum/daemon.py` — renumbered the legacy V1 RFC 0028 daemon
  errors to free codes 11 and 12 for RFC 0043 (`DaemonAuthError → 14`,
  `DaemonCapabilityError → 15`; `DaemonUnreachableError → 10` retained
  with a docstring note that the RFC 0043 §3 entry-layer error
  `striatum.errors.DaemonUnreachableError` (exit 11) is now the
  socket-unreachable surface). Tests assert these errors by class name,
  not by numeric exit code, so no test fixtures break.
- `src/striatum/cli/parser.py` — removed the `--no-daemon` member of the
  daemon mutual-exclusion group. `--daemon` remains as the V1 RFC 0028
  read-mode opt-in until the daemon-mediated CLI dispatch absorbs it.
  Parsing `--no-daemon` now returns the argparse standard
  `unrecognized arguments: --no-daemon` error (exit code 2).
- `src/striatum/cli/daemon_required.py` (new) — env-gated daemon-required
  dispatch helper. Exposes `enforce_daemon_required(command, repo)`
  which (under `STRIATUM_DAEMON_REQUIRED=1`) probes the daemon socket,
  raises `DaemonUnreachableError` with the stderr remediation block
  when unreachable, and raises `RepoNotMigratedError` when the repo
  shows pre-cutover state. Carries the canonical stderr templates for
  both refusals plus structured `hint` fields for the JSON error
  envelope. `daemon`, `init`, `skills`, `plugin` are in the
  `DAEMON_OPTIONAL_COMMANDS` allowlist so the doctor and lifecycle
  commands run without a daemon (RFC 0043 §3 acceptance criterion).
- `src/striatum/cli/dispatch.py` — wired
  `enforce_daemon_required(args.command, repo)` at the top of
  `dispatch()`, added a dedicated `except (DaemonUnreachableError,
  RepoNotMigratedError)` arm in `main()` that emits the multi-line
  stderr block in human mode and the JSON error envelope with the
  structured `hint` field in `--json` mode, and removed the
  `args.no_daemon` reference. Replaced the three stale legacy
  exit_code=12 callsites (V1 "--daemon does not support X" / cross-repo
  cancel placeholder / daemon-routable fall-through) with exit_code=8
  WorkflowError-style codes so the new code 12 stays unambiguously
  the `repo_not_migrated` semantic.
- `src/striatum/daemon_rpc/registry.py` — expanded `_ENTRIES` per the
  design synthesis vocabulary. Every mutation in `cli/mutations.py`
  now has a dotted method name (`session.register`, `session.close`,
  `work.claim_next/ack/heartbeat/complete/block/release/send_message`,
  `artifact.publish`, `review.submit/verdict/override`,
  `decision.record`, `checkpoint.resolve`, `branch.confirm`,
  `run.prepare/start/pause/resume/cancel/retry_job`, `worktree.create/
  release/list`, `recovery.stale_leases/requeue_stale/cancel_job/
  process_reconcile/resume/auto/watch`, `supervise.*`, `workflow.*`).
  Added read-capability entries for `status`, `why`, `doctor`,
  `dashboard`, `dashboard.all`, `evidence.export`, `corpus.export`,
  `run.summary`, `run.graph`, the `list.*` family, and the new
  daemon-global entries `repo.list`, `daemon.migrate_repo_local`.
  Legacy undotted vocabulary (`ack`, `heartbeat`, `release`, `block`,
  `complete`, `publish_artifact`, `claim_next`, `verdict`,
  `submit_review`) kept as `deprecated=True` registry entries so the
  existing daemon RPC server still resolves in-flight clients while
  callers migrate.
- `src/striatum/daemon_rpc/server.py` — expanded `CLI_ROUTES` to map
  every dotted name onto the matching CLI verb tuple. Legacy aliases
  share the same routes so deprecated names still execute. No
  behavioral change to the request-routing pipeline.

### Tests

- `tests/cli/__init__.py` (new), `tests/cli/test_no_daemon_retired.py`
  (new) — asserts argparse exits 2 with `unrecognized arguments:
  --no-daemon`, asserts `--daemon` still parses, asserts the retired
  flag is absent from `--help` output.
- `tests/cli/test_daemon_doctor_without_daemon.py` (new) — exercises
  the `DAEMON_OPTIONAL_COMMANDS` allowlist so `daemon`, `init`,
  `skills`, `plugin` continue to work without a reachable socket
  under enforcement.
- `tests/exit_codes/__init__.py` (new),
  `tests/exit_codes/test_rfc0043_refusals.py` (new) — covers exit code
  11 (`daemon_unreachable`) and 12 (`repo_not_migrated`): stderr
  template assertions for the four remediation channels (Linux systemd,
  macOS launchd, foreground, Postgres), JSON envelope shape with
  `hint`, env-gated activation (no-op by default), unix-socket
  reachability probe (negative + positive), pre-cutover sqlite
  detection. End-to-end `dispatch.main(...)` call paths assert the
  exit codes and stderr / JSON envelope wiring on the live CLI surface.
- `tests/daemon_rpc/__init__.py` (new),
  `tests/daemon_rpc/test_registry_rfc0043_coverage.py` (new) — the
  exhaustiveness test: a static map of mutation function names →
  RFC 0043 §5 method names, asserts (a) every mutation has a
  registered method, (b) every method's required capability matches
  the §5 table, (c) every canonical method either routes via
  `CLI_ROUTES` or is in the server-inline allowlist, (d) legacy
  undotted aliases are flagged `deprecated=True`, (e) `recovery` and
  `surgical_recovery` capabilities are recognized, (f) repo-scope
  modes (single_repo / cross_repo / daemon_global) match the synthesis.

### Documentation

- `docs/dogfood/048/build/track_b/HANDOFF.md` (this file).

## Test execution

`make lint`, `make typecheck`, and `make test` could not be executed
inside this supervised invocation — the operator's permission
configuration declined approval for shell commands across the run
(including `striatum ack`, which the work packet's task prompt
explicitly accommodates with "If `striatum ack` is denied, write the
HANDOFF and exit normally"). The shipped code was reviewed for
correctness by inspection against the existing test patterns and the
design synthesis. The new test suites under `tests/cli/`,
`tests/exit_codes/`, and `tests/daemon_rpc/` are designed to be
self-contained (only stdlib + pytest + monkeypatch + tmp_path), so they
should run cleanly against the repo's existing pytest configuration.

The next operator action is to run `make lint typecheck test` against
the branch and report verdict back through `submit-review`. Expected
behavior of existing fixtures is unchanged because daemon-required
enforcement is opt-in via `STRIATUM_DAEMON_REQUIRED=1`.

## Deviations from the design synthesis

None substantive. Three small judgment calls:

- The synthesis suggested a `dispatch_via_daemon_required(args, repo)
  -> object` shape that fully replaces the legacy dispatch body. Track
  B instead delivers a smaller `enforce_daemon_required(command, repo)
  -> None` hook that the existing dispatcher calls before the SQLite
  fallback path. This preserves backward compatibility against the
  current test suite (the prompt's non-negotiable requirement) while
  giving the operator the explicit refusal shape under
  `STRIATUM_DAEMON_REQUIRED=1`. The dispatcher rewrite the synthesis
  envisaged is a clean follow-up once Track A lands and the daemon
  becomes the actual routing target for every verb.
- The synthesis vocabulary distinguished `session.register/claim` from
  `work.claim_next/claim`. Both keep the `claim` capability in Track
  B's registry. The `session.close/claim` synthesis entry was kept as
  `claim` rather than `write` because session-close is a claim-lane
  lifecycle event, not a state-mutation event in the workflow sense.
- The synthesis envisioned a `0006_rpc_registry_rfc0043.sql` migration
  to widen `striatumd.rpc_methods.required_capability` CHECK. Track A
  owns `src/striatum/daemon_pg/sql/` so Track B does not write that
  migration here. The existing CHECK widening from `0002_…sql` already
  covered `recovery` and `surgical_recovery`; the new admin/claim/
  review capabilities are pre-existing values.

## Open follow-ups (not in Track B scope)

- The actual daemon-mediated CLI dispatch (turning the env-gated
  `enforce_daemon_required` hook into a full RPC client call for every
  verb) lands in a subsequent track. Track B ships the refusal surface
  and the registry vocabulary; the routing change is a separate
  implementation.
- The `repo.init` registry entry has no CLI route nor inline handler
  yet — the existing `striatum init` verb is in the daemon-optional
  allowlist. A follow-up track should either wire `repo.init` through
  the registry or remove the entry.
- `supervise.reattach_status` is in the registry without a CLI verb —
  this predates Track B and is preserved as-is.
- `workflow.generate.preview` similarly has no live handler — also
  preserved as-is. Both are explicitly flagged as latent gaps for the
  next dogfood pass.
- Track A's `daemon migrate-repo-local` body will need to coordinate
  with the `repo_is_migrated(repo)` helper to read the authoritative
  `striatumd.repo_migrations` row rather than the heuristic sqlite /
  tombstone check Track B ships. The helper is a single function with
  a clear contract; the swap is mechanical.
