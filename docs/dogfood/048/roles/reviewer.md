# Reviewer Role (Dogfood 048)

One design review (gating implement) plus a 3-way build review at the
end. Reviewers run with `fresh_session_required: true` and
`reviewer_context_policy: fresh`.

## Design review (claude, `ergonomics_dx`)

Does the synthesized design name exact `daemon_pg/sql/` migration
filename, the 15 tables with `repository_id UUID NOT NULL` and
`(repository_id, ...)` indexes, the schema-namespacing decision (single
vs per-repo), append-only grants on `events`/`artifacts`, an exact
`migrate-repo-local` flag set + SERIALIZABLE single-tx semantics,
audit-chain byte-equivalence algorithm, idempotent re-run via checkpoint
row, exact `parser.py` line where `--no-daemon` is removed, exact exit-
code 11 + 12 stderr templates with platform remediation, exact RPC
method-registry expansion list covering every mutation in
`src/striatum/cli/mutations.py`, and exact test paths including the V1
SQLite fixture? Is D094 framing cited (supersedes D006/D007/D036 and the
SQLite half of D009)?

## Build review (3-way, `parallel_group: build_review`)

- **codex** `threat_model` — schema invariants (append-only grants,
  `repository_id` NOT NULL, indexes); audit-chain byte-equivalent re-
  anchor verifies; method registry exhaustive vs `mutations.py`;
  `--no-daemon` truly removed (no silent SQLite fallback path).
- **claude** `ergonomics_dx` — `migrate-repo-local --dry-run` legible;
  tombstone semantics obvious; daemon-unreachable (exit 11) message
  names socket + remediation; unmigrated (exit 12) message names
  `migrate-repo-local`; `daemon doctor` works without daemon; SQLite
  stays readable post-migration; flag defaults safe.
- **gemini** `adversarial threat_model` — concurrent migrate idempotency;
  `--confirm-delete` + `--keep-sqlite-readonly` conflict; partial-migrate
  crash recovery; older-client silent migrate; method-registry holes;
  `--no-daemon` escape paths (env var, alias, config file).

## Required finding front matter (all 5 fields)

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0043", "v1", "dogfood-048"]
---

author: reviewer-unknown-model-<NN>
```

`schema_version` must be the exact string `"striatum.finding.v1"`
(not `"1"`). `artifact_kind` is `"finding"`. `verdict_intent` is one of
`accept | accept_with_findings | needs_revision | reject` (not
`verdict`). `severity` is one of `low | medium | high | critical`.
`tags` is a JSON array. The `author:` byline is a plain markdown line
AFTER the front-matter block — not inside it. No lane prefix. No
markdown bold.

**IMPORTANT — write the REVIEW.md / finding artifact directly.** If
`striatum ack` is denied, write the artifact and exit normally; the
operator publishes on your behalf. Do not ask the operator clarifying
questions and exit. Per dogfood-037 intervention #5 + dogfood-041
friction patterns + dogfood-046 reviewer-emits-no-artifact +
gemini-no-frontmatter anti-patterns.
