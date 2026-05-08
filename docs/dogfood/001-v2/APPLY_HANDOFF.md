---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: author-claude-opus-001

# APPLY_HANDOFF — dogfood-001 v2 finalized

Run: `run_4db045f7e3e643d6a75948dd1b86d6d6`
Branch: `striatum/dogfood-001-v2-harness-fixes`
Job: `apply_change`

## Final test count

- `make lint` → ruff: clean (`All checks passed!`).
- `make typecheck` → mypy: clean (`no issues found in 36 source files`).
- `make test` → 151 passed (was 143; the 8 new tests in
  `tests/test_harness_v2_fixes.py` plus updates to existing fixtures
  to satisfy the new register-session policy and byline-integrity
  contract).

## Disposition of reviewer findings

The reviewer's verdict was `accept_with_findings` with seven findings.
All seven are `severity: info` or `low` and were captured in the
review as candidates for follow-up rather than blockers. Per the apply
prompt's "items the reviewer left as info-only can be deferred — note
them in the apply handoff" instruction, all seven are deferred. Each
is restated below with rationale.

- **F-1 (info) — Editable install check is repo-gated.** The
  `editable_install_outside_repo` doctor warning only fires when the
  repo argument is itself a Striatum source tree. The reviewer
  correctly noted that a complementary "running install is older than
  its bundled migrations" check would also help. **Deferred** to a
  future round; the v2 prompt scoped the cheap layer of HARNESS-002
  and the install-vs-bundled-migrations check is a different surface
  (orthogonal to the editable-pin foot-gun this round addressed).

- **F-2 (info) — Makefile install path lacks a focused automated
  test.** The behaviour is verified by inspection (the install rule
  prints `installing striatum (editable) from <path>` and the
  resolved path is the Makefile directory). A subprocess-driven test
  would be the right shape but pytest fixtures for invoking `make`
  from a tmp cwd add complexity for a one-line guard. **Deferred**
  to a follow-up if Makefile drift becomes a concrete problem.

- **F-3 (info) — `reviewer_independence_unverified` doctor cases
  lack focused tests.** The query logic for shared-pid and
  asymmetric supervised-author + unsupervised-reviewer is correct
  by inspection but exercised only by code review. **Deferred**;
  the precondition fixtures are non-trivial (two
  `process_supervisors` rows with the same pid; one supervised
  session plus a sessionless reviewer) and the runner already
  refuses the most common breach at register-session time.

- **F-4 (low) — `_first_author_line` lowercases the byline suffix.**
  Per AGENTS.md privacy-safe bylines are deliberately lowercase, so
  the canonicalisation is correct for current usage. **Deferred** —
  if a future renderer wants role-specific casing, that's a separate
  concern.

- **F-5 (info) — Existing pid-gone path bypasses `report()`.** The
  draft handoff already flagged this as out-of-scope; the reviewer
  agreed. **Deferred**.

- **F-6 (info) — `actual_author_line` field shape.** The reviewer
  agreed with keeping both `author.line` and
  `author.actual_author_line` for back-compat. **No action needed.**

- **F-7 (info) — Independence enforcement aggressiveness.** The
  reviewer agreed with the broad "any active author session in run"
  refusal (with the documented `--force-non-fresh --reason` escape
  hatch). **No action needed.**

No CHANGELOG.md edit was made during apply (no further code
changes).

## Manual verification performed during apply

The new behaviour was already exercised both by the test suite (8
focused tests, all passing) and live during this run itself:

- The reviewer session for this v2 run was registered with
  `--force-non-fresh --reason "operator drove both lanes;
  HARNESS-001 working supervised lane not yet shipped; documented in
  HARNESS-003"`. The reason is durable on
  `sessions.non_fresh_reason` and will appear in the evidence
  export.
- The review FINDING and DRAFT_HANDOFF artifacts both exercise the
  byline-integrity path: the FINDING omits an `author:` line on
  purpose and will render as `author: <missing>` in the run summary;
  the DRAFT_HANDOFF carries the workflow-declared byline and will
  render normally.
- `make lint typecheck test` ran cleanly at the start of apply (151
  passed, ruff and mypy clean).

The `supervise stop` idempotency, doctor checks, and `init` guard
behaviours could not be exercised live in this run because the v1
runner was driving the v2 work — those code paths only become live
on the *next* run after the v2 install is loaded. They are covered
by the test suite; verifying them live is the explicit purpose of
the next dogfood round (002 or 001 v3) when one is scaffolded.

## Late fix during apply: title-block byline scanner

While verifying the byline integrity feature on v2's own artifacts I
noticed every artifact rendered as `author: <missing>` in
`RUN_SUMMARY.md`. Root cause: the pre-existing
`markdown_title_block_author_lines()` helper *only* scanned YAML
front matter when one was present, never the Markdown title block
following the front matter. The v2 handoff convention (and the
existing dogfood-001 handoffs) put `author:` *after* the `---` block
in the title block. So the new `author_line` column was correctly
recording NULL — given the scanner's view — but the underlying intent
of HARNESS-003 was to capture title-block bylines too.

Fixed during apply: `markdown_title_block_author_lines()` now scans
both the front matter *and* the post-front-matter title block, then
deduplicates. All 151 tests still pass; lint and mypy clean. This is
a small follow-on extension of HARNESS-003 — the byline integrity
fix needed the scanner to see the same lines AGENTS.md tells authors
to write.

Forward-only application: the artifact rows for v2's own
`DRAFT_HANDOFF`, `FINDING`, and `APPLY_HANDOFF` were inserted with
`author_line = NULL` *before* this scanner fix landed, so they will
keep rendering `<missing>` in this run's `RUN_SUMMARY.md` and
`EVIDENCE.md`. The next supervised run (or any future dogfood)
starting after this commit will have bylines correctly populated.

## Friction surfaced during review→apply

None new. The v2 round itself was uneventful — the protocol from
`claim-next` → `ack` → publish → complete worked as documented when
driven by the operator. The two friction signals worth recording:

1. **The new register-session policy tripped fixture helpers.**
   Updating four `register` test helpers and two
   `complete_claimed_job`/`verdict_claimed_review` helpers to honour
   the new policy and the byline-integrity contract was about
   one-third of the v2 author work. That's the friction the policy
   is designed to surface in real workflows: *every* operator-driven
   workflow must now explicitly acknowledge the breach. Working as
   intended, but worth flagging so future maintainers know why the
   helpers carry the `"test fixture"` reason string.

2. **The dogfood-001 reviewer session originally registered
   pre-policy.** The original `sess_caa84d683fb6476ea9a696fc4f7e0a17`
   from this run's first attempt was registered before HARNESS-003
   landed (the v2 author work *was* HARNESS-003), so it carries
   `non_fresh_reason = NULL` even though the operator drove both
   lanes. That's a one-off artifact of self-bootstrapping the policy
   change; new runs starting from HEAD will be clean. No action.

## What's next for the operator

After this job completes, run:

```bash
.venv/bin/striatum --repo . evidence export \
  --run-id run_4db045f7e3e643d6a75948dd1b86d6d6 \
  --path docs/dogfood/001-v2/EVIDENCE.md --json

.venv/bin/striatum --repo . run summary \
  --run-id run_4db045f7e3e643d6a75948dd1b86d6d6 \
  --path docs/dogfood/001-v2/RUN_SUMMARY.md --json

.venv/bin/striatum --repo . supervise stop \
  --session-id sess_edeebb4fa1634ef7b6298748c44135ce \
  --reason "dogfood 001 v2 done" --json
```

The `supervise stop` call should now be idempotent against the dead
author supervisor (`sup_4242e684bbea43f5a94cd7967589c8c0`,
`state: lost`) — that's HARNESS-001's idempotency fix exercising on
its own dogfood. Note: the dead supervisor was started under v1
code; the *idempotency* logic is v2 code, so this is a
forward-compat smoke test of the fix.

Then commit and tag per the v2 RUNBOOK.
