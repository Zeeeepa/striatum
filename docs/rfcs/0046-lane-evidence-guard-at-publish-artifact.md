# RFC 0046 — Lane evidence guard at publish-artifact

**Status:** proposed
**Scope:** V1.7 (single-version)
**Closes:** GH #2, GH #5

## Background

The byline derivation at `src/striatum/identity.py::artifact_author_identity`
already differentiates on `attested` — an unattested session correctly yields
`author: operator` (never a model byline). What it does **not** verify is
that the supervised subprocess actually produced the artifact. `supervise
start` attaches a wrapper process to the session; the runner treats that as
attestation; the wrapper may exit immediately without the lane CLI emitting
any output; an operator can then write the file and call `submit-review`
on behalf and the resulting artifact lands with a model byline like
`reviewer-codex-gpt-5.5-001` even though no codex CLI produced it.

This was discovered during the v1.42.0 GH issue triage and is documented
in `~/.claude/projects/.../project_lane_attestation_gap.md`. The fix
lives at the publish-artifact layer, not the byline layer.

## Goals

- At publish-artifact time, refuse to attest a model byline unless the
  session has a matching `process_executions` row whose output covers
  the declared `expected_artifacts[].path`.
- Provide an explicit operator opt-out
  `--allow-no-process-execution` that records the override in the
  audit chain.
- Pin a regression test that exercises the byline forgery path and
  confirms the refusal triggers.

## Non-goals (V1.7)

- Cryptographic signature of artifact bytes (RFC 0031 future).
- Multi-process attestation (single supervisor per session today).
- Removing the operator-on-behalf flow entirely. The override path
  stays; it just becomes visibly qualified.

## Design

### Trust boundary

The supervised process's `process_executions` row is the authoritative
evidence record. Schema (existing):

```
process_executions(
  process_id, run_id, job_id, session_id,
  command_json, started_at, ended_at, exit_code,
  duration_seconds, stdout_path, stderr_path,
  declared_output_paths_json, observed_output_paths_json,
  state                          -- running | completed | lost | timed_out
)
```

For each artifact published under a model byline, there must exist at
least one `process_executions` row where:
- `session_id` matches the publisher's session,
- `state` is `completed` (not `lost` or `timed_out`),
- `observed_output_paths_json` contains the artifact's repo-relative path.

### Refusal path

`publish_artifact` in `src/striatum/artifacts.py`:

1. After existing scope + byline + schema validation, compute the
   `expected_author_line` for `(job, session)`.
2. Parse the canonical byline: if it matches the operator template
   (`author: operator` or `author: operator [self-declared: ...]`),
   pass through. The trust gap doesn't apply.
3. If the byline is a model byline (`<role>-<model>-<ord>`), look up
   `process_executions` rows for the session and verify the artifact
   path appears in at least one row's `observed_output_paths_json`.
4. If no match, raise `ArtifactError("lane_evidence_missing: artifact
   path <path> not present in any process_executions row for session
   <sid>; pass --allow-no-process-execution to override with an
   operator rationale.")`.

### Override path

`publish-artifact --allow-no-process-execution --override-rationale "..."`:

- Refuses without `--override-rationale` text (operator must record
  why).
- Writes a `provenance.publish_without_process_execution` event into
  the run's event log with the artifact id, session id, byline, and
  rationale.
- Stores the rationale on the artifact row in a new column
  `attestation_override_rationale TEXT` (schema migration in this RFC).

### Schema migration

Add to repo-local schema (next version after current `LATEST_VERSION`):

```sql
ALTER TABLE artifacts ADD COLUMN attestation_override_rationale TEXT;
```

Same on the Postgres daemon side. Existing artifacts continue to read
the column as `NULL`; downstream consumers treat `NULL` as "no
override".

### CLI surface

```
striatum publish-artifact \
  --session-id <sid> --job-id <jid> --lease-id <lid> \
  --kind <kind> --logical-name <lname> --path <p> \
  [--allow-no-process-execution --override-rationale "<text>"]
```

The dispatch layer (`src/striatum/cli/dispatch.py::_resolve_publish_defaults`
already added in v1.41.0) chains the new override flag through to
`publish_artifact`.

### Web UI

Per `CLAUDE_DESIGN_UI_REWORK_PROMPT.md`:

- `LaneAttestationChip` shows `attested` / `unattested:<reason>`.
- New `LaneEvidenceChip` shows `process_execution_present` /
  `process_execution_missing` / `override:<rationale>`.
- The artifact view surfaces the override rationale prominently when
  present.

### Dashboard parity

`striatum dashboard --once --run-id <id>` adds an `evidence` column
to the per-job line:
- `evid:ok` (process_executions match),
- `evid:absent` (no row, no override — blocked from publish),
- `evid:override` (override flag used).

## Acceptance

- `tests/test_lane_evidence_guard.py`:
  - Session with a `process_executions` row covering the artifact
    path → publish succeeds with model byline.
  - Session without a `process_executions` row + model byline
    → publish refuses with exit code 6 and the named error.
  - `--allow-no-process-execution` without `--override-rationale`
    → refuses with exit code 2.
  - `--allow-no-process-execution --override-rationale "..."`
    → publish succeeds, event recorded.
  - Operator-byline (no model byline) → publish passes through
    regardless of `process_executions`.
- `make lint`, `make typecheck`, `make test -m "not multi_repo"` green.

## Migration

Existing repos: add the `attestation_override_rationale` column with
`ALTER TABLE`. Existing artifacts pass through; the guard only refuses
**new** publish-artifact calls.

## Rollout

- v1.43.0: ship the schema migration + guard + override flag + tests.
- Bump exit-code register: reuse exit code 6 (artifact error) for
  refusal; no new code.

## Open questions

1. Should the guard also check `expected_artifacts[].author_line`
   directly when supplied (vs always recomputing
   `expected_author_line`)? Default: recompute, keep behavior single-
   sourced through `identity.py`.
2. For multi-process supervisor sessions (e.g. sub-agent dispatch in
   RFC 0027), do we require evidence from the parent process or any
   process? Default V1.7: any matching process_executions row counts.
