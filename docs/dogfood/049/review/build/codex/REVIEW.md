---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0039", "phase-2", "build"]
---

author: reviewer-unknown-model-001

# Build Review — RFC 0039 Phase 2

Verdict: needs_revision

## Trust Boundaries Reviewed

The reviewed change crosses these authority boundaries: Python CLI to selected daemon core, Python client to Go daemon over the RFC 0030 envelope, capability-token filtering in RPC and MCP, sealed-apply authority, cross-repo lifecycle/audit append, daemon-owned supervisor process control, FIFO packet delivery, PID/liveness detection, release packaging, and CI proof that both daemon cores execute under the same harness. I reviewed the supplied RFCs and the two implementation handoffs under the document-only review policy; I did not inspect repository source directly.

## Findings

### F1 — `daemon start --core go` is parsed but not dispatched

Severity: high

Track A adds the parser flag and resolver, but the handoff states that `src/striatum/cli/dispatch.py` still calls the Python daemon path directly and the new `launch_daemon_start(...)` helper is not connected [docs/dogfood/049/build/track_a/HANDOFF.md:21](/home/halbritt/git/striatum/docs/dogfood/049/build/track_a/HANDOFF.md:21), [docs/dogfood/049/build/track_a/HANDOFF.md:28](/home/halbritt/git/striatum/docs/dogfood/049/build/track_a/HANDOFF.md:28). This fails RFC 0039's acceptance criterion that `striatum daemon start --core go` launches the Go daemon binary through the Python CLI [docs/rfcs/0039-go-daemon-core.md:370](/home/halbritt/git/striatum/docs/rfcs/0039-go-daemon-core.md:370).

Threat impact: the operator-visible core selector is an escape path. A command line that appears to select Go can still run Python, which invalidates any security or parity evidence gathered under the assumption that the Go daemon handled the request.

Required fix: wire dispatch to the resolver/launcher, add an end-to-end test that observes the Go process or `daemon.welcome` core identity, and prove the default remains `python` unless `--core go` or an explicit configured default selects Go.

### F2 — Go mutation surface is registered but intentionally non-functional

Severity: high

Track A registers the RFC 0043 method vocabulary and broader mutation surface, but it also states that local participant mutation still requires a real local runner and that most registered mutation calls audit and return deterministic `not_implemented` [docs/dogfood/049/build/track_a/HANDOFF.md:14](/home/halbritt/git/striatum/docs/dogfood/049/build/track_a/HANDOFF.md:14), [docs/dogfood/049/build/track_a/HANDOFF.md:15](/home/halbritt/git/striatum/docs/dogfood/049/build/track_a/HANDOFF.md:15), [docs/dogfood/049/build/track_a/HANDOFF.md:30](/home/halbritt/git/striatum/docs/dogfood/049/build/track_a/HANDOFF.md:30). RFC 0043 requires every existing repo-local mutation to gain a registered RPC method so the daemon is complete, not partial [docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:91](/home/halbritt/git/striatum/docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:91). RFC 0039 Step 4 requires the full mutation verb table and full harness pass against `daemon_core="go"` [docs/rfcs/0039-go-daemon-core.md:427](/home/halbritt/git/striatum/docs/rfcs/0039-go-daemon-core.md:427).

Threat impact: fail-closed `not_implemented` is safer than method bypass, but it does not satisfy daemon-required semantics. If a CLI path silently falls back to Python or local runner behavior for these methods, capability/audit parity is bypassed. If it does not fall back, ordinary workflow verbs such as claim, ack, publish, complete, verdict, and recovery are unavailable on the Go core.

Required fix: implement the registered mutation handlers against the Postgres-backed repo-local schema, remove any Python/local-runner dependency for Go-core mutation execution, and run the required Go-core mutation harness with non-skipped assertions.

### F3 — Go supervisor does not meet the PTY/FIFO/liveness acceptance bar

Severity: high

Track B ships a supervisor package, but the PTY path returns a sentinel `"PTY launch not yet wired in Go core"` error and is explicitly deferred to V1.6 [docs/dogfood/049/build/track_b/HANDOFF.md:31](/home/halbritt/git/striatum/docs/dogfood/049/build/track_b/HANDOFF.md:31), [docs/dogfood/049/build/track_b/HANDOFF.md:93](/home/halbritt/git/striatum/docs/dogfood/049/build/track_b/HANDOFF.md:93). The Python parity test is scaffold-only, with FIFO write, heartbeat round-trip, and SIGTERM no-orphan-PTY assertions also deferred [docs/dogfood/049/build/track_b/HANDOFF.md:84](/home/halbritt/git/striatum/docs/dogfood/049/build/track_b/HANDOFF.md:84), [docs/dogfood/049/build/track_b/HANDOFF.md:98](/home/halbritt/git/striatum/docs/dogfood/049/build/track_b/HANDOFF.md:98). Concrete Postgres-backed pointer storage is interface-only and left to a follow-up [docs/dogfood/049/build/track_b/HANDOFF.md:104](/home/halbritt/git/striatum/docs/dogfood/049/build/track_b/HANDOFF.md:104).

This fails RFC 0039's requirement that the Go daemon own supervised processes including PTY, signal handling, and supervised-progress heartbeat [docs/rfcs/0039-go-daemon-core.md:366](/home/halbritt/git/striatum/docs/rfcs/0039-go-daemon-core.md:366), as well as the implementation step requiring PTY spawning, FIFO packet delivery, heartbeat, deterministic SIGTERM cleanup, and real supervised-lane smoke testing [docs/rfcs/0039-go-daemon-core.md:435](/home/halbritt/git/striatum/docs/rfcs/0039-go-daemon-core.md:435).

Threat impact: the liveness and cleanup boundary remains unproven. PID-recycling soundness cannot be accepted from an in-memory fake plus signal-0 probe alone, and the no-orphan-PTY claim is explicitly untested.

Required fix: wire `creack/pty` or an accepted equivalent, add the Postgres-backed pointer store, make `tests/test_daemon_go_supervisor.py` functional, and assert FIFO byte compatibility, heartbeat updates, PID identity checks, and SIGTERM cleanup end-to-end.

### F4 — Distribution ships all platform binaries through generic package data

Severity: medium

Track B says `daemon-go-release` cross-compiles all four target binaries and stages them into the wheel package-data tree [docs/dogfood/049/build/track_b/HANDOFF.md:51](/home/halbritt/git/striatum/docs/dogfood/049/build/track_b/HANDOFF.md:51), while `pyproject.toml` includes `striatum._daemongo` `binaries/*` and `MANIFEST.in` recursively includes the tree [docs/dogfood/049/build/track_b/HANDOFF.md:61](/home/halbritt/git/striatum/docs/dogfood/049/build/track_b/HANDOFF.md:61), [docs/dogfood/049/build/track_b/HANDOFF.md:64](/home/halbritt/git/striatum/docs/dogfood/049/build/track_b/HANDOFF.md:64). The release workflow then builds the wheel and sdist after producing all four binaries, so the artifact shape described by the handoff is a generic Python package carrying every platform binary [docs/dogfood/049/build/track_b/HANDOFF.md:76](/home/halbritt/git/striatum/docs/dogfood/049/build/track_b/HANDOFF.md:76).

Threat impact: this violates the review requirement that package-data binaries be tagged per-platform and avoid shipping a Linux binary into a macOS wheel. The resolver may select by platform at runtime, but the wheel/sdist still carry wrong-platform executable payloads, which increases substitution and review surface.

Required fix: either keep Go binaries as separate release artifacts per RFC 0039's V1 recommendation or produce platform-tagged wheels whose package data contains only the matching binary. Add a package smoke test that inspects wheel contents for each target.

## Positive Notes

The build preserves several fail-closed properties in the handoff narrative. Apply service behavior returns `sealed_key_missing` without key material and `apply_gate_unsatisfied` when authority prerequisites are absent [docs/dogfood/049/build/track_a/HANDOFF.md:12](/home/halbritt/git/striatum/docs/dogfood/049/build/track_a/HANDOFF.md:12). MCP tool visibility is capability-filtered [docs/dogfood/049/build/track_a/HANDOFF.md:13](/home/halbritt/git/striatum/docs/dogfood/049/build/track_a/HANDOFF.md:13). The CI plan names explicit `python` and `go` daemon-core jobs and hard-fails missing Postgres via `STRIATUM_MULTI_REPO_REQUIRE_PG=1` [docs/dogfood/049/build/track_b/HANDOFF.md:70](/home/halbritt/git/striatum/docs/dogfood/049/build/track_b/HANDOFF.md:70). These are the right directions, but they are not enough to accept the build while the core launch, mutation, supervisor, and package-boundary requirements remain unmet.

## Required Before Accept

- Prove `striatum daemon start --core go` actually starts the Go daemon and that `daemon_core` defaults to `python`.
- Implement Go-core mutation handlers for the full RFC 0043 method registry and verify claim, ack, publish, complete, verdict, and recovery against `daemon_core="go"`.
- Land functional supervisor parity: PTY, FIFO packet delivery, heartbeat, PID identity/lost detection, and SIGTERM cleanup.
- Fix binary distribution so installed artifacts are platform-correct, then add wheel-content smoke coverage.
- Re-run the required gates: `make test`, `cd go && go test ./...`, `make test-multi-repo CORE=python`, and `make test-multi-repo CORE=go` with hard-fail-on-missing-Postgres enabled.
