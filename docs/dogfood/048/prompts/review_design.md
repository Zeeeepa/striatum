# Design Review Prompt (RFC 0043 V1)

Produce REVIEW.md at `docs/dogfood/048/review/design/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`: `ergonomics_dx`.

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version` must be the exact string `"striatum.finding.v1"`, not `"1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0043", "v1", "design"]
---

author: reviewer-unknown-model-<NN>
```

`schema_version` is the exact string `"striatum.finding.v1"`. `artifact_kind` is `"finding"`. `verdict_intent` is one of `accept | accept_with_findings | needs_revision | reject`. `severity` is one of `low | medium | high | critical`. `tags` is a JSON array. The byline is a plain markdown line AFTER the front-matter block — no lane prefix, no markdown bold.

Read the synthesis at `docs/dogfood/048/DESIGN_SYNTHESIS.md`. Apply the ergonomics_dx lens.

Specific checks:

- **Schema (Track A)**: All 15 tables enumerated with `repository_id UUID NOT NULL` + `(repository_id, ...)` index strategy. Schema-namespacing decision (single shared schema vs per-repo) is made and justified. Append-only grants on `events` + `artifacts` preserved per RFC 0033 §3.
- **migrate-repo-local (Track A)**: SERIALIZABLE single-tx semantics named. Audit-chain byte-equivalence algorithm named. `--keep-sqlite-readonly` default + `--confirm-delete` required for destructive. Idempotent re-run via checkpoint row. Test fixture path locked.
- **CLI surface (Track B)**: `--no-daemon` retirement exactly cited (line in `parser.py`). Exit code 11 stderr template names socket path + platform remediation (Linux systemd, macOS launchctl, foreground hint, Postgres install hints). Exit code 12 names `migrate-repo-local` in remediation.
- **RPC registry (Track B)**: Every mutation in `src/striatum/cli/mutations.py` has a registered method per RFC 0043 §5 table. Capability mapping correct. Read-capability methods named.
- **Backward-compat**: existing daemon_mode=on test fixtures still pass; integration tests use daemon-mediated path only.
- **D094 framing cited** with RFC 0043 reference. Exit codes 11/12 documented.

Cite the synthesis section(s) you are challenging. Hand-waving findings ("the design is unclear") without a pinpoint citation will be down-weighted by the verdict gate.

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

**IMPORTANT — write REVIEW.md directly in this single supervised invocation.** If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
