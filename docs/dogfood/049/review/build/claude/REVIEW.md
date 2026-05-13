---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["ergonomics_dx", "rfc-0039", "phase-2", "build"]
---

author: reviewer-unknown-model-002

# Build Review (RFC 0039 Phase 2, ergonomics_dx)

Operator-UX cross-cut of Track A (`docs/dogfood/049/build/track_a/HANDOFF.md`)
and Track B (`docs/dogfood/049/build/track_b/HANDOFF.md`). Posture per work
packet: `ergonomics_dx`, first-time-user surface.

Verdict: **needs_revision**. Two high-severity silent-failure bugs land on
the very first operator gesture (`striatum daemon start --core go`), plus
several medium discoverability gaps. The implementation is structurally
sound — registries, supervisor primitives, CI matrix, and distribution
plumbing are all present — but the user-visible surface contradicts the
HANDOFFs in places that a first-time operator will discover by silently
falling back to the Python daemon and assuming the Go path works.

## F1 — `--core go` is silently inert (high)

`src/striatum/cli/daemon.py:76-80` defines `launch_daemon_start(args)` which
dispatches to `run_python_daemon_foreground` or `run_go_daemon_foreground`
based on `resolve_daemon_core(...)`. But the top-level dispatcher at
`src/striatum/cli/dispatch.py:888-893` still calls
`daemon_mod.run_daemon_foreground(...)` directly:

```python
if args.daemon_command == "start":
    return daemon_mod.run_daemon_foreground(
        sweep_interval_seconds=float(args.sweep_interval_seconds),
        max_sweeps=args.max_sweeps,
        postgres_url=getattr(args, "postgres_url", None),
    )
```

`args.core` is parsed (parser.py:142-150) and surfaces in `--help`, but is
never consulted. Result: `striatum daemon start --core go` silently runs
the Python daemon. There is no error, no warning, no log line; an operator
following the HANDOFF will believe they are exercising the Go core when
they are not.

Track A's HANDOFF acknowledges this as a deviation ("the new
`launch_daemon_start(...)` helper is present but not connected"), but a
shipped flag that does nothing fails the basic first-time-user test.
Tests `tests/cli/test_daemon_core.py` only assert parser shape and resolver
behavior — they do not exercise the end-to-end `daemon start --core go`
path, so this regression is not caught.

**Recommendation:** wire `_dispatch_daemon` to call
`striatum.cli.daemon.launch_daemon_start(args)` for `daemon_command == "start"`
(this is a one-line change touching the previously-forbidden file). Add an
integration test that asserts the launch helper is reached when `--core go`
is passed — fakes for `os.execv` are sufficient. If the wiring genuinely
cannot land in this phase, fail loudly: refuse `--core go` at parse time
with exit 9 and a "not yet wired in this build" message naming the V1.6
follow-up.

## F2 — Wheel-shipped Go binary is invisible to the resolver (high)

Track B ships `src/striatum/_daemongo/__init__.py` exposing
`find_binary() -> Path | None` and declares it via
`[tool.setuptools.package-data]`. Track A's resolver
(`src/striatum/cli/daemon.py:105-122`) looks for the packaged binary via:

```python
for name in ("resolve_binary", "binary_path", "path"):
    resolver = getattr(_daemongo, name, None)
```

`find_binary` is not in that list. Result: a wheel install with the binary
present under `src/striatum/_daemongo/binaries/striatumd-<slug>` will skip
the packaged path, fall through `STRIATUMD_GO_BIN` (unset), miss
`go/bin/striatumd` (not in a wheel install), miss PATH, and exit 2 with
"Go daemon binary not found" — even though the wheel did ship the binary.

This is a contract mismatch between Track A and Track B. The build review
objective ("wheel install ships the Go binary transparently OR names the
install step") fails in the silent direction: the binary ships but is not
findable.

**Recommendation:** rename the loop tuple in `_resolve_packaged_go_binary`
to include `"find_binary"` (or rename Track B's helper to `path` /
`resolve_binary` — Track B's module is already in Track A's write scope
indirectly through CLI dispatch). Add a regression test that drops a fake
binary under `_daemongo/binaries/` and asserts `resolve_go_binary()`
returns it.

## F3 — Missing-binary error omits the `make` remediation (medium)

`src/striatum/cli/daemon.py:99-102` raises:

```
Go daemon binary not found; set STRIATUMD_GO_BIN or build go/bin/striatumd
```

The work packet objective explicitly required the error to name
`make daemon-go-build` as the remediation verb. "Build go/bin/striatumd" is
the *effect*; `make daemon-go-build` is the *command*. A first-time user
without prior context will not know the Makefile target exists — the
Makefile has no `help` target (see F5).

**Recommendation:** change the error to
`"Go daemon binary not found; run \`make daemon-go-build\` or set STRIATUMD_GO_BIN=/path/to/striatumd"`.
Optionally add a one-line hint pointing to `make daemon-go-install` for
editable installs.

## F4 — Supervisor heartbeat + lost-detection invisible in `striatum dashboard` (medium)

`go/pkg/supervisor/liveness.go` ships heartbeat and lost-detection wired
to `PointerStore.UpsertSupervisorPointer` / `MarkSupervisorLost`. But
`src/striatum/dashboard.py` contains no `supervisor`, `heartbeat`, or
`lost` references (grep confirmed). Neither HANDOFF notes that dashboard
visibility "falls through to an existing surface" (the build objective
admits that escape hatch). There is no existing surface.

For supervised lanes this means an operator running
`striatum dashboard --run-id <id>` cannot see whether the Go supervisor
is heartbeating or whether a process has been marked lost. The data is
written to `striatumd.process_supervisor_pointers`; the dashboard simply
does not project it.

**Recommendation:** add a `Supervisors` panel to `striatum dashboard`
sourcing `striatumd.process_supervisor_pointers` (state, last_heartbeat_at,
lost_reason). If that lands outside this phase, add an explicit deferral
note to Track B's HANDOFF and a brief "Operator visibility" subsection
naming the SQL the operator can run today (e.g.
`striatum daemon audit --limit 50`-style query) so the surface gap is
documented rather than discovered.

## F5 — Makefile targets undiscoverable (low)

`make help` does not exist. Targets `daemon-go-build`, `daemon-go-test`,
`daemon-go-lint`, `daemon-go-install`, `daemon-go-release` are documented
only inline in the Makefile (lines 12, 70-96) and in the Track B HANDOFF.
The build objective conditioned this finding on a `help` target being
present ("if present"), so this is not a refusal, but the absence is now
load-bearing: it amplifies F3 because operators have no `make help` to
fall back on after the binary-missing error.

**Recommendation:** add a `help` target that greps `## ` doc lines from
the Makefile (standard idiom). Cheap, opens up every Makefile-routed
operator gesture, and makes F3's remediation discoverable.

## F6 — `--core go` does not compose with `--socket` / `--foreground` (low)

The build objective asked: "`--core go` composes with `--foreground` and
`--socket`". Neither flag exists on `daemon start` (parser.py:141-154
defines only `--core`, `--sweep-interval-seconds`, `--max-sweeps`,
`--postgres-url`, `--json`). `daemon start` is always foreground today,
so `--foreground` being absent is acceptable, but the socket path is
hard-coded inside `run_go_daemon_foreground` via `daemon_mod.socket_path()`
with no operator override.

Additionally, `--sweep-interval-seconds` and `--max-sweeps` are accepted
on the command line but `run_go_daemon_foreground` does not forward them
(the Go core has a different sweep model). Silently ignored flags are
worse than rejected ones.

**Recommendation:** either (a) accept `--socket /path` on `daemon start`
and forward it to the Go path, or (b) document explicitly in the parser
help text that the Go core uses the daemon-resolved socket. For the
silently-dropped Python-only flags, mark them mutually exclusive with
`--core go` at parse time so the operator sees an immediate exit-2 refusal
instead of an unobservable no-op.

## F7 — Positive: PTY sentinel error is well-shaped

`go/pkg/supervisor/pty.go:47` returns:

```
supervisor: PTY launch not yet wired in Go core; set USE_PTY=false or fall back to Python supervisor (RFC 0039 V1.6 follow-up)
```

This is the ergonomics_dx gold standard for a deferred feature: names the
limitation, names the workaround, names the follow-up. Keep doing this.
The other not-yet-wired surfaces (F1 in particular) should adopt this
shape.

## F8 — Positive: CI matrix axis identity is clean

`.github/workflows/ci.yml:18` adds `daemon-core: ["python", "go"]` as a
top-level matrix axis. GitHub Actions appends matrix axis values to job
names by default, so an operator looking at the Actions UI will see
`test (ubuntu-latest, 3.11, python)` vs `test (ubuntu-latest, 3.11, go)`
without further work. The `STRIATUM_MULTI_REPO_REQUIRE_PG=1` sentinel
(ci.yml:47) addresses the dogfood-047 F3 finding directly — an
all-skipped pass on the go axis is now impossible.

## F9 — Positive: default `daemon_core` is preserved

`resolve_daemon_core(None)` with `STRIATUM_DAEMON_CORE` unset returns
`"python"` (`src/striatum/cli/daemon.py:47-55`, tested by
`tests/cli/test_daemon_core.py:27-30`). `--core go` is strictly opt-in.
No implicit env-var flip lands here. Phase 2 default contract intact.

## Summary

| ID | Severity | Title | Where |
|----|----------|-------|-------|
| F1 | high | `--core go` is silently inert | `src/striatum/cli/dispatch.py:888-893` |
| F2 | high | Wheel-shipped Go binary invisible to resolver | `src/striatum/cli/daemon.py:105-122` vs `src/striatum/_daemongo/__init__.py:34` |
| F3 | medium | Missing-binary error omits `make daemon-go-build` | `src/striatum/cli/daemon.py:99-102` |
| F4 | medium | Supervisor heartbeat invisible in `striatum dashboard` | `src/striatum/dashboard.py`, `go/pkg/supervisor/liveness.go` |
| F5 | low | No `make help` target | `Makefile` |
| F6 | low | `--core go` does not compose with `--socket`; silently drops Python-only flags | `src/striatum/cli/parser.py:141-154`, `src/striatum/cli/daemon.py:66-73` |
| F7 | (positive) | PTY sentinel error names limitation, workaround, follow-up | `go/pkg/supervisor/pty.go:47` |
| F8 | (positive) | CI matrix axis identifies CORE clearly + hard-fail on missing PG | `.github/workflows/ci.yml:18,47` |
| F9 | (positive) | Default `daemon_core` is `python`, no implicit flip | `src/striatum/cli/daemon.py:47-55` |

F1 and F2 are blockers from an ergonomics_dx perspective because they turn
the entire Phase 2 operator surface into a silent-fallback experience.
Fix the dispatcher wire-up and the resolver name list; the rest of the
review is medium/low and tractable inside this phase or as a V1.6
follow-up.
