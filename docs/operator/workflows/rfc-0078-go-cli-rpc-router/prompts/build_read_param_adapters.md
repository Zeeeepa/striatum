# Build Read Parameter Adapters

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Implement parameter adapters for generated read/list/reporting routes.

Scope examples include status, why, doctor, dashboard, list routes, run summary,
run graph, evidence/corpus/archive exports, repo list, git snapshot, inbox, and
cross-repo read routes. Use the route-contract artifact as the final grouping
authority.

Requirements:

- preserve current flag names and JSON/text output behavior where the prior
  Python CLI already defined operator-facing behavior;
- build typed params without duplicating daemon business logic;
- name unsupported or local-only command behavior explicitly;
- add focused tests for parsing into params and method selection;
- record command examples and validation results in the handoff.
