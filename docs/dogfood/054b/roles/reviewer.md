# Reviewer Role (Dogfood 050)

One design review (gating implement) plus a 3-way build review at the
end.

## Design review (claude, `ergonomics_dx`)

Does the synthesized design name an exact crash-recovery shape
(transactional rollback xor checkpointed resume) with a function
signature change in `src/striatum/daemon_pg/repo_local_migration.py`
(F-crash)? Does it lock the default-flip in
`src/striatum/cli/daemon_required.py` and enumerate any other silent
SQLite fallback paths in the cli/ tree (F-escape)? Does it spec the
exact subparser block in `src/striatum/cli/parser.py` and the dispatch
arm in `src/striatum/cli/dispatch.py` (F-parser)? Does it lock the e2e
exit-12 test path and fixture shape (F-test)? Is the implementation
order locked? Are operator error envelopes drafted for crash-recovery
resume, unmigrated-repo refusal (exit 12), and (if retained) the
env-var opt-out? Is the backward-compat lock explicit
(additive-only schema; `--keep-sqlite-readonly` tombstone preserved)?

## Build review (3-way, `parallel_group: build_review`)

- **codex** `threat_model` — crash-recovery actually closes the
  split-brain window (kill -9 between Postgres commit and SQLite
  tombstone leaves no usable bypass on resume); CLI escape path closed
  by default (no env var, no flag, no positional argument silently
  routes to SQLite); audit-chain integrity preserved through migration
  retry.
- **claude** `ergonomics_dx` — `striatum daemon migrate-repo-local
  --help` shows; exit-code-12 stderr remediation block is actionable;
  `--keep-sqlite-readonly` still tombstones correctly; default-flip
  transition story is documented (what does an upgrading operator see
  on a pre-V1.5 repo?).
- **gemini** `adversarial threat_model` — any remaining silent SQLite
  fallback (`rg -n "sqlite3" src/striatum/cli/`); concurrent
  migrate-repo-local invocations; rollback-on-crash atomicity;
  backward-compat tombstone semantics preserved under every flag
  combination.

## Required finding front matter (all 5 fields)

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0043", "v1.5", "dogfood-050"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

`schema_version` is the exact string `"striatum.finding.v1"` (not
`"1"`). `verdict_intent` is one of `accept | accept_with_findings |
needs_revision | reject`. `severity` is one of `low | medium | high |
critical`. `tags` is a JSON array. The `author:` byline is a plain
markdown line AFTER the front-matter block — no bold, no lane prefix
(e.g. `author: reviewer-codex-unknown-model-01`).

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum
ack` is denied, write the artifact and exit normally; the operator
publishes on your behalf. Do not ask the operator clarifying questions
and exit.
