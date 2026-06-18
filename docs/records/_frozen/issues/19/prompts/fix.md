# Fix Prompt: GH #19 + #21

Implement the fixes named in `docs/issues/19/SCOPE.md`. Produce `docs/issues/19/build/HANDOFF.md`.

## Scope per issue

### GH #21 — serve startup must not clobber retired-local-state

Source files (verify exact path during triage): typically `src/striatum/init.py` or wherever `striatum serve` initializes state. Make startup detect an existing healthy retired-local-state and attach to it instead of (re)creating. If the file exists but `PRAGMA integrity_check` fails, error out with a clear message — do not silently re-init.

Test under `tests/`: simulate the exact failure mode that happened 3 times in this session — start serve while retired-local-state has rows, kill -TERM, restart serve, assert row count + sha256 unchanged.

### GH #19 — stale-lease operator escape for repo_write

Source files (verify exact path during triage): typically `src/striatum/cli/recovery.py` + `src/striatum/cli/parser.py` (argparse for the new flag/verb). Minimum viable: add `--force --justification "<reason>"` to `recovery requeue-stale`. When both flags are present and the job is `stale_lease`, requeue regardless of `repo_write` — audit-chain the override.

Test under `tests/`: create a stale_lease repo_write job; call the new verb; assert the job state transitions back to ready/queued AND an audit row with `operator_override=true` + `justification=<reason>` lands in the audit chain.

## Sub-agents

- One sub-agent for #21 (serve startup logic + test).
- One sub-agent for #19 (recovery verb + test).

Each lands its files; integrator writes HANDOFF.

## HANDOFF.md must

For each issue: file paths changed, function names, test paths, test commands, confirmed acceptance per SCOPE.md.

## Byline

`author: implementer-unknown-model-<NN>`. Plain markdown line.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
