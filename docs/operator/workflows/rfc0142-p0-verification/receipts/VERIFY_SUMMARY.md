# RFC 0142 P0 — verifier summary (supporting evidence)

author: verifier-reviewer-002

Supporting note for the adjudicator. The minted receipts in this directory are
the ground truth; this note only reports each receipt's `passed` +
`classification` faithfully and records the diagnosed root cause of each RED.
**No claim here upgrades a receipt** — every classification is exactly as the
engine minted it.

## Receipt ledger (faithful, from the engine's exit codes — never upgraded)

| check | receipt file | exit_code | `passed` | classification | re-exec agreement |
|-------|--------------|-----------|----------|----------------|-------------------|
| `builtin:go-build` | `receipt-go-build.json` | 1 | **false (RED)** | `asserted` | true |
| `builtin:go-vet`   | `receipt-go-vet.json`   | 0 | **true (GREEN)** | `asserted` | true |
| `builtin:go-test`  | `RECEIPTS.md`           | 1 | **false (RED)** | `asserted` | **false** |

Sandbox: `bubblewrap`, strict. `negative_control_void: false` on all three (no
receipt is voided by a passing negative control). Builtins self-pin the striatum
binary SHA and cap at **ASSERTED**; none reaches **VERIFIED** — no external
operator-pinned-and-attested check exists (`verification/allowlist.intent.json`
is empty, per SEED / RFC 0141).

## Why the two RED receipts are environment/test artifacts, not P0-code defects

The P0 work under verification is **test-harness + test code only** in
`go/pkg/pgtest` and `go/pkg/db` (the two-role fixture + 42501 oracle). Neither
RED is caused by that code.

### RED #1 — `go-build` / `go-test`: VCS-stamping fails in the strict sandbox

`go-build`'s captured stderr:

```
error obtaining VCS status: exit status 128
	Use -buildvcs=false to disable VCS stamping.
```

- Executed argv (from `receipt-go-build.json`):
  `go build -o /tmp/verifier-scratch-*/gobuild-out ./...` — the verifier inserts
  the `-o <scratch>` sink but does **not** pass `-buildvcs=false` (registry argv
  is `go build ./...` / `go test ./...`; see `go/pkg/verifier/builtin.go`).
- Cause: this per-job worktree's `.git` is a *pointer file*
  (`gitdir: <repo>/.git/worktrees/wt_…`) — the real gitdir lives **outside** the
  worktree. Inside the strict `bubblewrap` sandbox (constrained user; host
  gitconfig / `safe.directory` allowlist absent), git's VCS-status probe returns
  exit 128, so `go build`/`go test` abort **before compiling**. `go vet` stamps
  no binary, so it is unaffected → GREEN.
- Evidence the code is fine: outside the sandbox (same worktree, same lane user
  `striatum-lane`): `go build -buildvcs=false ./...` → **exit 0** and
  `git status` → exit 0. The RED reflects the sandbox's inability to do VCS
  stamping, not a build failure.

### RED #2 — `go-test`: a non-hermetic, unrelated test in `go/pkg/mutations`

Running the suite directly with `-buildvcs=false` (to get past RED #1) still
fails — on a single test that is **not part of the P0 change set**:

```
--- FAIL: TestSpawnRunAsSpecResolvesLaneUser (0.00s)
    spawn_grant_test.go:93: spec = {"mode":"daemon_user","run_as_user":""}, want lane_user striatum-lane
FAIL    github.com/halbritt/striatum/go/pkg/mutations
```

Root cause (pinned): `spawnRunAsSpec()` → `configuredLaneRunAsUser()`
(`go/pkg/mutations/supervision_env.go:228`) collapses to daemon-user mode when
the configured lane user **equals the current OS user**:

```go
daemonUser := currentOSUsername()
if daemonUser != "" && sameOSUsername(laneUser, daemonUser) { return "" }
```

`TestSpawnRunAsSpecResolvesLaneUser` hardcodes the lane user as the literal
`"striatum-lane"` and mocks only `laneOSUserHome` — **not** `currentOSUsername()`.
This verification lane runs **as the OS user `striatum-lane`**, so
`laneUser == daemonUser`, the spec collapses to `daemon_user`, and the assertion
fails. The test passes under any other runner identity — it is a
**test-hermeticity bug colliding with the dogfood lane's own username**,
pre-existing on the fork point and orthogonal to RFC 0142 P0 (which never touched
`pkg/mutations`).

Scope note: the verifier sandbox `go-test` run never reached this test — RED #1
aborts it at buildvcs. RED #2 was surfaced by the supplementary direct run and
is reported for completeness, not as a sandbox observation. (The `go-test`
receipt's `independent_reexecution_agreement: false` is from the two sandbox
re-executions of the buildvcs-aborted run, not from this test.)

## P0 code health (the actual work under verification)

- `go vet ./...` → exit 0 (verifier GREEN, ASSERTED).
- `go build -buildvcs=false ./...` → exit 0 (direct, outside sandbox).
- `go/pkg/pgtest` and `go/pkg/db` → `ok` in the direct `go test` run; the
  PG-backed two-role suites **skip** without `STRIATUM_PG_TEST_URL`, as designed.
- **DESIGNED-level (per SEED, not re-runnable in the no-network sandbox):** the
  PG-backed two-role suite was validated live **8/8** against a real cluster
  under a non-superuser owner DSN (the 42501 oracle reproduces). Stated as a
  DESIGNED claim with that evidence pointer — **not** claimed VERIFIED.

## Surfaced runner/test defects (escalated; not fixable within verify write scope)

1. **Verifier sandbox defect (blocks the gate for ANY repo under per-job
   worktree isolation):** builtin `go-build`/`go-test` do not pass
   `-buildvcs=false` (nor configure git `safe.directory` / mount the resolved
   gitdir), so they reliably false-RED on VCS stamping inside the strict sandbox.
   Fix: add `-buildvcs=false` to the `go-build`/`go-test` builtin argv (the
   verifier already self-pins the striatum binary SHA and does not rely on go's
   VCS stamp), or set `GOFLAGS=-buildvcs=false` / provision `safe.directory` in
   the sandbox env.
2. **Non-hermetic test:** `TestSpawnRunAsSpecResolvesLaneUser`
   (`go/pkg/mutations/spawn_grant_test.go`) must also mock `currentOSUsername()`
   (or use a lane-user literal that cannot equal the runner identity), so it does
   not fail when the test runner's OS user is named `striatum-lane`.

Both are stop-and-fix runner/test defects, **not** P0-code defects. Adjudicate on
the sealed receipts; this summary only supplies the diagnosed causes so the REDs
are not mistaken for P0 regressions.
