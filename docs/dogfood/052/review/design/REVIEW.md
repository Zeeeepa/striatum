---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0043", "v1-6", "design"]
---

author: reviewer-unknown-model-001

# Design Review — RFC 0043 V1.6 synthesis

Posture: `ergonomics_dx`. Read scope: synthesis + three source
designs only.

## Summary

`docs/dogfood/052/DESIGN_SYNTHESIS.md` maps the four V1.6 findings
(F-escape, F-split-brain, F-lock, F-help) onto a sequential edit
order with per-finding acceptance tests. V2.0 deferral of gemini A1
is named explicitly with its reasoning (method-by-method Postgres
port, compatibility fixtures, RPC coverage, retirement of
`striatum.api.invoke` delegation). The synthesis decisively resolves
the contested exit code (8, not 14) and the two-key opt-out
(`STRIATUM_DAEMON_REQUIRED=0` + `STRIATUM_TEST_HARNESS=1`).

From a first-time-implementer perspective the synthesis is
actionable: a reader can walk the implementation order, open the
named file, and apply the change. The findings below are small
discoverability gaps an implementer will hit and have to resolve
with judgment rather than direct quotation from the synthesis.

## Findings

### F-dx-1 (low): split-brain test file path is an either/or

The synthesis says (lines 60-62):

> Add the split-brain regression tests in
> `tests/exit_codes/test_rfc0043_split_brain.py` or
> `tests/test_db_split_brain.py`

Two paths joined by "or" forces the implementer to choose without a
stated tiebreaker. The acceptance section (line 81) later refers
back to "the split-brain tests" generically. Recommend collapsing
to one path. `tests/exit_codes/test_rfc0043_split_brain.py` matches
the sibling test for F-escape (`test_rfc0043_refusals.py`) already
named in the same paragraph, which is the consistent choice.

### F-dx-2 (low): error class name is unspecified

Synthesis line 32-34 directs the implementer to "raise a typed
Striatum error with exit code 12 and the same `repo_not_migrated`
remediation shape used by the daemon-required gate." The three
source designs disagree on the class name: claude proposes a new
`RepoMigratedError` subclass; gemini reuses the existing
`RepoNotMigratedError`; codex stays generic ("typed Striatum
error"). The synthesis does not pick. A first-time implementer will
either invent a new class or reuse the existing one without knowing
which the reviewer expects. Recommend the synthesis name the class
(reusing `RepoNotMigratedError` is the lighter touch and matches
the "same remediation shape" wording).

### F-dx-3 (low): F-help requirements list is partial

Synthesis lines 45-47 enumerate three specific help-text
requirements (`(default)` on `--keep-sqlite-readonly`,
`--confirm-delete` on the delete path, `STRIATUM_DAEMON_DB_URL` on
`--postgres-url`) but the surrounding directive is "help text for
every `migrate-repo-local` flag." The claude design enumerates all
nine flags with sample help strings; the synthesis does not lift
that table. An implementer following the synthesis alone will write
adequate but non-uniform help; following claude's enumeration will
produce the consistent surface the parser test wants to assert.
Recommend the synthesis either lift the per-flag table or point
explicitly at the claude design as the help-text source.

### F-dx-4 (low): dry-run lock policy is permissive

Synthesis lines 39-40: "Dry-run may take the same lock for a simple
'one migration command inspects or migrates at a time' rule." The
"may" leaves the implementer to decide. Codex notes "prefer
exclusive for simpler semantics." Recommend the synthesis make the
call (taking the exclusive lock on dry-run gives operators one
consistent error mode and matches the acceptance verifier the
implementer will write).

### F-dx-5 (low): fd-open idiom for the flock is unspecified

The three designs use three different patterns for the lock fd:
codex `source_path.open("rb")` context manager, claude `os.open` +
explicit `os.close`, gemini `open(source_path, "rb")` context
manager. The synthesis says "add the standard-library `fcntl` lock
context manager and wrap the full migration body" without picking.
Either pattern works; calling one out (the context-manager pattern
is the lighter touch) removes a small DX paper-cut.

## What works well

- The implementation order in lines 49-71 reads as a checklist; a
  first-time implementer can open files top-to-bottom.
- The exit-code conflict (claude proposed new code 14; codex/gemini
  reused 8) is resolved with a stated reason ("avoid introducing a
  new exit code for this narrow V1.6 slice").
- V2.0 deferral is justified, not just declared: the synthesis
  names the four things V2.0 must do that V1.6 is not doing.
- The split-brain decision to keep `db.connect` filesystem-local
  rather than reach for the daemon registry is motivated
  ("dependency-free", "no RPC or Postgres coupling to the low-level
  SQLite connector").
- Acceptance criteria (lines 73-95) tie each finding to a specific
  named test file and assertion shape.

## Verdict

`accept_with_findings` (low). The synthesis is implementable as
written; the five findings above are small DX clarifications the
designer can fold in without re-architecting anything.
