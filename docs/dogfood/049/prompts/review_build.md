# Build Review Prompt (RFC 0039 Phase 2, 3-way)

Produce REVIEW.md at `docs/dogfood/049/review/build/<lane>/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`:

- **codex**: `threat_model`
- **claude**: `ergonomics_dx`
- **gemini**: `threat_model` (adversarial angle)

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version: "striatum.finding.v1"` exact string):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0039", "phase-2", "build"]
---

author: reviewer-unknown-model-<NN>
```

`schema_version` is the exact string `"striatum.finding.v1"`. `artifact_kind` is `"finding"`. `verdict_intent` is one of `accept | accept_with_findings | needs_revision | reject`. `severity` is one of `low | medium | high | critical`. The byline is a plain markdown line AFTER the front matter — no lane prefix, no markdown bold.

Read both implementation handoffs at `docs/dogfood/049/build/track_a/HANDOFF.md` and `docs/dogfood/049/build/track_b/HANDOFF.md`. Cross-cut review across both tracks.

Per-lane angle:

- **codex (threat_model)**: every mutation in `src/striatum/cli/mutations.py` is registered on the Go core (`go/pkg/rpc/registry.go`) with the matching capability binding — no method bypass; apply-receipt fail-closed authority semantics preserved (RFC 0031 threat model intact); MCP `tools/list` filter denies unauthorized entries (RFC 0032); cross-repo lifecycle audit-row append fires; `--core go` subprocess launch closes file descriptors / sockets correctly (no FD leak across exec); `daemon_core` defaults to `python` (no implicit env-var precedence flip); supervisor SIGTERM cleanup deterministic (no orphan PTYs); FIFO packet schema byte-compatible with Python wrapper protocol; lost-detection sound under PID recycling; package-data binary tagged per-platform (no wrong-platform binary shipping in a wheel); CI matrix actually runs both `CORE=python` and `CORE=go` with hard-fail-on-missing-PG (per dogfood-047 F3 finding: matrix can't silently pass with all-skipped); D094 framing intact across both tracks.
- **claude (ergonomics_dx)**: `striatum daemon start --core go` legible error when binary missing (names `STRIATUMD_GO_BIN` + `make daemon-go-build` remediation); `--core go` composes with `--foreground` and `--socket`; wheel install ships the Go binary transparently OR names the install step in the error message; CI job names identify `CORE=python` vs `CORE=go` clearly; supervisor heartbeat + lost-detection visible in `striatum dashboard` (or note in HANDOFF that it falls through to the existing surface); Makefile targets discoverable via `make help` if a help target exists. Documentation deltas are operator-only — implementers do NOT touch README/TODO/CHANGELOG/SPEC/HOW_TO.
- **gemini (adversarial threat_model)**: `--core` flag escape paths (env var precedence, CLI alias, config file, ambient profile); a mutation in `src/striatum/cli/mutations.py` without a registered Go method — does it hard-refuse, silently fall through to Python, or 500? Apply-receipt forgery surface in the Go service; supervisor FIFO packet-injection (untrusted bytes from upstream); PID-recycling races in lost-detection; package-data binary substitution attack (wrong-platform or tampered binary on disk); CI matrix passing with Go binary missing (build failure masked as skip); daemon_core flip happening implicitly via env-var precedence. Cross-core compatibility: a Python CLI client at version N talking to a Go daemon at version N+1 refuses cleanly via the `daemon.hello` version handshake.

Required checks (all lanes):

- **`--core go` works**: `striatum daemon start --core go` launches the Go binary, serves the socket, accepts an envelope-v1 handshake from the Python CLI client. Test: `tests/test_daemon_go_*.py` against `MultiRepoHarness(daemon_core="go")`.
- **Mutating verbs work**: claim-next / ack / publish / complete / verdict / recovery all execute against the Go core and write audit rows. Test: `tests/test_daemon_go_mutations.py`.
- **Supervisor works**: supervised lane starts, packets deliver via FIFO, heartbeat fires, lost-detection recovers, SIGTERM cleanup is clean. Test: `tests/test_daemon_go_supervisor.py`.
- **CI matrix shipped**: `.github/workflows/` includes `CORE=python` and `CORE=go` as explicit jobs on Linux + macOS, with hard-fail-on-missing-PG.
- **Distribution shipped**: `make daemon-go-release` produces four per-platform binaries; `src/striatum/_daemongo/` package-data layout ships them in the wheel; the binary resolver order is correct.
- **Backward-compat**: existing test fixtures still pass against `daemon_mode=on` and `daemon_core="python"`. The Python daemon is unaffected.
- **Default unchanged**: `daemon_core` defaults to `python`. `--core go` is opt-in only. No implicit flip lands here.
- **Tests green**: `make test`, `cd go && go test ./...`, `make test-multi-repo CORE=python`, `make test-multi-repo CORE=go` all clean.

Cite specific files / lines / test names. "Looks good" is not a review.

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write and exit normally.
