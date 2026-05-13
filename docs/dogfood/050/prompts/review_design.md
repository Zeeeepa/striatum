# Design Review Prompt (RFC 0043 V1.5)

Produce REVIEW.md at `docs/dogfood/050/review/design/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`: `ergonomics_dx`.

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version` must be the exact string `"striatum.finding.v1"`, not `"1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0043", "v1.5", "design"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the synthesis at `docs/dogfood/050/DESIGN_SYNTHESIS.md`. Apply the `ergonomics_dx` lens:

- Does the crash-recovery path produce a useful operator message on resume? ("Resuming partially-completed migration: Postgres state present, SQLite source still writable, completing tombstone…")
- Does `striatum daemon migrate-repo-local --help` show up in `striatum daemon --help`? Is the help text actionable (mentions exit code 12 remediation, the `--confirm-delete` and `--keep-sqlite-readonly` flags)?
- Does the default-flip break the transition story? If a user upgrades striatum without first running `migrate-repo-local`, what do they see? Exit code 12 with a clear "run `striatum daemon migrate-repo-local`" hint, or a confusing daemon-unreachable error?
- Is the exit-12 e2e test discoverable from `make test`? Does it need a `STRIATUM_PG_TEST_URL`, or can it use a stub?

Specific checks:

- **F-crash**: chosen shape (transactional rollback xor checkpointed resume) is NAMED. Function signature change locked. Sentinel/lock primitive named. Regression test path locked.
- **F-escape**: default-flip locked (env var opt-OUT or removed). Audit list of other silent-SQLite-fallback paths in cli/ tree is exhaustive (or explicitly "none beyond X").
- **F-parser**: exact subparser block (argument names, help text). Exact dispatch arm. Smoke command works.
- **F-test**: exact test path named (one file). Fixture shape locked. Assertion shape locked.
- Implementation order locked (F-parser before F-test? F-escape before F-crash? — pick one with rationale).
- Backward-compat checks: any new SQL file is additive; `--keep-sqlite-readonly` tombstone still works under the new crash-recovery path.
- Operator UX: error envelope examples drafted for (a) crash-recovery resume, (b) unmigrated repo refusal (exit 12), (c) daemon-required env-var-opt-out (if retained).

Cite the synthesis section(s) you are challenging. Hand-waving findings ("the design is unclear") without a pinpoint citation will be down-weighted by the verdict gate.

**IMPORTANT — write the REVIEW.md directly in this single supervised invocation.** If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
