# Review prompt — HARNESS-001/002/003/004 fixes

## Task

Independently review the dogfood-001 v2 draft change. Your access scope
is `artifact_augmented`: read the draft handoff, the modified source
files, the new tests, and the four source HARNESS proposals. Do not
browse the rest of the repository.

## Context to read

- `docs/dogfood/001-v2/DRAFT_HANDOFF.md` — author's per-HARNESS
  disposition.
- `docs/dogfood/001/SYNTHESIS.md` — the synthesis that motivated v2.
- `docs/dogfood/001/findings/HARNESS-001.md`,
  `docs/dogfood/001/findings/HARNESS-002.md`,
  `docs/dogfood/001/findings/HARNESS-003.md`,
  `docs/dogfood/001/review/HARNESS-004.md` — source proposals; check
  the "Proposed change" sub-points against what landed.
- The modified source files in `src/striatum/` and `tests/` per the
  draft handoff.

## What to check

For each HARNESS proposal, walk the "Proposed change" sub-points and
mark each as **landed**, **partially landed**, or **deferred** in your
finding. The draft handoff should already make this disposition
explicit; your job is to verify it against the code, not to take it on
faith.

Specific gates:

1. **HARNESS-001 contract doc.** Does `docs/SPEC.md` have a
   "Supervised lane command contract" subsection that names the three
   requirements (alive across packets, NDJSON stdin, calls back via
   CLI)?
2. **HARNESS-001 doctor check.** Set up the `process_supervisors`
   `lost` + active-lease state in a tmp DB and confirm the new
   doctor problem record fires.
3. **HARNESS-001 status next_action.** Same setup; confirm
   `striatum status --run-id <id> --json` surfaces a `next_action`
   pointing at the recovery path.
4. **HARNESS-001 supervise stop idempotency.** Confirm a `supervise
   stop` against an already-lost supervisor exits 0 with the new
   note, not exit 4.
5. **HARNESS-002 doctor check.** Confirm the new
   `editable_install_outside_repo` check fires when
   `striatum.__file__` is outside the resolved repo argument.
6. **HARNESS-002 init guard.** Confirm `init` against a fresh DB
   refuses with exit 3 when the running install's `LATEST_VERSION`
   is lower than the repo's source-tree `LATEST_VERSION`.
7. **HARNESS-002 Makefile.** Confirm `make install` resolves the
   path explicitly (not cwd-dependent) and prints the resolved path.
8. **HARNESS-003 spec text.** Does `docs/SPEC.md` have a
   "Reviewer Independence" subsection making the advisory nature
   explicit and listing the operator obligations?
9. **HARNESS-003 doctor check.** Confirm
   `reviewer_independence_unverified` fires for shared-pid sessions
   and for unsupervised reviewer + supervised author.
10. **HARNESS-003 register-session policy.** Confirm refusal without
    `--force-non-fresh`, success with `--force-non-fresh --reason`,
    and that the reason is recorded on the session row.
11. **HARNESS-003 byline integrity.** Confirm the snapshot records
    `null` or `"missing"` when the file's author line is absent,
    not the workflow's declared expected byline.
12. **HARNESS-004 doc fix.** Confirm `docs/dogfood/001/roles/reviewer.md`
    points at a path within the review job's `write_scope`. Audit any
    other dogfood reviewer role docs.

## Verdict choices

Pick one and submit via `striatum submit-review`:

- `accept` — every gate passes, all advertised sub-points landed,
  tests cover each fix.
- `accept_with_findings` — gates pass and the change is mergeable, but
  some sub-points are partial or deferred and you want to record that
  for follow-up.
- `needs_revision` — at least one gate fails (e.g., a doctor check
  doesn't fire under its declared precondition, or `init` does not
  refuse, or the reviewer doc still points at an out-of-scope path).
  The workflow declares a one-shot revision cycle.
- `reject` — the bundle is structurally wrong (e.g., regressed test
  count, broke a previously-passing check, or the SPEC text is
  contradictory). Use sparingly.

## Finding artifact

Write your review at `docs/dogfood/001-v2/review/FINDING.md` with valid
`striatum.finding.v1` front matter:

```yaml
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["dogfood-001-v2", "harness-fixes"]
---
```

Then submit:

```bash
striatum submit-review \
  --session-id "$REVIEWER" \
  --job-id "$REVIEW_JOB_ID" \
  --lease-id "$REVIEW_LEASE_ID" \
  --kind finding \
  --logical-name review_finding \
  --path docs/dogfood/001-v2/review/FINDING.md \
  --verdict <verdict> \
  --json
```

If you hit runner friction during review, file a
`harness_improvement_proposal` under
`docs/dogfood/001-v2/review/HARNESS-NNN.md` (inside your write_scope —
HARNESS-004 is the reason this prompt is explicit).
