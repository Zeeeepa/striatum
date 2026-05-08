# Review Tool Harness Profile Implementation

Review the implementation from the accepted RFC 0010 build slice. Use fresh
context and inspect the changed code, tests, docs, design synthesis, and build
handoff.

Focus on:

- schema validation correctness, especially malformed profiles and missing
  `harness_profile_id` references;
- work-packet shape and backwards compatibility;
- whether provider-specific behavior stays in profiles, not core scheduling;
- whether default workflows without `harness_profiles` produce identical
  packets and behavior;
- whether native subagent guidance remains advisory and internal to the
  parent Striatum session;
- tests for malformed references and no-profile workflows;
- docs accuracy and generic language.

Write `docs/dogfood/003/review/BUILD_REVIEW.md` as a `finding` artifact and
submit a structured verdict. Use `needs_revision` for code or docs issues
that must be fixed before the run can finish.
