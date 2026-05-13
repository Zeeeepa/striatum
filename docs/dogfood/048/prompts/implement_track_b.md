# Implement Track B: RFC 0043 V1 — CLI surface + RPC registry (claude Python)

Blocked until `review_design` returns an accepting verdict.

Implement Track B per `docs/dogfood/048/DESIGN_SYNTHESIS.md`. **You write Python only. Track B is the operator-facing CLI surface (retirement of `--no-daemon`, exit codes 11/12, daemon-required refusal) + the RFC 0030 method-registry expansion.** Sister Track A (schema migration + migrate-repo-local body, codex) runs in parallel — do not cross into its write scope.

**Your scope (claude Python-side):**

- `src/striatum/cli/parser.py` — remove `--no-daemon`. Parsing it must return the standard unknown-option error per RFC 0043 §3.
- `src/striatum/cli/dispatch.py` — wire exit code **11** (`daemon_unreachable`) when the daemon socket is unreachable. Stderr names the socket path + platform-specific remediation (Linux `systemctl --user start striatumd`, macOS `launchctl bootstrap` hint, `striatumd --foreground` reminder, Postgres install hints). No SQLite fallback path. Wire exit code **12** (`repo_not_migrated`) when verbs are invoked against an unmigrated repo — stderr names `striatum daemon migrate-repo-local`.
- `src/striatum/errors.py` — exit code constants if not already present. `daemon doctor` continues to run without the daemon (touches config only).
- `src/striatum/daemon_rpc/` — expand the RFC 0030 method registry to cover every mutation in `src/striatum/cli/mutations.py` per RFC 0043 §5 table: `session.register`, `work.claim_next` / `ack` / `heartbeat` / `complete` / `block` / `release`, `artifact.publish`, `review.submit` / `verdict`, `decision.record`, `checkpoint.resolve`, `recovery.requeue_stale` / `cancel_job` / `resume`, `worktree.create`, `branch.confirm`, `run.prepare` / `start` / `pause` / `resume` / `cancel`, `supervise.*` (already present), `workflow.validate` / `generate`. Plus read-capability methods for `status`, `why`, `dashboard`, `doctor`, `run summary`, `run graph`, `evidence export`. Capability mapping per the §5 table.
- `tests/cli/`, `tests/exit_codes/`, `tests/daemon_rpc/` — exit-code coverage (11 + 12), `--no-daemon` retirement assertion (unknown-option error), method-registry exhaustiveness test (every CLI mutation has a registered method).
- `docs/dogfood/048/build/track_b/HANDOFF.md` — handoff summarizing shipped scope, files touched, test results, deviations from the synthesis (if any) with one-line rationale.

**Use sub-agents** (one per concern, dispatched in parallel):

- Sub-agent parser retirement: remove `--no-daemon`, assert unknown-option error.
- Sub-agent dispatcher exit codes: 11 socket-unreachable + 12 unmigrated-repo, with stderr remediation templates.
- Sub-agent RPC registry expansion clusters: claim / write / review / admin / recovery / read.
- Sub-agent doctor + remediation: `daemon doctor` works without the daemon; install-hint text.
- Sub-agent exhaustiveness test: enumerate every mutation in `mutations.py`; assert a registered method exists.

Reconcile sub-agent outputs yourself before writing HANDOFF.

**Do NOT touch**: `src/striatum/daemon_pg/sql/`, `src/striatum/daemon_pg/cutover.py`, or `src/striatum/cli/daemon.py` (sister Track A owns those). **Do NOT write to**: README / TODO / CHANGELOG / RFC index / SPEC / HOW_TO. Operator handles those manually after the dogfood lands.

**Backward-compat (non-negotiable)**: existing test fixtures must continue to pass against `daemon_mode=on`. Integration tests run the daemon-mediated path only.

Verification: `make lint`, `make typecheck`, `make test` all pass. Exit-code tests are in-tree and pass.

One-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.

## Byline discipline

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, lowercase `author:`, NO bold, NO italics, NO lane prefix. Slug shape: `implementer-unknown-model-<NN>`.
