---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["dogfood-030", "rfc-0026", "rfc-0027", "devils_advocate", "provenance", "fresh-context"]
---

# Devil's Advocate Review — RFC 0026 + RFC 0027 Build

author: operator
date: 2026-05-11
status: fresh-context attempt 3

## Posture and Scope

Fresh-context, repo-level devil's-advocate read of the working
tree against the BUILD_HANDOFF and the accepted DESIGN_SYNTHESIS.
The question is not "is the byline plumbing in roughly the right
place?" — it largely is — but "do the runtime behaviors, tests,
docs, and surfaces match the guarantees the RFCs and SPEC now
print?" Multiple printed claims do not survive a literal read.

Verdict: **needs_revision**.

## Verdict Rationale — Read This First

Three findings carry the verdict on their own:

- **F1 (overclaim).** `docs/SPEC.md` and the new ubiquitous-language
  entry describe `attested_bylines` as a *mode* that "means RFC 0026
  lane-liveness attestation affects byline derivation and optional
  review-job gates." The runtime does no such thing. Byline derivation
  in `expected_author_line` and the verdict-side gate in
  `_enforce_required_attestation_for_verdict` read attestation state
  and the per-job `require_attested_lane` flag; neither consults
  `provenance_mode` at all. Switching a workflow from
  `provenance_mode: advisory` to `provenance_mode: attested_bylines`
  changes nothing observable. RFC 0027's whole stated purpose was to
  prevent exactly this gap between named guarantees and runtime
  behavior; the V1 SPEC text reintroduces it.

- **F2 (decision-log integrity).** The newly written acceptance row
  reuses an already-assigned decision id. `docs/DECISION_LOG.md` now
  contains two distinct `D080` rows (one for "Accept RFC 0026 V1 plus
  RFC 0027 Phase 2 guardrails" at line 24, and the pre-existing
  "Accept RFC 0024 V4" at line 81). The decision log is the appeal
  surface for "why was this accepted"; a colliding id breaks every
  future `D080` citation.

- **F3 (acceptance criteria still uncovered).** RFC 0026 §"Acceptance
  Criteria" enumerates eight publish-time invariants and explicitly
  names `tests/test_lane_attestation.py` as the home for the new
  tests. No such file exists; the build instead leans on a handful of
  scattered cases in `tests/test_cli_mvp.py` and `tests/test_supervise.py`.
  Several enumerated scenarios — supervised-pid kill → downgrade,
  re-attach → restore, `--operator-label` end-to-end byline rendering,
  evidence-bundle byline distinction — are not asserted anywhere.

These are "documentation that overstates provenance", "migration /
documentation breakage", and "missing tests for core claims" — the
three triggers the review prompt names for `needs_revision`.

## Findings

### F1 — `attested_bylines` provenance_mode is operationally decorative (high, overclaim)

**Claim under review.** `docs/SPEC.md:288-296` (newly added § "Provenance
Modes") and the new ubiquitous-language entry both say that
`attested_bylines` "means RFC 0026 lane-liveness attestation affects
byline derivation and optional review-job gates." The README and RFC
0027 §"Current Implementation Status" repeat the same shape: the mode
is presented as the V1 honesty surface.

**Evidence the runtime contradicts this.** All `provenance_mode` reads
in the code are:

- `src/striatum/workflow.py:949-973` — validation only.
- `src/striatum/cli/mutations.py:192-196` — `run_start` rejects
  `sealed_patch`.
- `src/striatum/cli/introspect.py:198,212-238,1041-1057` — `status`
  reports the mode; `doctor` raises `sealed_patch_unsupported` on
  non-terminal `sealed_patch` runs.

Byline derivation lives in `src/striatum/artifacts.py:569-594`
(`expected_author_line` → `artifact_author_identity`). That code path
reads `session_lane_attestation(conn, session_id=..., mark_lost=True)`
and the session's `operator_label`. It never inspects
`provenance_mode`. The verdict-side gate at
`src/striatum/db.py:1631-1648` reads `require_attested_lane` from the
job snapshot. It also never inspects `provenance_mode`.

**Net effect.** For an unattested session publishing into a workflow
that does not declare `require_attested_lane: true`, the runtime
behavior is identical under `advisory`, `attested_bylines`, and a
hypothetical fourth mode. The mode label only flips `run start`
behavior for `sealed_patch`. The DESIGN_SYNTHESIS asserts that
provenance modes are "schema-visible" but disclaims runtime semantics
("Each mode includes clear 'does not prove' text"); the SPEC text as
shipped does the opposite, ascribing behavioral consequence the
runtime never delivers.

**Why this matters.** This is the exact "named honestly but enforced
advisorily" failure mode RFC 0027 was written to prevent. A user
toggling their workflow from `advisory` to `attested_bylines`
expecting a behavioral gate to flip sees no change. Worse, the new
SPEC text invites that expectation in the same paragraph that defines
the mode.

**Required revision.** Pick one of two paths and commit to it:

- (a) Make `attested_bylines` mean something the other modes do not.
  Reasonable choices: auto-promote every review job's
  `require_attested_lane` to `true` under this mode; raise a doctor
  warning when the mode is declared but no lane has a process-adapter
  command or any sessions are unattested; surface a clear status
  signal that distinguishes mode-declared honesty from
  mode-derived enforcement.
- (b) Update SPEC §"Provenance Modes" and the ubiquitous-language
  entry to state plainly that under V1 `attested_bylines` is a
  *self-declared metadata label* with the same runtime semantics as
  `advisory`, and that lane-liveness attestation derives bylines
  under every mode.

Either fixes the overclaim. The current text does not.

### F2 — `docs/DECISION_LOG.md` reuses an existing decision id (high, docs integrity)

**Evidence.** Two rows in `docs/DECISION_LOG.md` are labelled `D080`:

- Line 24: "Accept RFC 0026 V1 plus RFC 0027 Phase 2 guardrails..."
- Line 81: "Accept RFC 0024 V4: pause/resume + per-job mutations..."

The old `D080` was already cited by `docs/dogfood/027/BUILD_HANDOFF.md`
and by the RFC 0024 row itself. The build's own BUILD_HANDOFF §"Human
Decisions" then declares "D080 records acceptance of RFC 0026 V1 plus
RFC 0027 Phase 2 guardrails", which now references *either* of two
distinct decisions depending on which `D080` the reader lands on.

**Why this matters.** The decision log is the durable appeal surface
for "why is this state accepted?". Decision ids are the only stable
handle a future reviewer has on a row whose narrative has been edited
or whose status has changed. A colliding id is not a typo; it breaks
every future citation, including the citation the build itself just
made.

**Required revision.** Renumber the new acceptance row to the next
unused id (D085 in this branch's history; D083/D084 exist on `main`
but not on this branch's merge-base, so the implementer should
re-check against `main` before picking) and update every citation in
this branch — RFC 0026/0027 status footers, `BUILD_HANDOFF.md`,
README, CHANGELOG, and any other doc that names "D080" — to point
at the chosen id.

Bonus (low): the new row's narrative calls the shipped surface
"RFC 0027 Phase 2 guardrails", but RFC 0027 §"Current Implementation
Status" and §"Phased delivery" both call mode surfacing **Phase 1**,
with RFC 0026 as the Phase 2 prerequisite. The build *does* ship
both Phase 1 (mode surfacing) and Phase 2 (RFC 0026), so the
narrative is selective rather than wrong, but the asymmetric naming
between RFC body and decision row hurts grep.

### F3 — RFC 0026 acceptance criteria are not end-to-end tested (high, missing tests)

RFC 0026 §"Acceptance Criteria" enumerates eight invariants. The
shipped tests cover only a subset:

| Acceptance criterion (paraphrased) | Test? |
|---|---|
| Unattested session publishing under lane byline is refused with exit code 6; valid `author: operator` succeeds | partial: `tests/test_cli_mvp.py:958-1040` covers the *publish-time* author-line check for unattested sessions (good); does not exercise *the runner-side expected line being `author: operator` for a session that did supervise-then-die*. |
| Attested supervised session publishes under lane byline (unchanged behavior) | partial: `tests/test_supervise.py:253-292` asserts the *packet's* `author.line` but not the *published artifact's* byline. |
| Killing the supervised process between register and publish downgrades the next publish to `author: operator` | **missing** |
| Re-attaching a fresh supervisor restores the attested byline on subsequent publishes | **missing** |
| `require_attested_lane: true` refuses verdict from unattested session | covered: `tests/test_cli_mvp.py:1043-1099`. |
| `register-session` JSON includes `lane_attestation: "unattested"` and the supervise hint | covered: `tests/test_cli_mvp.py:701-719`. |
| `--operator-label` publishes with `author: operator [self-declared: <label>]` | **register-time-only**: `tests/test_cli_mvp.py:701-719` covers the JSON envelope; no test writes an artifact with `author: operator [self-declared: foo]` and asserts the publisher accepts it for a labelled session and refuses the unlabelled `author: operator` for the same session. |
| `evidence export` bundles distinguish attested from unattested bylines in rendered output | **missing**: the snapshot JSON includes `lane_attestation` (good), but no test asserts the per-bundle byline distinction the criterion requires. |

The RFC also writes, plainly: "Unattested-session publishes get new
tests in `tests/test_lane_attestation.py`." That file does not exist
in this build. The implementer chose to scatter tests across
`tests/test_cli_mvp.py` and `tests/test_supervise.py` instead. Either
choice is defensible, but the RFC text and the build disagree, and
the absent scenarios above are absent regardless of where the file
lives.

**Why this matters.** The RFC reacts to a real, observed forgery
shape (operator surrogates publishing under lane bylines without a
running lane). The forgery is a *publish-time* act. End-to-end
publish tests are the only ones whose passage proves the forgery
shape is closed. Structural unit checks of `register-session` JSON
and packet author lines do not.

**Required revision.** Add the missing scenarios. The
`tests/test_supervise.py` fixture already demonstrates how to start,
mutate, and kill a supervised process; the kill-between-register-
and-publish and re-attach scenarios reuse that machinery directly.

### F4 — `supervise send` trusts only pid liveness while `session_lane_attestation` requires pid identity (medium, behavioral)

**Claim under review.** RFC 0026 V1 (also in `docs/SPEC.md` §"Byline
Integrity") says the trusted-edge invariant for a supervised session
is "the recorded pid is alive *and* the Linux `/proc/<pid>/stat`
start-time token still matches *and* the supervisor command equals
the snapshot lane command."

**Evidence the delivery path is weaker.**
`src/striatum/identity.py:185-213` (`_inactive_reason`) implements
the full invariant. `src/striatum/supervisor.py:495-...`
(`deliver_packet_to_attached_supervisor` / `supervise_send`) checks
only `_pid_alive(pid)` before writing to the supervisor's stdin
FIFO. If the supervised child has died between `supervise start`
and a later `supervise send` and the OS has reused the pid (rare on
short runs, plausible on long-lived ones), `_pid_alive` returns
True. The send writes packet bytes into the named pipe (which lives
independent of the original child). The new pid is almost certainly
not reading that pipe; the kernel will buffer until full, then
block.

**Why this matters.** It is not a forgery primitive — bylines still
downgrade correctly. But it is a behavioral inconsistency between
two trusted-edge code paths that both claim to inspect the same
supervisor binding. RFC 0026's framing requires that one binding
mean the same thing in both places, and a future security audit
will reasonably ask "what does an attached supervisor row mean?".
The honest answer must not depend on which CLI verb is calling.

**Required revision.** Reuse `_inactive_reason` (or an extracted
helper) inside `supervise_send` / `deliver_packet_to_attached_supervisor`
so the send refuses on `pid_identity_mismatch`,
`pid_identity_unavailable`, and `lane_command_mismatch`. The lazy
`_mark_supervisor_lost` semantics the attestation path already
uses apply unchanged.

### F5 — `RUN_SUMMARY.md` rendering omits the new session fields SPEC promises (medium, doc/code drift)

**Claim under review.** `docs/SPEC.md` (Sessions section) now states:
"`evidence export` and `run summary` include a per-session block
with each session's `state`, `closed_at`, `close_reason`,
`lane_attestation`, `operator_label`, and (when set by HARNESS-003
override) `non_fresh_reason`."

**Evidence.** `src/striatum/cli/evidence.py:441-462`
(`evidence_session_summaries`) does select and emit `operator_label`
and lane_attestation fields into the JSON snapshot — so the *data*
makes it into evidence export. **But**
`src/striatum/cli/run_summary.py:266-283`
(`render_run_summary_markdown` "## Sessions" block) renders only
`slug`, `state`, `closed_at`, `close_reason`, and
`non_fresh_reason`. The new attestation/label fields are silently
dropped from the Markdown output.

The SPEC's "run summary" claim has two reasonable readings:

- "The summary's JSON data carries these fields" — partially true
  (status JSON in the summary's JSON does, but the durable Markdown
  surface is what humans actually read for run provenance).
- "The summary's rendered Markdown lists these per-session" — false.

The dogfood-030 SPEC change is unambiguous in the text; the
implementation half is missing.

**Why this matters.** Run summaries are the durable, human-readable
artifact a future reader uses to understand who reviewed what.
HARNESS-003 already proved this surface is load-bearing for byline
integrity. Promising attestation on this surface and then not
rendering it puts the SPEC and the artifact out of sync exactly
where readers will rely on the contract.

**Required revision.** Either render `lane_attestation` and
`operator_label` (when present) in the run-summary Markdown session
block, or trim the SPEC sentence to claim only what the JSON
snapshot actually surfaces.

### F6 — Decision-record byline contract is ambiguous (medium, RFC vs implementation)

**Claim under review.** RFC 0026 §"Non-Goals" states "No change to
`decision record`'s authority. Decision artifacts remain
operator-authored under the operator byline introduced here."

**Evidence.** `src/striatum/cli/mutations.py`'s
`render_decision_markdown` emits `owner: human` in the front matter
but no `author:` line in either the front matter or the title
block. Decision artifacts skip
`validate_optional_markdown_author_line` (the publisher path is the
`decision record` route, which writes the file itself). So the
"operator byline introduced here" never lands on decision
artifacts.

**Why this matters.** This is the smallest of the doc/runtime
gaps, but it is the one most likely to surface in a future audit
("the RFC says decisions are operator-authored under the new
byline; show me where") and get a no-hits grep.

**Required revision.** Either add `author: operator` to the
decision template (and ensure the decision write path agrees with
the publisher's title-block author scan), or rewrite RFC 0026
§"Non-Goals" to say decisions are owner-typed and do not carry an
artifact author byline. The current text supports neither reading
unambiguously.

### F7 — Stray `foo` file at repo root (low, hygiene)

`git status` reports an untracked file `foo` at the repo root
containing what looks like captured `striatum status` JSON output.
It is not part of any expected artifact, fixture, or build output;
it is residue from interactive dogfooding.

**Why this matters.** Project change discipline (`AGENTS.md` §
"Change Discipline") says: "Do not commit `.striatum/`, `.venv/`,
caches, egg-info, transcripts, or private diagnostics." A stray
`foo` is the smallest possible violation, but it is the kind of
thing a Striatum dogfood is supposed to *prevent* operators from
leaving behind.

**Required revision.** Remove `foo` from the working tree before
the build branch lands.

### F8 — Cross-platform attestation behavior is asserted in prose only (low)

`process_start_time` (`src/striatum/identity.py:249-267`) reads
`/proc/<pid>/stat`. On macOS or Windows it returns `None`.
`_inactive_reason` then returns `pid_identity_unavailable`, and
every supervised session on those platforms is permanently
unattested.

SPEC §"Byline Integrity" admits this in prose ("Platforms that
cannot provide a stable process-start token are unattested rather
than silently upgraded"). The build adds no test to lock that
contract in. There is also no `supervise start` stderr hint that
mentions *why* the session won't attest on those platforms; a
first-time macOS operator sees `attestation: unattested` after
starting a real supervisor and has no obvious diagnosis.

**Why this matters.** SPEC prose is not a verified invariant. A
future refactor that uses `psutil.Process(pid).create_time()` (a
reasonable cleanup that would unify the start-time read across
platforms) would silently upgrade macOS/Windows attestation
*without* implementing the start-time identity check those
platforms can also be made to support. The unit test is the only
thing standing between that refactor and a quiet provenance
overclaim.

**Required revision.** Add a unit test that monkeypatches
`process_start_time` to return `None` and asserts the publisher
expected byline downgrades to `author: operator`. Optional: add a
`supervise start` stderr hint when start-time tokens are
unavailable on the local platform.

### F9 — `_pid_alive` swallows `PermissionError` (low, latent)

`src/striatum/identity.py:270-277` treats both `ProcessLookupError`
and `PermissionError` from `os.kill(pid, 0)` as "dead". Today
supervisors run as the same uid as the runner, so this is moot. The
moment RFC 0027 explores separate-Unix-user containment for the
sealed-patch authority model — which the synthesis explicitly
contemplates (Phase 5) — this becomes a real bug: a healthy
supervised process running as a different uid will probe as "dead",
its supervisor will be marked lost, and its publishes will downgrade
incorrectly.

**Why this matters.** Not an RFC-acceptance blocker today. Worth a
follow-up TODO row so the sealed-mode design does not collide with
this in eight months.

**Required revision.** Distinguish `PermissionError` from
`ProcessLookupError`. On `PermissionError`, the pid exists but is
not the runner's to probe; the correct call is `True` plus a note
that pid-identity probe cannot run for this pid under the current
uid.

### F10 — Release hygiene: pyproject not bumped, CHANGELOG still `Unreleased` (low, release hygiene)

`pyproject.toml` reads `1.20.1`. `CHANGELOG.md` has the RFC 0026 +
RFC 0027 entry under `## Unreleased`. The project pattern (and the
user-memory note recorded for this repo: each landed RFC bumps
`pyproject.toml` minor, promotes CHANGELOG, and tags the merge)
expects the build to land as `1.21.0`. Not an RFC acceptance
criterion, but the build cannot ship cleanly without it.

**Required revision.** Bump `pyproject.toml` to `1.21.0`; promote
the `Unreleased` block to `1.21.0 — 2026-05-11` (or the actual
merge date); leave a fresh `Unreleased` heading. Tag at merge per
the established convention.

## Non-Blocking Observations

- **Web read views unchanged.** DESIGN_SYNTHESIS line 161 directs
  "Web UI starts read-only: mode chips, attestation labels, patch and
  receipt detail pages, and doctor warnings." A `grep -rn
  "provenance_mode\|lane_attestation\|operator_label" src/striatum/web/`
  finds no hits. Not blocking V1 (the synthesis frames it as the
  starting surface, not the gate), but it should not be claimed as
  done. The doctor warning surfaces in `striatum doctor` JSON; the
  web view does not consume it.
- **`supervise_list` does not surface attestation.** `supervise status`
  was updated to include `lane_attestation` /
  `lane_attestation_reason` (`src/striatum/supervisor.py:457-459`).
  `supervise_list` was not. RFC 0026 §"Surface visibility" promises
  only `supervise status` and `striatum status`, so this is scoped
  correctly, but a reader who relies on `supervise list` to find
  unattested supervisors won't.
- **Title-block author scan reads only the first 40 lines after the
  front matter** (`src/striatum/artifacts.py:559-565`). An artifact
  whose first `author:` line lives below line 40 of the title block
  slips past `validate_optional_markdown_author_line`. Not
  exploitable in practice — the convention puts the byline near the
  top — but the 40-line bound is hardcoded with no comment about why.
- **`session_mismatch` and `run_mismatch` reasons in
  `_inactive_reason` are mostly unreachable** because the SQL query
  already filters by `session_id` and the partial unique index
  forbids more than one active supervisor per session. The defense
  is fine, but `session_mismatch` is dead code. Worth either deleting
  or commenting why it remains.
- **`supervise_start` return shape duplicates attestation derivation**
  (`src/striatum/supervisor.py:225`): it returns `"attested" if
  pid_start_time is not None else "unattested"` rather than calling
  `session_lane_attestation`. The result agrees with
  `session_lane_attestation` today, but it is a second source of
  truth that can drift.

## Survival Test — What Does V1 Actually Buy?

To be fair to the build: the byline-honesty machinery *is* in the
right place.

- `expected_author_line` (`src/striatum/artifacts.py:569-594`)
  derives the byline from `session_lane_attestation` and the session's
  `operator_label` at publish time.
- `validate_optional_markdown_author_line` (lines 487-508) refuses
  non-matching bylines with the existing exit-code-6 chokepoint, and
  `tests/test_cli_mvp.py:958-1040` exercises that path for the
  unattested case (which the prior round of this review missed).
- Migration v12 (`src/striatum/migrations.py:408-417`) is additive,
  idempotent, and uses the existing nullable-column pattern.
- `_inactive_reason` correctly enforces the four-factor binding
  (run, pid liveness, pid start-time, lane command).
- `sealed_patch` runs refuse to start unconditionally
  (`mutations.py:192-196`) — the cleanest honesty surface in the
  build, and the prior reviewer's praise is deserved.

The substantive complaint is not "the machinery is wrong." It is
"the printed claims and the test coverage make the V1 acceptance
gate dishonest in places the V1 acceptance gate must not be
dishonest, because the *whole point* of RFC 0026 is that *what the
SPEC says* matches *what the runtime does*." Closing F1, F2, F3, F4,
and F5 puts the surface in agreement with the runtime; F6-F10 are
either local cleanup or follow-up TODOs.

## Counterarguments Considered

- **"`attested_bylines` is meant as advisory metadata for now; the
  SPEC sentence is meant to describe the *concept*."** If so, write
  that sentence. The current text reads as a runtime claim, in the
  same paragraph that distinguishes the three modes by behavior.
- **"The publish-time downgrade for unattested sessions is already
  proven by `test_publish_artifact_validates_optional_markdown_author_line`."**
  Partly — that test does cover the default unattested case, and the
  prior round's F1 was overstated. But the kill-between-publishes
  and re-attach-restores cases the RFC enumerates as Acceptance
  Criteria are still untested.
- **"PID reuse in `supervise send` is rare in practice."** True.
  But the runner publishes its trusted-edge contract in
  user-visible SPEC text, and that contract should mean the same
  thing in every code path that consults it. Two-line fix.
- **"D080 collision is editorial."** Sure, but it is also concrete:
  the build's own BUILD_HANDOFF cites `D080` in a way that no longer
  resolves uniquely. Trivial to fix in this revision round; harder
  later.

## Suggested Sequencing of the Revision

1. Renumber the new decision row, update every citation
   (BUILD_HANDOFF, README, CHANGELOG, RFC 0026/0027 footers) (F2).
2. Decide and update SPEC + ubiquitous-language wording for
   `attested_bylines` — either make it bind or admit it does not
   (F1).
3. Add the missing publish-time scenarios to `tests/`. The RFC
   names `tests/test_lane_attestation.py`; either ship that file or
   note in the RFC that the tests live elsewhere (F3).
4. Tighten `supervise_send` to share `_inactive_reason` (F4).
5. Make the run-summary Markdown render the new session fields, or
   walk back the SPEC sentence (F5).
6. Decide the decision-byline contract once and reconcile RFC and
   template (F6).
7. Remove `foo`; bump `pyproject.toml`; promote CHANGELOG (F7, F10).
8. Optional but recommended: add the macOS/Windows monkeypatch
   test (F8) and the `PermissionError` distinction TODO (F9).

After these, the V1 acceptance gate is honest. Until then, a
devil's advocate cannot vote accept.
