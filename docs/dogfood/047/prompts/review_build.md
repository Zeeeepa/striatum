# Build Review Prompt (RFC 0039 V1.5, 3-way)

Produce REVIEW.md at `docs/dogfood/047/review/build/<lane>/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`:

- **codex**: `threat_model`
- **claude**: `ergonomics_dx`
- **gemini**: `threat_model` (adversarial angle)

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version` must be the exact string `"striatum.finding.v1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0039", "v1.5", "build"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

The `author:` byline is a plain markdown line AFTER the front-matter block — not inside it, no markdown bold, no lane prefix. Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the implementation handoff at `docs/dogfood/047/build/HANDOFF.md`.

Per-lane angle:

- **codex (threat_model)**: F1 authorization correctness — is `AllowAllAuthorizer` actually gone from the launch path? Token validator rejects unknown tokens, denial audited, no fail-open on Postgres error. F4 audit-chain integrity — transaction actually wraps the read+append, isolation level honored, race test fails on the un-fixed code. Manifest / hash invariants intact.
- **claude (ergonomics_dx)**: F2 `daemon_core=go` launches cleanly via `tests/_harness/daemon.py` and produces a useful error if the binary is missing. F3 `make test-multi-repo CORE=go` is discoverable and passes; CI parity with the Python core is clear. Error messages from the new authorizer and from transaction conflicts are operator-readable.
- **gemini (adversarial threat_model)**: F4 race conditions — concurrent appenders, transaction deadlock recovery, hash-link tampering between read and commit. F5 supply chain — the chosen Go driver's module hash pinned, no transitive surprise deps, `go.sum` consistent.

Required checks (all lanes):

- **F1 authorizer wired**: `rg -n "AllowAllAuthorizer" go/cmd/striatumd/` returns no production-launch hits.
- **F2 launch works**: a real `pytest tests/_harness/...` (or equivalent) launches the Go daemon end-to-end.
- **F3 matrix wired**: `make test-multi-repo CORE=go` is a real target and passes locally.
- **F4 race test in-tree**: a regression test that fails on the un-fixed audit append exists and passes on the fixed version.
- **F5 driver swapped**: no `exec.Command("psql", ...)` under `go/pkg/db/`; `go.mod` lists the chosen driver; supply-chain note in HANDOFF.
- **Tests pass**: `make test` green; `go test ./...` green under `go/`.

Cite specific files / lines / test names. "Looks good" is not a review.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write and exit normally.
