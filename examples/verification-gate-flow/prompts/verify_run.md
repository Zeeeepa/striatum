# Task: run the witnesses, report ground truth

Read `CLAIM_LEDGER.md`. For each `VERIFIED` and `ASSERTED` row, **run its
witness yourself** with your tools — do not trust the builder's claimed result.

Publish `VERIFICATION_REPORT.md` as a table:

| claim | command run | exit code | PASS/FAIL | reason |
|---|---|---|---|---|

Discipline:
- Execute the command; record the real exit code and the output that decides it.
- A witness that runs green but does not actually exercise the claim (asserts
  nothing, greps a comment, CLI output lacks the claimed behavior) is `FAIL`.
- Missing, erroring, or irrelevant witness → `FAIL`. Default to `FAIL` under
  doubt.
- You report; you do not adjudicate.

Follow your role definition (`roles/verifier.md`).
