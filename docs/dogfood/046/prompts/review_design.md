# Design Review Prompt (RFC 0044 V1)

Produce REVIEW.md at `docs/dogfood/046/review/design/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`: `ergonomics_dx`.

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version` must be the exact string `"striatum.finding.v1"`, not `"1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0044", "v1", "design"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the synthesis at `docs/dogfood/046/DESIGN_SYNTHESIS.md`. Apply the ergonomics_dx lens: is `striatum corpus export --since <ref> --out <path>` operator-discoverable from `--help`? Is the JSON error envelope behavior consistent with other CLI verbs (`run summary`, `recovery`)? Does the redaction policy fail loudly or silently when it encounters a new field?

Specific checks:

- CLI verb wiring names the exact `parser.py` and `dispatch.py` functions, not "the CLI dispatcher".
- `src/striatum/corpus/` module layout is one chosen shape, not three alternatives. Each file has a named single responsibility.
- Enumeration sources are named functions or named CLI verbs (run summaries route through `striatum run summary --json` per RFC 0044 §3).
- Redaction policy is a concrete denylist with per-field rules. `.env`, transcripts, `.striatum/state.sqlite3` blobs, terminal output explicitly excluded.
- JSONL emission shape locks to RFC 0044 §3 line shape and `external_id` table. Ordering rule named so re-export hashes are stable.
- Manifest fields lock to RFC 0044 §3 list.
- Augmentation-not-dependency: no Engram import in `src/striatum/`. Regression check named.
- Tests have exact file paths and the integration test is against a real run, not a synthetic fixture.
- Operator UX: `--help` text drafted; error envelope examples for `--since` parse failure and `--out` permission failure.

Cite the synthesis section(s) you are challenging. Hand-waving findings ("the design is unclear") without a pinpoint citation will be down-weighted by the verdict gate.

**IMPORTANT — write the REVIEW.md directly in this single supervised invocation.** If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
