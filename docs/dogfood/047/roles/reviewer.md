# Reviewer Role (Dogfood 047)

One design review (gating implement) plus a 3-way build review at the
end.

## Design review (claude, `ergonomics_dx`)

Does the synthesized design name an exact authorizer file path (F1),
an exact harness argv / env contract and Makefile target (F2), an
exact `make test-multi-repo CORE=go` target shape and pytest
parametrize file (F3), an exact `go/pkg/db/audit.go` function signature
+ chosen isolation level (F4), and ONE driver chosen with a one-sentence
justification (F5)? Is the implementation order locked? Are operator
error envelopes drafted for the new failure modes (missing Go binary,
rejected token, transaction conflict)?

## Build review (3-way, `parallel_group: build_review`)

- **codex** `threat_model` — auth correctness (no fail-open on
  Postgres error; `AllowAllAuthorizer` gone from production wiring);
  audit-chain integrity (transaction wraps read+append; race test
  fails on the un-fixed code); denial audit hook actually emits.
- **claude** `ergonomics_dx` — `daemon_core=go` launches cleanly via
  `tests/_harness/daemon.py`; `make test-multi-repo CORE=go` is
  discoverable and passes; error messages from the new authorizer and
  from transaction conflicts are operator-readable.
- **gemini** `adversarial threat_model` — race conditions (concurrent
  appenders, deadlock recovery, hash-link tampering); supply chain
  (chosen driver's module hash pinned, no transitive surprise deps,
  `go.sum` consistent).

## Required finding front matter (all 5 fields)

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0039", "v1.5", "dogfood-047"]
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
