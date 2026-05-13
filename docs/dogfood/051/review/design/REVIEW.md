---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0039", "v1-6", "design"]
---

author: reviewer-unknown-model-001

# Design Review (RFC 0039 V1.6, ergonomics_dx)

Fresh-context, document-only review of
`docs/dogfood/051/DESIGN_SYNTHESIS.md` against the three source designs
under `docs/dogfood/051/design/{codex,claude_code,gemini}/DESIGN.md`.
Posture per work packet: `ergonomics_dx`, judging whether a first-time
implementer can pick up the synthesis and execute it without
re-litigating decisions.

Verdict: **accept_with_findings**. The F-by-F mapping is clean, the
front matter validates, and the implementation order is concrete enough
to start tomorrow. But three medium gaps (F-store interface fit,
F-pid-recycling Linux parsing pitfalls, F-ci scope narrowing) and three
low gaps (PTY drain semantics, ordering rationale, thin acceptance
section) leave the synthesis short of self-sufficient — an implementer
reading only the synthesis will silently miss correctness landmines
that the lane designs flag. None of these block implementation because
the lane designs are still in scope; ergonomics_dx asks that the
synthesis *be* the contract.

## F1 — F-store `PointerRow` shape vs interface fit is ambiguous (medium)

The synthesis says (DESIGN_SYNTHESIS.md:26-29):

> **F-store:** Concrete `*pgxpool.Pool`-backed implementation in
> `go/pkg/db/supervisor_pointers.go`. Defines a local `PointerRow`
> shape that mirrors `supervisor.PointerRow` to avoid an import cycle.

All three lane designs target the existing `supervisor.PointerStore`
interface (codex DESIGN.md:36-37, claude_code DESIGN.md:163-173,
gemini DESIGN.md:103-129). That interface takes and returns
`supervisor.PointerRow`. If the synthesis introduces a *local*
`db.PointerRow` instead, the implementer faces a real fork the
synthesis does not resolve:

- (a) the store no longer satisfies `supervisor.PointerStore` — call
  sites must convert at the boundary, and the supervisor package now
  depends on the db package shape;
- (b) the store satisfies `supervisor.PointerStore` and the "local
  PointerRow" is only an internal mapping struct, in which case the
  decision wording is misleading;
- (c) `supervisor.PointerRow` is moved or re-exported from a leaf
  package both can import — a small refactor the synthesis does not
  name.

Codex DESIGN.md:99 also says `MarkSupervisorLost` must scope by
`(repository_id, supervisor_id)`; that constraint disappears from the
synthesis. The two lane field lists disagree on which columns
`PointerRow` carries — gemini DESIGN.md:181-194 names
`supervisor_id, repository_id, session_id, pid, started_at,
last_heartbeat_at, stdin_pipe_path, state, lost_reason`; codex
DESIGN.md:25-28 adds `DaemonSupervisorID, RunID, PIDStartTime,
UpdatedAt, MetadataJSON`. The synthesis is silent on the reconciliation.

A first-time implementer will read the synthesis, write a struct, and
discover the import cycle (or interface mismatch) on first `go build`.
That should be a synthesis-time call.

**Recommendation:** name the path explicitly — most likely (b) or (c)
— and pin the canonical `PointerRow` column list before
implementation.

## F2 — F-pid-recycling Linux parsing skips two known landmines (medium)

The synthesis says (DESIGN_SYNTHESIS.md:19-24):

> Linux path reads `/proc/<pid>/stat` field 22
> (clock-ticks-since-boot) and converts to absolute time via
> `/proc/stat`'s `btime` + assumed 100Hz `CLK_TCK`.

Two issues a first-time implementer will trip over:

1. **`CLK_TCK` is not always 100.** It is 100 on most modern Linux
   kernels but should be obtained via `sysconf(_SC_CLK_TCK)` (or
   `unix.SysconfClktck`), not assumed. Hard-coding 100 is fragile and
   the synthesis bakes it in without naming the syscall.
2. **The `comm` field in `/proc/<pid>/stat` contains spaces and
   parentheses.** A naive `strings.Fields` split misaligns every
   field after `comm` when the process name has a space. Codex
   DESIGN.md:78-81 calls this out explicitly ("split only after the
   final `") "`, then take field 22 from the full stat record, which
   is index 19 in the post-comm slice"). The synthesis drops the
   gotcha. An implementer who reads only the synthesis will write the
   fragile parser, miss it in unit tests (Go test child processes are
   `[exe]`), and hit it the first time a supervised lane is named
   `claude (worker 1)` or similar.

If Darwin support is genuinely out of scope for V1.6 — the synthesis
says fall back to signal-0 only on non-Linux — that constraint should
be lifted out of the F-pid-recycling decision and made an explicit
acceptance line, so reviewers don't wonder why two lane designs gave
Darwin-specific sketches that the synthesis discards.

**Recommendation:** add two sentences naming `sysconf(_SC_CLK_TCK)` and
the trailing-`") "` split rule. Both are one-liners; both prevent a
multi-hour debug session.

## F3 — F-ci scope narrows from lane designs without explanation (medium)

The synthesis says (DESIGN_SYNTHESIS.md:30-33):

> **F-ci:** Add a "Verify Go binary present" step in
> `.github/workflows/ci.yml` immediately after `make daemon-go-build`
> when `daemon-core == 'go'`. The step `exit 1`s with a clear stderr
> message when `go/bin/striatumd` is missing.

The three lane designs converged on three layered safeguards:

- codex DESIGN.md:131-136 — a Makefile precondition + a conftest-side
  raise when the resolver returns `None`;
- claude_code DESIGN.md:212-242 — a `STRIATUM_MULTI_REPO_REQUIRE_GO_BIN=1`
  env sentinel toggled in `tests/conftest.py` so the Python harness
  itself raises `pytest.UsageError` instead of skipping;
- gemini DESIGN.md:138-162 — a `tests/test_daemon_go_supervisor.py`
  assertion calling `resolve_go_binary()` and failing loudly when
  `CORE=go`.

The synthesis collapses all three into one CI-yaml step. That step is
load-bearing in only one place (the CI workflow); a local developer
running `make test-multi-repo CORE=go` after a `git clean -xfd` will
not see this guard at all. Dogfood-049 F1 ("`--core go` silently
inert") was exactly this failure pattern — a single guard that misses
the local path. From an ergonomics_dx view, narrowing three guards to
one without naming why is a regression in defense-in-depth.

**Recommendation:** either (a) keep at least the conftest.py raise so
the local-developer path also hard-fails, or (b) state in the
synthesis that local-developer skip-pass is an accepted residual risk
and name the next dogfood that closes it. As written, the implementer
cannot tell whether to keep the lane-design guards or drop them.

## F4 — F-pty decision is silent on drain semantics and D028 (low)

The synthesis says (DESIGN_SYNTHESIS.md:15-18):

> **F-pty:** Use `github.com/creack/pty v1.1.24`. The single-function
> `pty.Start(cmd)` returns the master fd which becomes the daemon's
> `StdinWriter`. Slave-side is wired automatically by creack/pty as
> the child's stdin/stdout/stderr.

What's missing for the implementer:

- The PTY master is bidirectional. Nothing in the synthesis says what
  drains stdout/stderr off the master. All three lane designs include
  a drain goroutine (codex DESIGN.md:53, claude_code DESIGN.md:66-68,
  gemini DESIGN.md:29-33) bounded by `cmd.Process.Wait()`. Without a
  drain, the kernel PTY buffer fills, the child blocks, and supervised
  lanes hang. This is the kind of foot-gun that breaks the second
  smoke test.
- Claude_code DESIGN.md:66-68 ties the drain to D028 (no transcript
  capture) by routing to `os.DevNull`. The synthesis never references
  D028; an implementer might naively log the drained bytes and quietly
  break a product invariant.
- The synthesis says "Slave-side is wired automatically by creack/pty
  as the child's stdin/stdout/stderr." That's true, but it conflicts
  visually with the existing `StdinPipePath`/FIFO machinery (gemini
  DESIGN.md:89-93, claude_code DESIGN.md:53-56). Does FIFO
  ensure-and-write still run alongside PTY mode? The synthesis is
  silent.

**Recommendation:** add one sentence — "drain goroutine reads from the
PTY master to `os.DevNull` (per D028) until `cmd.Process.Wait()`
returns" — and one sentence on FIFO mode coexistence.

## F5 — Implementation order contradicts lane-design rationale silently (low)

The synthesis Implementation Order is:

1. pointer.go (perms)
2. pty.go (perms + creack import + PTY launch)
3. liveness.go (start-time)
4. supervisor_test.go (flip PTY test)
5. db/supervisor_pointers.go (Postgres store)
6. go.mod / go.sum
7. ci.yml
8. RFC mark V1.6 closed

Claude_code DESIGN.md:247-254 and codex (by file order) both pin
F-store **before** F-pid-recycling so liveness assertions can run
against real Postgres rows. The synthesis flips that and puts the
store at step 5, after liveness. That may be fine — no hard test-suite
dependency — but the lane designs gave the reverse ordering for a
stated reason, and the synthesis silently reverses it. A reviewer or
implementer asking "was that intentional?" has no way to tell.

**Recommendation:** either follow the lane-design ordering or add a
one-line note that the synthesis intentionally lands the store after
liveness because (reason). Silent ordering flips are the ergonomics_dx
hazard.

## F6 — Acceptance section omits gates for three of the five findings (low)

The synthesis Acceptance section (DESIGN_SYNTHESIS.md:49-56) has four
gates:

- `cd go && go build ./...` clean
- `cd go && go test ./pkg/supervisor/...` green
- `cd go && go test ./pkg/db/...` green
- CI `daemon-core=go` axis fails fast when `go/bin/striatumd` is
  removed by hand between build + test steps

This covers F-pty (implicitly, via supervisor tests), F-store
(implicitly, via db tests), and F-ci (the last bullet). It does **not**
explicitly mention:

- **F-perms** — that a permission test asserts `0o700`/`0o600` and
  fails if a future refactor relaxes it. Without an explicit gate the
  finding is silently re-passable as a mode-bit regression.
- **F-pid-recycling** — that liveness marks a row lost on a
  recycled-pid scenario. Lane designs included a fake-provider
  injection test (claude_code DESIGN.md:125-130, gemini
  DESIGN.md:67-69); synthesis acceptance is silent on whether that
  test must exist.
- **Python harness** — `tests/test_daemon_go_supervisor.py` being
  promoted from placeholder. Two lane designs (codex DESIGN.md:39-41,
  claude_code DESIGN.md:34-39) made this an explicit acceptance signal
  because it's the proof that PTY launch works from the operator
  surface, not just from a Go-side unit test.

**Recommendation:** add three acceptance gates: explicit permission
test, recycled-pid liveness test, and
`tests/test_daemon_go_supervisor.py` no-skip behavior under
`STRIATUM_MULTI_REPO_DAEMON_CORE=go`.

## F7 — Positive: F-by-F mapping is clean

Five findings (F-pty, F-pid-recycling, F-perms, F-store, F-ci) — five
decisions, one per finding, in the same order. No drift between
section headings and the CHANGELOG-authoritative finding names. This
is the gold standard for synthesis ergonomics: a reader scanning the
document can ground each decision to a known follow-up without
cross-referencing.

## F8 — Positive: Front matter validates and author byline is correct

The synthesis carries the `striatum.synthesis.v1` schema_version /
artifact_kind pair (DESIGN_SYNTHESIS.md:1-4) and the byline
`author: designer-unknown-model-002` (DESIGN_SYNTHESIS.md:6) on the
right line. Publisher exit-code-6 surprises are off the table here.

## F9 — Positive: Implementation order is concrete and step-keyed

Eight numbered steps, each naming a single file. No "consider
refactoring X" or "if time permits" hedges. An implementer can land
each step as a separate small commit and tick them off. This is much
tighter than typical synthesis prose.

## Summary

| ID | Severity | Title | Where |
|----|----------|-------|-------|
| F1 | medium | F-store `PointerRow` interface fit ambiguous | DESIGN_SYNTHESIS.md:26-29 |
| F2 | medium | F-pid-recycling skips `CLK_TCK` syscall + `comm`-with-spaces parsing | DESIGN_SYNTHESIS.md:19-24 |
| F3 | medium | F-ci narrows three lane-design guards to one CI step | DESIGN_SYNTHESIS.md:30-33 |
| F4 | low | F-pty silent on drain goroutine and D028 | DESIGN_SYNTHESIS.md:15-18 |
| F5 | low | Implementation order silently flips F-store / F-pid-recycling sequence | DESIGN_SYNTHESIS.md:35-47 |
| F6 | low | Acceptance section omits F-perms, F-pid-recycling, Python harness gates | DESIGN_SYNTHESIS.md:49-56 |
| F7 | (positive) | F-by-F mapping is clean | DESIGN_SYNTHESIS.md:14-33 |
| F8 | (positive) | Front matter + author byline correct | DESIGN_SYNTHESIS.md:1-7 |
| F9 | (positive) | Implementation order concrete and step-keyed | DESIGN_SYNTHESIS.md:35-47 |

None of the findings block implementation — the lane designs cover the
gaps and an implementer who reads all four files has the context they
need. The verdict is **accept_with_findings** because the *synthesis*,
read in isolation, leaves enough ambiguity that a first-time
implementer will either re-derive missing detail from the lane designs
or hit a foot-gun (CLK_TCK assumption, `comm`-with-spaces parsing,
missing PTY drain, interface-vs-local-row decision). Closing F1-F3
inline would lift this to a clean `accept`.
