# GH #28 - review_posture: compliance_license evidence scope

Source: https://github.com/halbritt/striatum/issues/28

## Summary

Issue-workflow verify jobs use `review_posture: compliance_license`. One
fresh verifier interpreted that posture as a closed evidence policy and
refused to inspect the implementer handoff, changed files, tests, and command
outputs that the verify prompt explicitly required.

## Acceptance

1. Document that `compliance_license` scopes findings, not evidence.
2. Ensure issue verify prompts or policy docs tell reviewers to inspect the
   implementer handoff and changed files named by it.
3. Add a regression test or pattern check that the expected evidence set stays
   visible to fresh reviewers.
