---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["devils_advocate", "rfc-0026", "rfc-0027", "provenance", "design"]
---

# Devil's Advocate Review: RFC 0026 + RFC 0027 Implementation Synthesis

author: reviewer-claude-opus-001
date: 2026-05-11
status: draft
target: docs/dogfood/030/DESIGN_SYNTHESIS.md

## Posture

This is a devil's-advocate review. The synthesis must survive
counterarguments at its strongest claims before its plan is implementation-
ready. The arguments below are framed against the artifact's claims; the
goal is to surface concrete revisions before any code lands.

## Summary

The synthesis is broadly competent: it carves the work into honest phases,
keeps `sealed_patch` from shipping before containment, and flags the
overclaim risk in receipts and bylines. But the document still ships
several **load-bearing claims that the plan does not actually deliver**,
plus several **specific design holes** that would either land as bugs in
Phase 1 or force schema rework before Phase 3. The verdict is
`needs_revision`. Minimum revisions are enumerated in §10.

## 1. The framing "byline honesty" overclaims its own scope

The synthesis names "byline honesty is not source-byte provenance" as the
core product rule. But the document then repeatedly markets Phase 1 as
delivering **byline honesty** ("Phase 1: RFC 0026 byline honesty") and
treats lane attestation as the load-bearing primitive that makes future
phases meaningful.

That framing is itself dishonest. RFC 0026 §"Non-Goals" explicitly admits:

> Lane attestation in this RFC means *"a process from this lane's command
> is alive on the recorded pid for this session"*, not *"this artifact
> was authored by that process"*.

A "live pid" tells you a process exists. It does not tell you the bytes
in `findings.md` came from that process's stdout. With D028 forbidding
broad transcript capture, the runner has no mechanism to bind artifact
bytes to a supervised process. Once an operator runs `supervise start`,
they have **fully attested** sessions under which they can ghost-write
arbitrary content under a lane-typed byline. The current synthesis frames
this away as "forgery becomes a deliberate act with a visible trace, not
a frictionless slip." But the visible trace is **identical** to a
legitimate run: supervisor row, live pid, lane-typed artifact. There is
no per-publish evidence that distinguishes the cases.

So what Phase 1 actually delivers is **lane-liveness attestation**, not
byline honesty. Calling it "byline honesty" creates exactly the
overclaim the synthesis says it wants to avoid: downstream consumers
(findings ledgers, syntheses, cross-lane convergence calculators) will
treat `attested == True` bylines as model-authored when only one of two
forgery paths has been closed.

**Required revision:** rename Phase 1 from "byline honesty" to
"lane-attestation gates" everywhere in the synthesis, the staging plan,
docs/SPEC.md, RFC 0026 docs/UBIQUITOUS_LANGUAGE.md updates, the
`provenance_mode` docstring for `attested_bylines`, and `doctor` output.
Document explicitly that "attested" means **a process from this lane is
alive**, not **bytes came from that process**.

## 2. The `attested_bylines` mode label is itself overclaim

The synthesis Phase 2 introduces three modes: `advisory`,
`attested_bylines`, `sealed_patch`. The middle name is the same overclaim
as §1: it implies bylines are attested when only **lane liveness** is
attested.

A more honest label is `lane_attested` or `attested_lane_liveness` or
`role_typed_bylines_when_attested`. The name `attested_bylines` will leak
into spec docs, status output, evidence export legends, dashboard chips,
and operator mental models. By the time the team realizes the label is
misleading it will be a baked-in part of the workflow JSON schema and
expensive to rename.

**Required revision:** pick a mode name that does not assert authorship.
Recommendation: `lane_attested`. If the team prefers
`attested_bylines`, then the synthesis must add explicit per-surface
"does not prove authorship" language equivalent to what is currently
proposed only for `sealed_patch` and evidence export.

## 3. `os.kill(pid, 0)` is a racy attestation primitive

RFC 0026 §"Attestation as a derived property of a session" specifies the
liveness probe as `os.kill(pid, 0)` and the synthesis carries this
forward via `session_lane_attestation`. This is **incorrect by
construction** because Unix recycles pids.

Sequence:

1. Operator runs `supervise start --session-id S` against lane `codex`.
2. The supervised codex process is forked with pid 12345.
3. The codex process exits naturally or is killed.
4. The OS reuses pid 12345 for an unrelated process (a shell, a `sleep`,
   anything the operator runs in the same shell).
5. `os.kill(12345, 0)` returns true.
6. Striatum reports `lane_attestation: "attested"`.
7. Publisher derives lane-typed byline. Operator publishes ghost-written
   content under `author: reviewer-codex-gpt-5.5-001`.

Pid recycling is not a hypothetical adversarial threat. On a normal Linux
desktop with default `/proc/sys/kernel/pid_max=32768`, pid wraparound
happens within hours under typical interactive shell use. This is a
real, frequent, **non-adversarial** failure mode that the current spec
silently mislabels as attested.

**Required revision:** the attestation probe must disambiguate by
start-time. On Linux, read `/proc/<pid>/stat` field 22 (`starttime`) at
supervisor row creation, persist it on `process_supervisors`, and
require both pid liveness **and** matching `starttime` to call the
session attested. On macOS/Windows, use the equivalent `kinfo_proc` /
`PROCESS_BASIC_INFORMATION` start times; if a platform cannot give a
reliable start time, sealed/attested modes must report unsupported on
that platform (consistent with the synthesis's own non-degradation
rule).

If `process_supervisors` does not already store start-time, this is a
new column and a new migration that the synthesis omits. The "no new
migration" claim in RFC 0026 §"Backwards compatibility" is wrong once
start-time is required for correctness.

## 4. `operator_label` lacks deceptive-pattern validation

RFC 0026 and the synthesis both render the operator label as
`author: operator [self-declared: <label>]`. The validation rule is
"short, single-line, ASCII; recommended grammar is lowercase letters,
digits, dot, underscore, and hyphen."

Nothing prevents:

- `--operator-label "claude-opus-001"` rendering as
  `author: operator [self-declared: claude-opus-001]`. To a glancing
  reader (or a downstream parser that splits on `: ` and reads the next
  token group), this looks identical to an attested model byline.
- `--operator-label "reviewer-claude-opus-001"` which is **literally an
  attested byline string** inside square brackets.
- `--operator-label "supervised"` or `--operator-label "attested"` which
  invert the meaning of the surrounding `operator` token.
- A label containing UTF-8 lookalikes if ASCII enforcement is not
  strict (the synthesis says ASCII; RFC 0026 just says "short,
  single-line"; they disagree).

The "recommended grammar" is not a validation rule. Recommendations are
not enforced; they are decoration.

**Required revision:**

- Make the grammar a **required** rule, not a recommendation. Reject any
  label not matching `^[a-z0-9._-]{1,64}$`.
- Reject labels matching the role-model-ordinal regex used for attested
  bylines. The implementation already has `expected_author_line`
  derivation; reuse it to refuse any label whose rendering would equal
  any allowed attested byline format.
- Reject labels matching reserved tokens that invert the meaning of the
  surrounding `operator` token: `attested`, `supervised`, `lane`,
  `model`, and any registered lane id from the active workflow.
- Add a test case to the Phase 1 test matrix proving these rejections.

## 5. Phase 4 receipt verification overclaims tamper detection

The synthesis Phase 4 testable milestone says:

> apply refuses each missing precondition with a specific error, receipt
> verification detects patch/tree/metadata drift, and exported receipts
> can be verified without broad transcripts.

But Phase 4 ships **before** Phase 5 (containment). In Phase 4 the
signing key still lives in operator-writable authority (because
containment doesn't exist yet). A sophisticated tamper rewrites the
SQLite row **and re-signs** with the same key. Receipt verification then
succeeds against the tampered state, because the verifier has the
public key and the new signature is valid.

The synthesis acknowledges this elsewhere ("apply and receipt output
must say `authority: advisory` or `authority: apply_gate_only`; it must
not call the result sealed") but then immediately writes a Phase 4
acceptance criterion that the implementation cannot deliver under those
constraints.

**Required revision:** rewrite the Phase 4 testable milestone to be
honest about what it actually proves:

- Receipt verification detects **unsophisticated** drift: SQLite row
  edit without re-signing, patch substitution, tree drift on a fresh
  checkout that hasn't loaded the operator's key.
- Receipt verification **does not** detect tamper by an operator that
  re-signs with the same key. This is a Phase 5 property only.
- Acceptance tests must include a positive case proving the
  re-signed-tamper path **succeeds** verification in Phase 4 (this is
  the negative test the staging plan says every phase must have).

Without this revision, the Phase 4 acceptance criterion is unfalsifiable
in a way that will be caught later by anyone trying to dogfood it.

## 6. Internal inconsistency: 5-phase plan vs 6-phase staging

The synthesis has two phase lists that do not match:

- "Phased Plan" lists 5 phases. Phase 5 bundles containment, signing key
  custody, and "the narrow local signed-commit exception be enabled."
- "Staging Plan To Avoid Overclaiming" lists 6 phases. Step 5 is "Ship
  the first containment mechanism." Step 6 is "Ship sealed-mode signed
  local commits after the decision-log/spec carve-out is accepted."

These are incompatible. The Phased Plan says signed commits ship in
Phase 5; the Staging Plan says signed commits ship in step 6 **after a
decision-log/spec carve-out is accepted**. The decision-log/spec
carve-out is itself a human-decision question (§Human-Decision Questions
#7) that has no answer in the synthesis.

If signed commits ship in Phase 5 without the decision-log carve-out,
the synthesis violates its own staging discipline. If they ship in a
separate Phase 6, the Phased Plan section is wrong about Phase 5's
scope.

**Required revision:** pick one of the two structures. Recommendation:
keep the 6-phase structure (with signed commits as a distinct phase)
because it preserves the carve-out as a gating decision. Then rewrite
the Phased Plan section to match: Phase 5 ships containment, key
custody outside operator authority, and `sealed_patch` runs starting
successfully; Phase 6 ships signed local commits **after** the
decision-log/SPEC carve-out is accepted in a separate human-decision
artifact.

## 7. Downstream workflow migration is unaddressed

The synthesis says:

> The largest compatibility break is intended: manually driven
> unattested sessions can no longer publish lane-typed bylines.

Then prescribes a fixture audit in this repository: "audit work-packet
assembly, artifact publishing, evidence export, run summary, web views,
and verdict rendering."

But Striatum is **a standalone, local-first workflow runner for
terminal-based AI coding agents** (per `AGENTS.md`). The synthesis
treats this as if Striatum-the-repository owns all the workflows. It
doesn't. Downstream users have their own target repositories with their
own `workflow.json` files and their own dogfood histories. After this
upgrade lands, **every downstream workflow that uses lane-typed bylines
without supervise** breaks silently at publish time with exit code 6,
with no migration path and no tooling.

The synthesis offers:

- `doctor` diagnostics — but `doctor` runs against `.striatum/state.sqlite3`
  in a running target repo; it does not diagnose workflow files at rest.
- Fixture updates — for fixtures **in this repository only**.

**Required revision:** the implementation must ship at least one of:

- A `striatum workflow lint <path>` command that flags workflows whose
  review jobs publish lane-typed bylines but lack `supervise start` in
  their normal operator flow. Or:
- A `striatum status` warning when an existing run is configured for
  lane-typed bylines without an active supervisor binding, surfaced
  before the next publish refusal. Or:
- A `striatum workflow migrate <path>` command that inserts
  `require_attested_lane: true` into review jobs and emits a diff for
  operator review.

Without one of these, the upgrade story for downstream users is "your
workflow worked yesterday; today every publish fails with exit code 6,
read the upgrade notes." That is not a tolerable upgrade story for a
standalone runner with no central rollout.

## 8. Patch capture semantics for unattested sessions are unspecified

Phase 3 introduces `patch_artifacts` with field
`producer_supervisor_id`. The synthesis nowhere specifies what this
field contains when the producing session is **unattested** (e.g., a
workflow in `advisory` or `attested_bylines` mode where the work is
operator-driven without supervise).

Possibilities:

- NULL: the field is nullable. Then `producer_supervisor_id` is not a
  reliable join key for downstream consumers, and the implementation
  needs to handle NULL everywhere it joins on this column.
- A sentinel value: a fixed UUID meaning "no supervisor." Then the
  schema constraints (likely a foreign key to `process_supervisors`)
  break.
- Refuse the capture: `patch capture` fails on unattested sessions.
  Then advisory-mode workflows cannot capture patches at all, which
  contradicts the synthesis's "Phase 3 can ship before containment"
  story.

The synthesis owes a specification. Same question for `producer_session_id`
when the operator is acting without a registered session (which is
allowed today for some recovery flows).

**Required revision:** state the actual policy in §"Schema And Migration
Changes" Phase 3 migration. The author recommends: `producer_supervisor_id`
is nullable; `producer_session_id` is **required**; capture refuses
when there is no claimable session. Then add to the test matrix:
"patch capture from an unattested session writes
`producer_supervisor_id = NULL` and the patch artifact is correctly
labeled advisory in evidence export."

## 9. `require_attested_lane` placement on non-review jobs is ambiguous

The synthesis Phase 1 limits `require_attested_lane` to **review jobs
only** ("Non-review use should be deferred"). But RFC 0026 §"Optional
opt-in for hard refusal" describes the field as applying to "review
jobs (and, for symmetry, on lanes)."

Phase 1 implementation question the synthesis does not answer:

- If a workflow author writes `require_attested_lane: true` on a
  `build` job in Phase 1, what happens?
  - Workflow validation rejects the workflow at registration time
    (safe; declares the field's scope explicitly)?
  - Workflow validation accepts and silently ignores at runtime
    (unsafe; the workflow author thinks the build job is gated when it
    isn't)?
  - Workflow validation accepts and applies the gate (this is the
    "non-review use deferred" path, contradicted)?

The synthesis must pick one. Silent ignore is the worst possible
answer and is what implementations default to when no rule is written
down.

**Required revision:** state explicitly that Phase 1 workflow
validation rejects `require_attested_lane` on non-review job types
with a clear error message, and add a test for it. Phase 1 must not
silently accept-and-ignore.

## 10. Cross-platform "fail loudly" is undefined

The synthesis says non-Linux platforms attempting `sealed_patch`
"should fail loudly rather than skipping into advisory semantics." But
"loudly" is not a spec. Concretely:

- What is the exit code? (`run start` exits non-zero, but with what
  specific code? Operators will branch on it in scripts.)
- Where does the error surface? (`run start` stderr only? `doctor`
  output? `status --json` field?)
- Is the failure deterministic across CI runners? CI may not have the
  authority-probe utilities installed; the failure must distinguish
  "platform unsupported" from "probe tools missing."
- What does the workflow validator do at `workflow validate` time
  before `run start`? If `sealed_patch` is in the JSON but the host
  is unsupported, does workflow validation refuse, or does the
  workflow load successfully and only `run start` refuses?

These are not nitpicks. Without a specific surface, the implementer
will pick one and a downstream user who scripted against a different
choice gets paged in production.

**Required revision:** specify exit codes (recommended: reuse the
existing `InvalidTransitionError` exit code path so the operator's
existing error handling catches it), specify the stderr template, and
specify that workflow validation succeeds while `run start` refuses
(because workflow validity is platform-independent; runtime authority
is not).

## 11. Path validator: symlinks, case-folding, partial overlap

The synthesis Phase 2 schema rule for `protected_paths` and
`operator_writable_paths` is:

> Paths must be repo-relative, must not contain `..`, must not be
> absolute, must not target `.striatum/`, and protected/operator-writable
> sets must not overlap.

This is incomplete:

- **Symlinks**: if `src/` is a symlink to `/etc/secrets`, the validator
  cannot statically detect the redirect. The containment mechanism
  (Phase 5) then has a hole. The synthesis must require either: (a)
  no symlinks in protected/operator-writable paths at run-start time,
  with refusal if any exists; or (b) symlinks resolved and the
  resolved targets checked.
- **Case-folding**: macOS HFS+ is case-insensitive by default. If
  `protected_paths` contains `Src/` and `operator_writable_paths`
  contains `src/`, the validator's overlap check (presumably
  string-equality on the prefix) says no overlap, but the filesystem
  treats them as the same directory.
- **Partial overlap**: `src/` protected and `src/docs/` operator-
  writable. The synthesis says "must not overlap" but doesn't define
  overlap. The plain reading is "any path prefix shared," which would
  refuse this (probably correct). The implementation will need this
  spelled out.
- **Trailing slash semantics**: `src` vs `src/` — are they the same
  path? Workflow authors will write both.

**Required revision:** spell out the canonicalization rule: paths are
normalized as POSIX-style, trailing slashes stripped, then prefix-
compared. Symlinks in either set are refused at workflow validation
with a clear error. Case-folding is OS-dependent and must be probed at
`run start`; if the host filesystem is case-insensitive and the
declared paths only differ in case, refuse.

## 12. Test matrix is light on adversarial cases

RFC 0027 §"Acceptance Criteria" explicitly demands:

> A passing implementation must demonstrate adversarial behavior, not
> only happy paths.

The synthesis test matrix is essentially happy-path with refusal
cases. Concrete gaps:

- **Pid recycling** (see §3): no test that a recycled pid does not
  count as attested.
- **Re-signed tamper** (see §5): no negative test that the receipt
  verifier correctly does not detect re-signed tamper in Phase 4.
- **Operator label mimicry** (see §4): no test that a label of
  `claude-opus-001` is refused.
- **Sealed mode partial application**: no test that if `bwrap` denies
  writes to `src/` but `tests/` is still writable due to a config
  bug, the run start probe catches the gap.
- **Containment escape via setuid binaries, extended attributes,
  hardlink races, /proc/<pid>/root**: not enumerated. The synthesis
  treats containment as a single boolean ("operator can write
  protected path: yes/no") but real mechanisms have many bypass
  surfaces.
- **Concurrent supervisor death during publish**: the synthesis picks
  RFC 0026 open question (a) ("publish-time truth wins") but the test
  matrix does not include the case where the supervisor dies between
  the publisher's attestation check and the row insert. The pid liveness
  probe and the artifact row write are not atomic.

**Required revision:** expand the test matrix to include each of the
adversarial cases above. Each phase must demonstrate the negative test
the staging plan requires.

## 13. Migration of `process_supervisors` for start-time field

Per §3 above, the attestation probe must consult start-time. The
synthesis claims RFC 0026 needs no migration on `process_supervisors`:

> The `process_supervisors` table is unchanged. No new migration.

This claim depends on the racy `os.kill(pid, 0)` primitive being
acceptable. If §3's revision is adopted, `process_supervisors` gains a
new nullable column `pid_starttime TEXT` (or similar) and a new
migration.

**Required revision:** add this migration to Phase 1's "Schema And
Migration Changes" section. Existing rows have `pid_starttime = NULL`
and the attestation probe treats NULL start-time as **unattested**
(forward-only safety: rows from before the upgrade do not retroactively
become attested).

## 14. The `verdict_review_targets` side-table choice and its blast radius

The synthesis recommends a side table for verdict review-target
metadata. Fine. But the existing verdict-rendering code in
`evidence export`, `status`, `why`, dashboard, web run/job views, and
verdict-quoting in synthesis prompts **all read verdicts via existing
helpers**. Adding a side table means every read surface that wants the
reviewed-digest information needs to join through the side table.

The synthesis lists the surfaces but does not enumerate the read paths.
It also does not state whether prior verdicts (no row in
`verdict_review_targets`) render as "unbound" in those surfaces, or
whether the side-table absence is treated as "advisory verdict OK for
display but not for sealed apply."

**Required revision:** specify the render rule. Recommendation: a
verdict with no `verdict_review_targets` row renders normally as today
(no UI change); a verdict with a row renders with a small
`reviewed: <short_digest>` chip in dashboard/web; evidence export
includes the full row when present.

## 15. The `claim-next` work-packet attestation snapshot

The synthesis says:

> Work packets should include the current attestation snapshot, but
> publish-time truth wins if the supervisor dies after claim.

This picks RFC 0026 Open Question (a), which is fine, but the resulting
operator experience is bad. Concrete sequence:

1. Operator runs `supervise start`. Session is attested.
2. Operator runs `claim-next`. Work packet's `expected_author_line` is
   `author: reviewer-codex-gpt-5.5-001`.
3. Lane works on the artifact; writes the front-matter author line as
   instructed by the work packet.
4. Supervised process exits cleanly (codex completed and shut down).
5. Operator runs `publish-artifact`. Publisher re-derives expected
   line as `author: operator`. The artifact file's
   `author: reviewer-codex-gpt-5.5-001` mismatches. **Exit code 6.**

This is a frictionful upgrade path: the lane did its job exactly as
the runner told it to, and the publish fails on a race. The operator
has to manually rewrite the artifact's front matter to publish.

The synthesis acknowledges this trade-off implicitly but does not
specify the recovery path. Options:

- Auto-rewrite the author line at publish time. (Reasonable but
  changes file contents on the operator's behalf, which violates the
  current artifact-bytes-are-truth invariant.)
- Refuse with a clear message including the new expected line.
  (Frictionful but honest.)
- Allow a flag like `--accept-downgrade` on `publish-artifact`. (More
  operator surface; needs auditability.)

**Required revision:** pick one and document it. Recommendation:
refuse with a clear stderr message **and** add a `striatum artifact
downgrade-author <path>` recovery verb that rewrites the byline
in-place and records an event. The recovery verb is auditable and
explicit; auto-rewrite is silent and surprising.

## 16. Receipt key bootstrap chicken-and-egg

The synthesis Phase 4/5 says:

> Add `apply_receipts` or a `receipt` artifact kind plus a receipt
> metadata table after the receipt format is locked.
> ...
> V1 should use a local signing key whose private material is outside
> the operator's writable authority.

Question the synthesis does not answer: **who creates the key**?

If the operator runs `striatum keys init`, then the operator briefly
held the key before it was moved outside operator authority. Forever,
that key is compromised; its private bits transited operator memory and
storage. The synthesis's own threat framing — "no defense against
direct SQLite tamper by an adversarial operator with full local
authority" — applies equally to "no defense against the operator
exfiltrating the signing key during init."

Two viable answers:

- The first containment mechanism (Phase 5) is also the key bootstrap
  authority; it generates the key in a privilege boundary the operator
  cannot reach. `striatum keys init` only **requests** key creation
  from the apply service.
- The signing key is per-machine and provisioned at install time by a
  separate-user step the operator does not run. The synthesis would
  then need an install/setup section.

Either is fine. The synthesis must pick one.

**Required revision:** add a §"Key Bootstrap" subsection to the
synthesis specifying which authority generates the signing key, when,
and under what user identity. Then add a test: receipt verification
must fail for a run whose key material has ever touched operator-
writable storage.

## 17. Vague staging language: "if convenient"

The synthesis Verdicts section:

> Record an audit-only `attested_at_record_time` flag if convenient,
> but do not render it as authorship proof.

"If convenient" is not a design decision. Either the flag is part of
the schema in Phase 1 (and operators can build tools against it) or it
is not (and the synthesis should not mention it).

**Required revision:** decide. Recommendation: include it, because
audit reconstruction will want this signal even though it is not
authorship proof. Add the column to the `verdicts` table or to the
side-table in §14, with the explicit non-authorship docstring.

## 18. Read-only source access path is under-specified

The synthesis introduces "Optional `striatum source read` and
`striatum source grep` with sealed mode, so operators have read-only
source access without write authority." This is named once in CLI
changes and never re-referenced.

Concrete questions the synthesis does not answer:

- In `sealed_patch` mode, can the operator still use their shell's
  `cat src/foo.py` if the protected path is filesystem-readable? If
  yes, `striatum source read` is decorative. If no (the OS mechanism
  enforces read-deny), then read-deny is a much bigger ask than write-
  deny and is not in the Phase 5 acceptance criteria.
- Do `source read` / `source grep` go through the apply service, or
  are they runner-local? If runner-local, they bypass the containment
  boundary.

**Required revision:** strike the `source read` / `source grep`
commands from the V1 scope unless the synthesis specifies the
mechanism. Otherwise they are unimplementable vaporware that operators
will ask about.

## 19. Patch capture's `write_scope_validated` semantics

The `patch_artifacts` table includes `write_scope_validated`. The
synthesis describes capture as refusing out-of-scope paths and
forbidden paths. So when is `write_scope_validated` ever false?

Two readings:

- Capture refuses entirely on out-of-scope writes; the column is
  always `true`; the column is decorative.
- Capture records `false` when the workflow allows lax scope, e.g.,
  in `advisory` mode; downstream readers know the scope was not
  enforced.

The synthesis is ambiguous. Choose one.

**Required revision:** specify when `write_scope_validated` can be
false. If never, remove the column.

## 20. `doctor` warnings list overlaps `status --json` without integration

The synthesis lists `doctor` warnings:

> warn for unattested sessions that publish operator bylines,
> unsupported sealed mode, writable protected paths, writable scratch
> paths, signing key in operator authority, and stale supervisor
> rows.

And separately says `status --json` should expose run
`provenance_mode`, per-session attestation, and patch/apply readiness.

These are two views of overlapping state but the synthesis does not
spec how they relate. Concretely: a stale supervisor row triggers a
`doctor` warning but `status --json` already shows the session as
`unattested` (computed from the same `process_supervisors` join).
Without a deduplication rule, operators get the same finding twice in
different shapes, and tooling consumers have to reconcile them.

**Required revision:** state the rule: `doctor` is the
human-actionable summary surface; `status --json` is the machine
surface. `doctor` warnings reference the underlying state
deterministically (e.g., "warning W007: session sess_abc unattested,
publish will downgrade to `author: operator`") and `status --json`
includes a `doctor_warnings: ["W007"]` array per session for tooling
consumers.

## 21. The "no opt-out" decision lacks an upgrade window plan

The synthesis Human-Decision Question #1:

> Should RFC 0026 ship with no legacy bypass, accepting immediate
> fixture/test churn, or with a short-lived opt-out? My recommendation
> is no opt-out.

The recommendation is reasonable in isolation, but the synthesis does
not say what release number this lands in, what minor-version
boundary, or how downstream users discover the breakage before they
upgrade. Striatum is a versioned package (CHANGELOG, pyproject.toml).
The synthesis owes a release-note shape: "v1.X.0: lane attestation
required for lane-typed bylines. Upgrade guide: …"

**Required revision:** the synthesis should specify that RFC 0026
ships in a single minor-version bump with a CHANGELOG entry under a
named release. The release notes must explicitly list:

- The exit code published artifacts will start returning (6).
- The doctor warning code that surfaces the impending downgrade.
- The migration command (§7).
- A direct link to a migration guide in `docs/`.

Without this, "no opt-out" is correct on the merits but operationally
inhumane.

## Minimum Concrete Changes Required Before Implementation

To upgrade this verdict from `needs_revision` to `accept_with_findings`,
the synthesis must:

1. Rename Phase 1 from "byline honesty" to "lane-attestation gates"
   throughout, and update the `attested_bylines` mode name to one that
   does not assert authorship (§1, §2).
2. Specify start-time disambiguation in the attestation probe, with a
   new nullable `pid_starttime` column on `process_supervisors` and a
   migration in Phase 1 (§3, §13).
3. Tighten `operator_label` validation: required grammar, refusal of
   attested-byline mimicry, reserved-token list, ASCII strict (§4).
4. Rewrite the Phase 4 receipt-verification testable milestone to be
   honest about its scope (no detection of re-signed tamper) and add a
   negative test proving the re-signed-tamper case (§5).
5. Resolve the 5-phase vs 6-phase inconsistency; pick the 6-phase
   structure with signed commits gated on a separate decision-log
   carve-out (§6).
6. Ship a workflow lint/migrate command for downstream users, or
   commit to one of the two equivalent alternatives in §7.
7. Specify `producer_supervisor_id` and `producer_session_id` policy
   for unattested-session patch capture (§8).
8. State explicitly that Phase 1 workflow validation **rejects**
   `require_attested_lane` on non-review job types (§9).
9. Specify cross-platform "fail loudly" precisely: exit code, stderr
   shape, validator vs runtime split (§10).
10. Spell out path-validator canonicalization: symlinks, case-folding,
    trailing-slash semantics, "overlap" definition (§11).
11. Expand the test matrix with the adversarial cases listed in §12.
12. Specify the `verdict_review_targets` render rules across each
    surface (§14).
13. Specify the publish-time downgrade recovery path: refuse with a
    new recovery verb, not auto-rewrite (§15).
14. Specify key bootstrap authority and add the negative test (§16).
15. Decide on `attested_at_record_time` and remove "if convenient"
    (§17).
16. Strike or fully specify `striatum source read` / `source grep`
    (§18).
17. Specify when `write_scope_validated` is false, or remove the
    column (§19).
18. State the `doctor` / `status --json` deduplication rule (§20).
19. Add a release-notes plan to the synthesis covering the upgrade
    sequencing, doctor warning codes, and migration tooling links
    (§21).

## Strongest counterargument the synthesis already survives

For balance: the synthesis does correctly resist three temptations
that a weaker plan would have fallen into:

1. **It refuses to ship a `legacy_unattested_bylines` opt-out.** An
   opt-out would weaken the exact failure mode RFC 0026 closes. The
   synthesis is right to refuse it even at the cost of fixture churn.
2. **It refuses to ship `sealed_patch` runs before containment
   exists.** This is the load-bearing staging discipline. The
   synthesis enforces it via `run start` refusal and the staging
   plan's negative-test requirement.
3. **It defers macOS/Windows sealed support rather than silently
   degrading.** Silent degradation from sealed to advisory would be
   the worst possible failure mode. The synthesis takes the harder
   "explicit unsupported" path.

These three calls are correct and should not be unwound during
revision. The §10 verdict reflects fixable specification gaps in an
otherwise sound architectural direction, not a fundamental rejection
of the plan.

## Verdict

`needs_revision`. The plan is implementable and mostly right at the
architectural level. But the framing ("byline honesty",
"attested_bylines"), the attestation primitive (pid recycling), one
testable milestone (re-signed tamper), and a handful of specification
gaps would each surface as bugs, overclaim incidents, or downstream
pain within the first few releases. Address §10's 19 items, and this
upgrades to `accept_with_findings`.
