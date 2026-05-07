---
schema_version: "striatum.harness_improvement_proposal.v1"
artifact_kind: "harness_improvement_proposal"
target: "defaults"
expected_benefit: "Prevent silent staleness of the runner installed in `.venv` when Claude Code (or any other tool) re-runs `pip install -e` from inside a temporary worktree. Today this can leave the dogfood-1 user running an outdated Striatum (missing recent migrations and recent `ALLOWED_ARTIFACT_KINDS` entries) without any visible warning."
risk: "Tightening the install path breaks workflows that intentionally install editable from a worktree. The mitigation is to detect-and-warn rather than refuse."
rollback: "Revert the install hardening; the runner already tolerates a stale install, the failure is just confusing."
---

# HARNESS-002 — Editable install can silently pin Striatum to a stale worktree copy

Status: proposed
Run: dogfood-001
Reporter: author-claude-opus-001
Surface: defaults

## Observed friction

A clean run of `striatum init` against the dogfood-001 workflow yielded a
state DB at `PRAGMA user_version = 4`, even though the source tree on disk
ships migration v5 (`open artifact_kind to python validation`). The
artifacts table still carried the old SQL `CHECK` constraint:

```
artifact_kind TEXT NOT NULL CHECK (artifact_kind IN (
  'prompt','finding','findings_ledger','synthesis','marker',
  'handoff','decision','patch_summary','test_report','other'
))
```

So the first `striatum publish-artifact --kind harness_improvement_proposal`
crashed with `CHECK constraint failed: artifact_kind IN (...)` — a kind
that `src/striatum/artifacts.py:ALLOWED_ARTIFACT_KINDS` explicitly accepts
and that `src/striatum/artifacts.py` registers a v1 front-matter schema for.

Root cause: `pip show striatum` reported the editable install as

```
Editable project location: /home/halbritt/git/striatum/.claude/worktrees/agent-a646fe1798effc72c
```

That worktree is a Claude Code-managed temporary directory; somewhere in
the run's history Striatum got `pip install -e .` against the worktree
copy, then later development on the canonical tree (including v5) never
re-installed. The runner kept running with the worktree's snapshot of
`migrations.py`, which has `LATEST_VERSION = 4` and no v5 migration.

A `pip install -e /home/halbritt/git/striatum` against the canonical tree
fixes it; on the very next `striatum status` call, `apply_migrations`
upgraded the DB to `user_version = 5` and dropped the CHECK constraint.

## Supporting runner evidence

- run_id: `run_a04880660517480a95438fcc0368d2e0`
- job_id: `job_run_a04880660517480a95438fcc0368d2e0_draft_change`
- packet_id: `wp_3c1fc153e2cd40f6ab7ef1447ae2a5e7`
- supervisor_id: n/a (this is a host-side install hazard)
- relevant event types from `striatum why <id>`: not visible to the runner
  — the failure is in `striatum publish-artifact` returning exit 1, and the
  diagnostic only surfaces by inspecting `pip show striatum` and
  `PRAGMA user_version`.

## Proposed change

1. **`striatum doctor` should compare installed package location to repo
   root and warn loudly when they diverge.** The check is cheap: read
   `striatum.__file__` and compare to the repo argument's resolved path.
   Print a high-severity diagnostic ("editable install points at <X> but
   you are running against <Y>; re-run `pip install -e <Y>`") at the top
   of `doctor --verbose` and as a `next_action` in `status` if a run is
   active.

2. **`Makefile install` target should always install from the
   `Makefile` directory, not `.`**, and should print the resolved path it
   used. Today `make install` happens to be `pip install -e .` which is
   `cwd`-dependent; running `make install` from a worktree silently pins
   the install to the worktree.

3. **`striatum init` should compare the on-disk `LATEST_VERSION` to the
   running install's `LATEST_VERSION`** and refuse to initialize a brand
   new state DB against an install that lags the source. This catches the
   "I just ran init in a fresh repo and got an old schema" failure mode
   loudly, before any artifact publish surprises an operator.

## Risk

- Doctor warnings are low-risk additions; only style is at stake.
- Refusing to init against a stale install is more disruptive but is the
  right default: the dogfood operator hit the symptom 60 seconds after
  trying to publish a perfectly-valid artifact, with no useful guidance.
- Makefile change is additive.

## Rollback

- Revert each change independently. None of them affect on-disk state
  shapes or workflow contracts.

## Notes

This is adjacent to the friction in HARNESS-001 but distinct: HARNESS-001
is about the supervised lane shape; HARNESS-002 is about the operator's
host environment lying about which Striatum is in charge. Both surfaced
on the very first `publish-artifact` call of dogfood-001.
