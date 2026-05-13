# Design Review Prompt (RFC 0039 V1.5)

Produce REVIEW.md at `docs/dogfood/047/review/design/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`: `ergonomics_dx`.

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version` must be the exact string `"striatum.finding.v1"`, not `"1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0039", "v1.5", "design"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the synthesis at `docs/dogfood/047/DESIGN_SYNTHESIS.md`. Apply the `ergonomics_dx` lens: is `make test-multi-repo CORE=go` discoverable from `make help` or `make`? Does `daemon_core=go` produce useful error output if the Go binary is missing? Does the Postgres-backed authorizer return a JSON error envelope shape consistent with the Python daemon path?

Specific checks:

- **F1**: authorizer file path, validator interface, denial envelope shape, audit-on-deny hook all NAMED. Not "the new authorizer module".
- **F2**: locked argv / env contract between `tests/_harness/daemon.py` and the Go binary. Exact `go/Makefile` target. Exact binary path. Smoke command the operator can run today.
- **F3**: exact `make` target shape, the chosen pytest parametrize file under `tests/_harness/`, the test-selection rule. Not "the multi-repo tests".
- **F4**: exact function signature change in `go/pkg/db/audit.go`, chosen isolation level (one of, not "an appropriate level"), exact regression-test path.
- **F5**: ONE driver chosen (lib/pq xor pgx) with one-sentence justification. Exact `go.mod` line. Explicit first-third-party-dep callout.
- Implementation order locked (F5 before F4? F1 before F2?) with cross-finding dependency rationale.
- Operator UX: error envelope examples drafted for (a) missing Go binary, (b) Postgres token rejected, (c) transaction conflict on audit append.

Cite the synthesis section(s) you are challenging. Hand-waving findings ("the design is unclear") without a pinpoint citation will be down-weighted by the verdict gate.

**IMPORTANT — write the REVIEW.md directly in this single supervised invocation.** If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
