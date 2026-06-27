---
schema_version: striatum.decision.v1
decision_id: "D272"
run_id: "run_010c81ec8ca17ffd182e0bd7be3f28cc"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "RFC 0168 P0 design accepted via operator override after v6 convergence"
created_at: "2026-06-27T04:18:39Z"
---

# RFC 0168 P0 design accepted via operator override after v6 convergence

Decision ID: `D272`
Run ID: `run_010c81ec8ca17ffd182e0bd7be3f28cc`
Outcome: `accepted_with_follow_up`

## Rationale

Security half of OQ4 discharged across v3-v6: C1 scrub classifier (classifyPoolUIDTaskState), C2 bearer-path (writeEphemeralMCPConfig under .striatum/scratch/<supervisor_id>/), credential-cache ancestry (modeled selectors), and fail-closed completeness against unmodeled selectors incl. CLAUDE_SECURESTORAGE_CONFIG_DIR (falsifier confirmed no bypass, incl ANTHROPIC_CONFIG_DIR). The lone open v6 finding is over-broad-refusal of legitimate non-credential lane env (AGY_HOME/FIXTURE_CONFIG_DIR) -- a usability/over-refusal refinement, not a security gap. Operator accepts the converged, secure design and folds the discriminator as a binding build constraint rather than grinding a v7.

## Follow-Up

BUILD CONSTRAINT (binding): give the OQ4.1.2 coverage-gap gate a discriminator so it fails closed ONLY for provider-owned credential selectors (scope to keys declared credential-bearing by the in-scope provider adapter, or a resolver-owned credential-selector/provider-prefix registry), NOT every *_HOME/*_CONFIG_DIR/*_CACHE_DIR-shaped key. Keep the typed lane_uncovered_credential_selector_inside_repo refusal for an uncovered PROVIDER credential-dir selector resolving in-repo; add a positive-control test that a legitimate in-repo non-credential lane env (AGY_HOME/FIXTURE_CONFIG_DIR) STILL launches.
