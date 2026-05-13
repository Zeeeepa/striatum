---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["rfc-0046", "v1-7-backlog", "ergonomics_dx", "publish-artifact"]
---

author: reviewer-unknown-model-002

# Build Review — RFC 0046 V1 lane evidence guard (ergonomics_dx)

Posture: ergonomics_dx. Evaluated the CLI surface, error messages,
schema migration, and tests from a first-time-operator perspective.
Verdict: **accept_with_findings**. The guard is wired through end to
end, the override path is discoverable from `--help`, and the schema
migration is the smallest possible ALTER. The findings below are
discoverability / consistency rough edges, not correctness defects.

## Required-check verification

| Check | Status |
| :--- | :--- |
| Schema migration adds the column; existing rows read NULL | **Pass** — `migrations.py::_apply_v15_attestation_override_rationale` is an idempotent `ALTER TABLE artifacts ADD COLUMN attestation_override_rationale TEXT`. `tests/test_lane_evidence_guard.py::test_migration_v15_adds_override_rationale_column` asserts the column exists and `notnull=0`. |
| `publish_artifact` refuses model-byline publish when no `process_executions` row covers the path | **Pass with caveat (F1)** — refusal fires when no clean `exited`/`exit_code=0` row exists for the session. The V1 check is not actually path-specific; the handoff acknowledges this as a deferred V1.7 tightening. |
| Override flag with empty rationale refuses (exit 2) | **Pass** — `cli/dispatch.py::dispatch` raises `StriatumError(..., exit_code=2)` before opening the write transaction when `--allow-no-process-execution` lands without a non-empty `--override-rationale`. `artifacts.py::publish_artifact` re-checks at the artifact-layer boundary (defense in depth) — see F3 on the exit-code split. |
| Override with rationale stores rationale + emits the event | **Pass** — `publish_artifact` writes the stripped rationale into `artifacts.attestation_override_rationale` and emits `provenance.publish_without_process_execution` with `{byline, path, rationale}` payload after the successful publish (`artifacts.py:600-614`). |
| Operator-byline publishes pass through unchanged | **Pass** — `_is_operator_byline` matches both `author: operator` and `author: operator [self-declared: ...]`; the guard branch is skipped before the `_lane_evidence_present` lookup. Pinned in `test_is_operator_byline_recognises_canonical_form`. |

All five required checks land. The build is functionally complete.

## Findings

### F1 — `lane_evidence_missing` error message claims a path-specific check V1 does not perform

**Severity: Medium (ergonomics — first-time user will be misled)**

The refusal message at `src/striatum/artifacts.py:522-530` reads:

> lane_evidence_missing: artifact path '<path>' is not present in any
> process_executions row for session '<sid>'; pass
> --allow-no-process-execution --override-rationale "<text>" to record
> an operator override.

But `_lane_evidence_present` (`src/striatum/artifacts.py:625-656`) does
not check the path at all — `path_text` is `del`'d on entry and the SQL
filter is just `session_id = ? AND state = 'exited' AND exit_code = 0
LIMIT 1`. The HANDOFF deviation note is explicit: "V1 ships the weaker
but real 'ran cleanly' guarantee; the path-specific check
(observed_output_paths covers artifact path) is V1.7 once
`process_executions` gains an observed-outputs column."

The mismatch hits a first-time user like this:

1. They publish, get the refusal, read "artifact path … is not present
   in any process_executions row."
2. They look for how to "add this path to a process_executions row" —
   no such affordance exists.
3. They never figure out that any clean exit-0 lane process for the
   session would have unblocked them.

This is a real DX rough edge because the right remediation (run the
work through the supervised lane CLI so a process_executions row lands,
e.g. via `striatum supervise start` / `striatum adapter run`) is hidden
behind a message that points at the wrong shape of the problem.

**Recommended fix (small):**

```python
raise ArtifactError(
    "lane_evidence_missing: session "
    f"{session_id!r} has no completed exit-0 process_executions row; "
    "this typically means no supervised lane CLI ran for the session. "
    "Either run the lane via `striatum adapter run` / "
    "`striatum supervise start`, or pass "
    "--allow-no-process-execution --override-rationale \"<text>\" to "
    "record an operator override."
)
```

This trades a (currently false) path-specific claim for a description
that matches the V1 query *and* names the legitimate-path remediation.
The override remediation stays unchanged. When V1.7 tightens the check
to be path-specific, the message can grow back the path clause without
having lied in the V1 window.

### F2 — `_lane_evidence_present(path_text=…)` signature carries a parameter that does nothing in V1

**Severity: Low (internal-surface ergonomics; future-author footgun)**

`_lane_evidence_present` (`src/striatum/artifacts.py:625-656`) keeps
`path_text` in its kwargs and `del`'s it on the first line, with a
docstring note that the parameter is "kept in the signature for V1.7
binary compatibility once the schema gains the column." The HANDOFF
reiterates the same trade-off.

For a future implementer landing the V1.7 tightening, this signature is
a deliberate placeholder — fine in isolation. For *anyone else* reading
the function (e.g. a contributor extending the guard, or a reviewer
auditing the call site), the parameter looks like it's plumbed through
when in fact it's stubbed. The publisher's call site at
`src/striatum/artifacts.py:519` does pass `path_text=path_text`, which
makes the plumbing look intentional and load-bearing.

**Recommended fix (smaller):** either drop the parameter from the V1
signature (the V1.7 RFC reintroduces it when the schema gets the
observed-paths column — the API surface here is internal-only, so
"binary compat" isn't load-bearing), or leave a `# V1 placeholder —
see RFC 0046 §Open question 1` marker right at the parameter name so
readers don't assume it's already wired. The current `del path_text`
plus docstring works but is easy to miss when grepping.

### F3 — Split exit codes between CLI-layer and artifacts-layer rationale checks

**Severity: Low (defense-in-depth contract drift; surfaces only for programmatic callers)**

The "operator passed `--allow-no-process-execution` without a non-empty
rationale" condition is checked in two places:

- `src/striatum/cli/dispatch.py:548-555` raises
  `StriatumError(..., exit_code=2)` (argparse-style "invalid args").
  This is the CLI path.
- `src/striatum/artifacts.py:531-536` raises `ArtifactError(...)` which
  carries exit code 6 (artifact-layer refusal). This is the
  defense-in-depth belt at the artifacts-layer boundary.

Both messages are nearly identical, but the two raise paths surface
with different exit codes. End-user CLI invocations always hit the
dispatch.py path first, so exit-code-2 is what the operator
experiences. Programmatic callers of `publish_artifact()` (RFC 0046
references operator-on-behalf tooling, dashboard backends, the daemon
RPC router) bypass the CLI guard and get exit code 6 instead.

**Recommended fix (tiny):** raise the same exit code from both sites.
Argparse-style 2 is the documented convention for "invalid argument
combination," so the artifacts-layer raise should also land as exit
code 2 (e.g. `raise ArtifactError("...", exit_code=2)` or a fresh
`StriatumError` subclass that both layers share). The lockstep
contract is cheap to honor and prevents downstream renderers from
needing to special-case the artifact-layer path.

### F4 — Silent fallthrough when `expected_author_line` raises

**Severity: Low (guard bypass when byline derivation breaks)**

`src/striatum/artifacts.py:513-517`:

```python
try:
    byline = expected_author_line(conn, job=job, session_id=session_id)
except ArtifactError:
    byline = None
if byline is not None and not _is_operator_byline(byline):
    ...
```

If `expected_author_line` raises (malformed workflow snapshot, missing
role, session with no ordinal), the guard short-circuits to
pass-through. The exception swallow is fine defensively —
`_enforce_required_attestation_for_artifact` already fired earlier
and the existing `validate_optional_markdown_author_line` path runs
before this block. But for the lane-evidence guard specifically, a
"we couldn't determine the byline" outcome should arguably be treated
as a refusal, not as a green light, since the whole point of the
guard is to refuse model bylines without evidence.

**Recommended fix (small, lower priority than F1):** keep the
exception swallow for the byline-derivation failure mode, but add a
single-line comment naming the deliberate fallthrough and the
rationale (why an `ArtifactError` here is non-fatal for the guard).
Optionally audit the call sites of `expected_author_line` that *can*
raise — if the only legitimate raise is "no expected byline
configured" (which is genuinely operator-byline-equivalent), the
swallow is fine; if other raise paths exist, the comment becomes a
load-bearing invariant.

### F5 — No `striatum why` / `striatum doctor` surfacing of lane-evidence state

**Severity: Low (V1.7 follow-up already named in the RFC; flagging for completeness)**

The RFC 0046 design names follow-ups for the web UI
(`LaneEvidenceChip`) and dashboard (`evid:ok` / `evid:absent` /
`evid:override` column). The HANDOFF defers these to V1.7. Neither
`striatum byline` nor `striatum inbox` nor `striatum why` currently
surfaces the lane-evidence state for a session.

For a first-time operator running into a `lane_evidence_missing`
refusal, the natural debugging move is "what does the runner think
about this session?" — `striatum why <sid> --json` is the canonical
introspection verb but does not yet expose `process_executions`
state. The user is forced to either dig into SQLite directly or
guess.

**Recommended fix:** not in scope for this build, but I'd advocate
adding a `lane_evidence` block to `striatum why <session-id>`'s output
in the V1.7 follow-up so the chain "publish refused → why was it
refused → here's what to do" is one CLI call deep instead of a
SQLite-and-docs hunt. The HANDOFF's V1.7 follow-up list could
reasonably grow this item.

## Discoverability checks — passed

These I tested as a first-time operator reading the surface for the
first time. All passed:

- `striatum publish-artifact --help` lists both new flags with
  RFC-tagged help text that explains the relationship between them
  (must pair `--allow-no-process-execution` with `--override-rationale`).
- The description on the `publish-artifact` subparser already names
  the V1.41 `--kind` / `--logical-name` defaulting behavior; the V1.43
  evidence guard help fits naturally alongside.
- The exit codes are recoverable from `argparse`-style intuition
  (`exit code 2` for missing rationale).
- The `provenance.publish_without_process_execution` event name is
  long but self-describing — it shows up in `striatum why` event walks
  and an operator can grep for it.

## Schema migration sanity

`migrations.py::_apply_v15_attestation_override_rationale` (l. 464-478)
is a textbook idempotent `PRAGMA table_info` guard + `ALTER TABLE ADD
COLUMN`. `LATEST_VERSION` is bumped (l. 543). The `MIGRATIONS` list
preserves the historical-immutable ordering. The migration test pins
both the column presence and the nullable flag (`notnull == 0`). No
concerns.

## Test coverage

`tests/test_lane_evidence_guard.py` pins:

- `_is_operator_byline` recognises canonical operator forms.
- `_is_operator_byline` rejects model bylines (codex, claude-code,
  unknown-model variants).
- `_lane_evidence_present` returns False on an empty DB.
- `_lane_evidence_present` returns True for a `state='exited',
  exit_code=0` row.
- `_lane_evidence_present` returns False for an `exit_code=7` row
  (the failed-completion case).
- Migration v15 adds the column with the nullable flag clear.

This is a tight, fast-feedback set. The HANDOFF cross-references the
broader integration suite (`tests/test_cli_mvp.py` + the dogfood-053
acceptance) for full-publish-flow assertions; I did not re-execute
those. F1's recommended message change would benefit from a regression
test pinning the refusal text (or at least the substring "completed
exit-0 process_executions row") so future text edits don't drift back
toward the path-specific phrasing.

## Verdict

**accept_with_findings.**

The affordances are discoverable, the override path is well-documented
on the CLI, the schema migration is minimal, and the tests pin the
behavior. F1 (refusal-message text) is the one finding I'd want
addressed before the V1.43.0 tag ships, because the current text will
actively mislead first-time operators into looking for a path-specific
affordance that does not exist. F2/F3/F4 are smaller ergonomic edges
that can land in the same patch or a V1.7 follow-up. F5 is
RFC-acknowledged future work, flagged for visibility.
