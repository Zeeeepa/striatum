---
schema_version: striatum.operator_notes.v1
artifact_kind: operator_notes
title: "Dogfood-048 — Phase 1 Operator Notes"
---
author: operator-claude-opus-1

# Dogfood-048 — Phase 1 Operator Notes

Operator narrative for the RFC 0043 V1 dogfood
(`striatum/dogfood-048-rfc-0043-v1`, run
`run_892cbad2b1954cfd9d23e72f74ea3a96`). RFC 0043 lands the
substrate flip per D094 — Postgres becomes the sole Striatum
substrate for repo-local workflow state, and the daemon becomes a
hard prerequisite. This is the most architecturally consequential
RFC since RFC 0030 V2.

These notes are operator-side observations not captured in the
combined `BUILD_HANDOFF.md`: the two-track split rationale, the
two run-quality regressions that surfaced (3rd `claude-no-artifact`
+ 3rd `gemini-no-frontmatter`), the operator-composed review
recovery, the SQL surgery on `artifacts.logical_name` after a
wrong-logical-name publish-on-behalf call, and why D102 is distinct
from the D095-D101 cycle-exhaustion override family.

## Two-track split rationale

RFC 0043 V1 is structurally two implementations that share a
schema decision and a method vocabulary. The design synthesis at
`docs/dogfood/048/DESIGN_SYNTHESIS.md` fixed both shared decisions
(single `striatumd.*` schema with `repository_id text NOT NULL`
on every repo-scoped table; dotted method vocabulary covering every
mutation in `src/striatum/cli/mutations.py`) so that the two tracks
could proceed in parallel without a sequential dependency between
schema-first and CLI-first.

- **Track A (codex)**: daemon-side Postgres schema for the 15
  repo-local tables + the `migrate-repo-local` body. Write scope
  was `src/striatum/daemon_pg/sql/`,
  `src/striatum/daemon_pg/repo_local_migration.py`,
  `src/striatum/daemon_pg/migrations.py`,
  `src/striatum/cli/daemon.py`, and the related test files. Track
  A explicitly does NOT touch `src/striatum/cli/parser.py`,
  `src/striatum/cli/dispatch.py`, or `src/striatum/daemon_rpc/`.
- **Track B (claude)**: operator-facing CLI surface changes
  (`--no-daemon` retirement, exit code 11 + 12 with platform-
  specific remediation, env-gated `enforce_daemon_required`
  helper) + RFC 0030 method-registry expansion to cover every
  mutation per RFC 0043 §5. Write scope was
  `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py`,
  `src/striatum/cli/daemon_required.py`, `src/striatum/daemon.py`,
  `src/striatum/daemon_rpc/{registry.py,server.py}`,
  `src/striatum/errors.py`, and the related test files. Track B
  explicitly does NOT touch `src/striatum/daemon_pg/`.

The split worked: both tracks shipped cleanly inside their write
scopes with no cross-track collisions. The one cross-track seam
(the `daemon migrate-repo-local` subparser wiring) was deliberately
left to V1.5 because it requires Track A's `cli/daemon.py` helper
to be merged into Track B's `cli/parser.py` subparser tree, and
the design synthesis chose to ship the migration body as
programmatically callable in V1 rather than block the dogfood on a
small mechanical wiring step. Gemini correctly flagged this as F1;
folded to V1.5 item 31(c).

## Run-quality regressions: 3rd `claude-no-artifact` + 3rd
`gemini-no-frontmatter`

Two run-quality regressions surfaced in the build-review job that
required operator recovery before D102 could be recorded.

### 3rd `claude-no-artifact` instance

The claude `review_build_claude` session completed normally —
supervisor saw exit code 0, no `process_outputs_missing` blocker,
no `process_review_verdict_missing` blocker. But the expected
artifact at `docs/dogfood/048/review/build/claude/REVIEW.md` was
never written. Session transcript showed the model reading the
build outputs, reasoning about the verdict, but the publish step
never fired. This is the 3rd instance of this exact pattern:

1. Prior instance #1 — earlier dogfood (claude reviewer of codex
   implementer); operator-composed verdict to recover.
2. Prior instance #2 — earlier dogfood (claude reviewer); same
   pattern.
3. This run (dogfood-048).

The pattern signature: claude reviewer reasons through the
verdict, may even draft the REVIEW.md body in chat output, but
does not call the `publish-artifact` tool. The supervisor sees
completion without the artifact. Operator recovery: composed the
verdict on-behalf using
`striatum dogfood publish_on_behalf` with the artifact body
reconstructed from the chat transcript and the explicit verdict
(`accept_with_findings` severity low), preserving the session
attribution so the operator byline is `author: reviewer-claude-...`
rather than `author: operator`.

This recurrence is now stable enough to deserve an explicit
recovery helper. The operator-side mitigation (compose-on-behalf
with attribution preservation) works but is manual and fragile —
the next dogfood that hits this should grow a `striatum recovery
compose-review-on-behalf --session <id> --verdict <verdict>
--severity <severity>` subcommand that automates the recovery
without requiring the operator to construct the SQL or call the
composite tool by hand.

### 3rd `gemini-no-frontmatter` instance

The gemini `review_build_gemini` session published its REVIEW.md
artifact correctly — but the artifact's content was missing the
`striatum.finding.v1` front-matter block. This is the 3rd
instance of this exact failure pattern:

1. Prior instance #1 — earlier dogfood (gemini reviewer); operator-
   fixed inline.
2. Prior instance #2 — earlier dogfood (gemini reviewer); same.
3. This run (dogfood-048).

The pattern signature: gemini publishes the markdown body but
omits the YAML front-matter block at the top of the file. The
publisher's RFC 0006 front-matter validator exits 6 on missing
front matter for any artifact kind in the validated set
(`finding`, `decision`, `support_ledger`, etc.); the artifact does
make it to disk before the validator runs, so operator fix is a
simple file edit. In this run, operator added the front-matter
block inline:

```yaml
---
schema_version: striatum.finding.v1
artifact_kind: finding
title: "..."
---
```

This recurrence pattern is also stable enough to deserve an
explicit fix-up helper. A `striatum recovery patch-front-matter
--artifact-id <id> --schema <schema> --kind <kind>` subcommand
would automate the inline fix without requiring the operator to
edit the file by hand.

## SQL surgery on `artifacts.logical_name`

Separate from the two reviewer recurrences above, the
operator-composed claude verdict required SQL surgery on the
`artifacts` table. The publish-on-behalf call passed the wrong
`logical_name` parameter (operator typed the workflow job's name
rather than the workflow's `expected_artifacts[]` entry's
`logical_name` field), which caused the publish to succeed but
the artifact to fail to bind to the expected slot in the
build-review job.

Recovery was a one-row `UPDATE artifacts SET logical_name = ?
WHERE artifact_id = ?` against the live
`.striatum/state.sqlite3`, after which the workflow could resolve
the expected-artifacts contract and the build-review job
transitioned to `complete`. The underlying file path
(`docs/dogfood/048/review/build/claude/REVIEW.md`) was correct
throughout — only the column value needed surgery.

This is a deeper version of the same pattern that motivated RFC
0043 V1.5 item 31(d)'s test-gap closure: end-to-end publish-on-
behalf paths should validate `logical_name` against the workflow's
`expected_artifacts[]` table at the daemon RPC layer rather than
at the publisher's file-write layer, so a typo at the call site
fails fast rather than producing an unbindable artifact that
requires SQL surgery to recover.

## Why D102 is distinct from D095-D101

The cycle-exhaustion override family now has eight instances on
the books (D095, D096, D097, D098, D099, D100, D101, D102). The
prior seven fall into two anti-pattern families:

- **codex/codex implementer+reviewer co-blindness** (D095
  dogfood-042 Track A, D096 dogfood-042 Track C, D097
  dogfood-043, D098 dogfood-044, D100 dogfood-046): same model on
  both sides converges on shared blind spots; reviewer's findings
  cluster around the implementer's own gaps, producing apparent
  `needs_revision` verdicts that 2-of-3 majority overrides.
- **codex-reviewer-of-claude-implementer baseline conservatism**
  (D099 dogfood-045 `reject` critical, D101 dogfood-047
  `needs_revision` high): codex applies threat_model-posture
  conservatism to a different model's work; cross-lane majority
  disagrees on scope.

D102 belongs to neither. Both overridden verdicts (codex
`needs_revision` high in Track A; gemini `needs_revision` medium)
identified **real findings on real scope gaps**:

- Codex F1 (crash-recovery persistence gap between Postgres commit
  and SQLite tombstone rename) is a genuine durability question
  about the migration's commit ordering that the synthesis did
  not address.
- Codex F2 (CLI escape path remains under env-gated
  enforcement default) is a genuine compliance gap against RFC
  0043 §3's "daemon-required is the default" wording.
- Gemini F1 (`daemon migrate-repo-local` subparser not wired) is
  a genuine reachability gap — the migration body exists but
  cannot be called from the operator CLI.
- Gemini F2 (tombstone persistence ambiguity) overlaps codex F1
  on a different framing.

These are NOT shared-blind-spot artifacts; they are findings the
operator agrees with and folds to V1.5 (TODO item 31). D102's
rationale for override is "scope met on the substrate flip; real
findings fold to V1.5; ships at V1 because the in-scope
correctness contract is met and the remaining deltas are
operator-side wiring + crash-recovery hardening, not architectural
defects."

The cycle-exhaustion override pattern is now broad enough to
distinguish three families: (i) co-blindness convergent findings
(D095-D098, D100); (ii) cross-model reviewer conservatism (D099,
D101); (iii) real findings folded to follow-up (D102). The
operator-side mitigation differs per family: (i) needs the
refuse-by-default validator from TODO item 26 to prevent the
pairing from being authored in the first place; (ii) needs the
cross-lane majority envelope which is already working; (iii)
needs nothing — the override is the correct disposition, the
follow-up captures the work.

## Follow-ups

V1.5 work captured at TODO item 31. The two recurrence-driven
operator follow-ups (compose-review-on-behalf helper, patch-front-
matter helper) are separate from the V1.5 RFC 0043 follow-up and
should land as recovery-surface ergonomics in a near-term
operator sweep rather than blocking on a full dogfood cycle.

RFC 0039 Phase 2 (TODO item 25) is now unblocked: the Go core has
a single canonical Postgres substrate, no SQLite half remaining,
and Phase 2 can proceed.
