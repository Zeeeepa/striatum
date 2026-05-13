# Design Review Prompt (RFC 0039 Phase 2)

Produce REVIEW.md at `docs/dogfood/049/review/design/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`: `ergonomics_dx`.

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version` must be the exact string `"striatum.finding.v1"`, not `"1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0039", "phase-2", "design"]
---

author: reviewer-unknown-model-<NN>
```

`schema_version` is the exact string `"striatum.finding.v1"`. `artifact_kind` is `"finding"`. `verdict_intent` is one of `accept | accept_with_findings | needs_revision | reject`. `severity` is one of `low | medium | high | critical`. `tags` is a JSON array. The byline is a plain markdown line AFTER the front-matter block — no lane prefix, no markdown bold.

Read the synthesis at `docs/dogfood/049/DESIGN_SYNTHESIS.md`. Apply the ergonomics_dx lens.

Specific checks:

- **Track-boundary separation**: Track A (CLI integration + mutating verbs) and Track B (supervisor + distribution + CI) have non-overlapping Go file paths and non-overlapping Python/CI/Makefile file paths. The workflow.json write-scope sets enforce this; the synthesis should mirror it. `go/go.mod` and `go/go.sum` are in Track A's scope — if Track B needs a new Go runtime dep (creack/pty), the synthesis must name the handoff mechanism (Track B captures the require line, Track A folds during their work).
- **CLI integration (Track A)**: exact `src/striatum/cli/parser.py` line where `--core {python,go}` lands; exact dispatch hook in `src/striatum/cli/daemon.py`; exact subprocess launch shape (binary resolver order: shipped wheel binary → `STRIATUMD_GO_BIN` → `go/bin/striatumd`); `daemon_core` defaults to `python` (no implicit env-var flip).
- **Mutating verbs (Track A)**: every mutation in `src/striatum/cli/mutations.py` is enumerated against `go/pkg/rpc/registry.go` with the same capability binding as `src/striatum/daemon_rpc/registry.py`. Apply service + MCP + cross-repo layouts cite the Python entrypoints being mirrored.
- **Supervisor (Track B)**: `go/pkg/supervisor/{pointer.go,liveness.go,pty.go}` layout named; FIFO packet schema locked (line-delimited JSON shape) byte-compatible with the Python wrapper protocol; supervised-progress heartbeat mechanism matches Python; SIGTERM cleanup uses signal channel + waitgroup drain (the well-trodden Go pattern, per RFC 0039 §6).
- **Distribution (Track B)**: cross-compile targets named for all four platforms (linux-amd64, linux-arm64, darwin-amd64, darwin-arm64); top-level Makefile `daemon-go-build` / `daemon-go-install` / `daemon-go-release` targets named; `src/striatum/_daemongo/` package-data layout + `pyproject.toml` block + `MANIFEST.in` line locked; per-platform binary tagging (no Linux binary shipping into a macOS wheel); fallback resolver order documented.
- **CI matrix (Track B)**: `daemon_core={python,go}` on Linux + macOS; explicit jobs (not in-process parametrization) per dogfood-047 F3 finding; ephemeral Postgres wired; hard-fail-on-missing-PG sentinel for `CORE=go` so the matrix can't silently pass with all-skipped.
- **Backward-compat invariant**: `daemon_core` defaults to `python`; `--core go` is opt-in only. No implicit default flip lands here. Step 6 of THIS dogfood does NOT flip the default — that is a follow-up RFC per RFC 0039 §9 Phase 2.
- **D094 framing cited**: Go daemon implements RFC 0030 over the same Postgres schema as the Python daemon; the two cores are mutually exclusive at runtime via pidfile + socket-path lock. No parallel SQLite path.

Cite the synthesis section(s) you are challenging. Hand-waving findings ("the design is unclear") without a pinpoint citation will be down-weighted by the verdict gate.

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

**IMPORTANT — write REVIEW.md directly in this single supervised invocation.** If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
