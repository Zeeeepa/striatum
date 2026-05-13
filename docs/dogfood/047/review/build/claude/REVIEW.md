---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0039", "v1.5", "build"]
---

author: reviewer-unknown-model-002

# Build Review — RFC 0039 V1.5 (claude, ergonomics_dx posture)

Scope of evidence (per `review_policy.access_scope = document_only`,
`context_policy = fresh`):

- `docs/rfcs/0039-go-daemon-core.md` (the V1.5 deltas section is the
  spec under review).
- `docs/dogfood/042/track_a/review/build/{codex,claude,gemini}/REVIEW.md`
  (the V1 build reviews that named the ergonomics defects V1.5 is meant
  to close).

I did not consult the repository tree, the V1.5 implementation packets,
the dogfood-047 design synthesis, or the build handoff. The review
prompt's "Required checks" section instructs ripgrep verification of
artifacts on disk; those are unverifiable from inputs alone and are
flagged below where the document evidence is silent or thin. A
second-pass build review with `access_scope = code` will be needed to
finish the acceptance bar.

Lane angle (per the prompt's per-lane split): **first-time-user
ergonomics** — the harness launch contract, `make test-multi-repo
CORE=go` discoverability and CI parity story, and operator-readable
error surfaces from the new authorizer + transactional audit append.
The codex lane owns F1/F4 correctness, the gemini lane owns F4/F5
adversarial supply-chain.

Verdict: **accept_with_findings** at severity **low**. The V1.5 deltas
text (RFC 0039 §"V1.5 Deltas (correctness slice)", lines 493-569)
materially closes the four headline ergonomics blockers from the V1
reviews. The findings below are residual surface concerns and
unverifiable claims that the next review round should close at
`access_scope = code`.

## What the V1 reviews told V1.5 to fix (baseline)

Cross-referencing the dogfood-042 build reviews, four ergonomics
defects gated `daemon_core="go"` from being a usable first-time-user
path:

1. **Flag wiring break.** Harness invoked `--db-url` /
   `--migrations-dir`; binary accepted `--postgres-url` / `--migrate`
   (codex F2 lines 28-34; claude F1 lines 82-118).
2. **Binary path mismatch.** `make -C go build` emitted `go/striatumd`;
   harness probed `go/bin/striatumd` (claude F2 lines 128-156).
3. **CI parity not wired.** Top-level `Makefile` had no `CORE`
   plumbing; `tests/conftest.py` constructed `MultiRepoHarness(...)`
   without `daemon_core`; no pytest parametrization or job for the Go
   core (codex F3 lines 36-42).
4. **Doc-honesty break.** `HOW_TO_HUMAN.md` Go-daemon section was
   non-executable as written because each command landed on a
   flag/path the binary did not implement (claude F7 lines 261-291).

A fifth ergonomics-adjacent finding — `AllowAllAuthorizer` wired in
production main (codex F1 lines 19-26; claude F12 lines 491-510) —
sits on the line between threat-model and ergonomics: the *symptom*
operators would have seen first is "every method passes the
authorizer regardless of token," which is a denial-reason ergonomics
gap as much as an authorization one.

These five items are the V1 ergonomics floor V1.5 has to clear before
the next CI round.

## How V1.5 reads against that floor

### F2 (Go harness launch contract) — RFC lines 547-558

The V1.5 text commits the Go binary to a fixed flag surface:
`--socket`, `--postgres-url`, `--migrate`, `--describe`, plus the
new optional `--migrations-sha-source`. `go/Makefile` writes
`go/bin/striatumd`, and `STRIATUMD_GO_BIN` is preserved as a
"trusted developer-environment override."

From the first-time-user perspective this is the single most
important ergonomics fix in V1.5:

- The harness no longer has to know two different flag vocabularies.
- The auto-build path described in the V1 claude review (F2: "even if
  F1 were fixed, the harness still cannot find the just-built
  binary") is closed by the explicit `go/bin/striatumd` target.
- The "trusted developer-environment override" phrasing for
  `STRIATUMD_GO_BIN` lines up with the V1 claude review F5 ask
  (lines 195-201) for a one-line context comment on the override.

Residual surface concerns — see Finding F-DX-1 and F-DX-2 below.

### F3 (`make test-multi-repo CORE=go`) — RFC lines 560-569

The V1.5 text wires `CORE ?= python` at the top-level `Makefile`,
forwards through `STRIATUM_MULTI_REPO_DAEMON_CORE`, and reads that
env var in a class-scoped `daemon_core` fixture in
`tests/conftest.py`. New tests are listed:
`tests/test_daemon_go_smoke.py` (the V1 reviews' missing smoke test)
and `tests/test_daemon_go_audit.py` (the F4 cross-core regression).
CI shape is "two explicit jobs (CORE=python, CORE=go) rather than
in-process parametrization, so the Go-core evidence is intentional
rather than implied."

This closes V1 codex F3 ("the Go parity target is declared but not
wired") at the design level. The explicit-jobs-over-parametrization
choice is well-motivated for first-time CI debugging: a Go-core
failure surfaces as a separately-named job rather than as a
parametrized subtest the operator has to scroll through.

Residual surface concerns — see Finding F-DX-3 and F-DX-4 below.

### F1 (PostgreSQL authorizer) — RFC lines 534-545

The V1.5 text moves `AllowAllAuthorizer` to test-only and wires
`PostgresAuthorizer` when a PostgreSQL URL is configured. The key
ergonomics commitment is:

> Denial reasons line up one-for-one with
> `src/striatum/daemon_rpc/capability.py` so clients cannot tell the
> two cores apart from the refusal envelope.

For ergonomics_dx this is the right contract: cross-core denial
parity means an operator debugging a `denied` envelope against
either daemon sees the same vocabulary and can reach for the same
runbook. Constant-time compare via `subtle.ConstantTimeCompare` is
a correctness-not-ergonomics property but contributes to the
"clients cannot tell the two cores apart" guarantee.

Residual surface concern — see Finding F-DX-5 below.

### F4 (transactional audit append) — RFC lines 521-532

The V1.5 text describes a single `READ COMMITTED` transaction with
`FOR UPDATE` on `striatumd.audit_chain_head`, compute, insert with
`RETURNING audit_id`, update chain head, commit. The Python
cross-core regression lives in `tests/test_daemon_go_audit.py` and
runs under `make test-multi-repo CORE=go`. The Go race test in
`go/pkg/db/audit_race_test.go` exercises this against an ephemeral
Postgres URL via `STRIATUM_PG_TEST_URL`.

For ergonomics this matters in two ways:

- The V1 reviews (codex F4 lines 44-49; gemini lines 42-47; claude
  F10 lines 360-453) collectively said the chain could fork under
  concurrent appenders, which would surface to operators as an
  `audit.show` verifier refusal. V1.5's design closes that.
- The `RETURNING audit_id` plus "the returned audit id flows back
  into the RFC 0030 response" closes the V1 claude F10 lines 425-446
  observation that the Go core returned an empty `audit_id` to
  clients — an RFC 0030 envelope-shape regression that broke client
  follow-up `audit.show` lookups.

Residual surface concern — see Finding F-DX-6 below.

### F5 (pure-Go driver) — RFC lines 506-518

Ergonomics-adjacent. The shift to `pgx/v5` from `exec.Command("psql",
...)` removes a class of operator surprise — an unpinned external
executable on PATH, "no statement timeout, no idle-in-transaction
timeout, no version floor check" (claude V1 F11 lines 463-488), and
"the URL contains the password and is visible in `/proc/<pid>/cmdline`"
(claude V1 F11 line 470; gemini lines 35-41). The configured
`application_name = "striatumd-go/<daemon_version>"` gives the V1
SPEC.md "mutual exclusion via `pg_stat_activity`" claim something to
inspect against — closing the V1 claude F3 lines 158-178 doc-honesty
gap at the mechanism level.

The decision to keep the PostgreSQL simple protocol so multi-statement
migrations work unchanged "while parameters are still bound through
the driver" is a thoughtful first-time-user accommodation: operators
who built mental models around the existing SQL files don't have to
relearn.

Residual surface concern — see Finding F-DX-7 below.

## Findings (ergonomics_dx)

### F-DX-1 — `--migrations-sha-source` flag name is non-obvious (low)

The new flag (RFC line 553) replaces V1's `--migrations-dir`
reloader. The behavior is "compare embedded migration file hashes
against the SQL files on disk before serving and exit non-zero on
drift." From an operator-discoverability angle:

- A first-time user reading `--help` output will not infer "compare
  embedded SHA against on-disk SHA, exit non-zero on drift" from the
  name. "SHA source" is unusual terminology; users may expect a path
  to a checksum file, or a URL, or an enumerated source like `git` /
  `disk`.
- The V1 claude review F6 lines 220-255 named this drift class
  explicitly. The new flag fixes the mechanism but the name does not
  signal the mechanism.

Suggested direction (not in scope): a clearer name like
`--verify-migrations-against <path>` or `--migrations-source-dir
<path-for-sha-check-only>`, plus a `--help` blurb that states the
exit-non-zero-on-drift behavior. The RFC text describes the behavior
but does not specify the help text the operator will read.

### F-DX-2 — `daemon_core` in `daemon.welcome` carry-over from V1 not addressed (low)

V1 claude review F9 lines 336-347 noted that `daemon.welcome` should
include `daemon_core` so a Python client can distinguish which core
answered, "which defeats the operator-visibility motivation in the
UBIQUITOUS_LANGUAGE entry." The V1.5 deltas section is silent on
this. For the ergonomics_dx posture this matters because the entire
F1-F5 work increases the surface where "did I just hit the Go core
or the Python core" becomes a frequent operator question (failed
auth, audit chain anomaly, migration drift exit code, etc.).

Suggested direction (not in scope): include `daemon_core` in
`daemon.welcome` and surface it in `striatum daemon describe` text
output. The RFC should either commit V1.5 to that or explicitly
defer it to Step 4 with a note that operators may need to inspect
the binary path or `pg_stat_activity.application_name` until then.

### F-DX-3 — Smoke / audit test default-collection scope is unstated (low)

The V1.5 text says "the test list now includes
`tests/test_daemon_go_smoke.py` and `tests/test_daemon_go_audit.py`
so the Go-core matrix exercises a real boot, a read-only RPC, and
the F4 audit chain." It is unclear whether these files run on a
default `pytest tests/` invocation (no `CORE` set), and if so, what
happens when no PostgreSQL is available.

Three scenarios that ergonomics_dx cares about:

1. Operator runs `pytest tests/` locally without setting `CORE=go`.
   Do the Go-core tests skip cleanly, or do they import-error /
   fail-to-collect because the Go binary or PG fixture is missing?
2. Operator runs `pytest tests/test_daemon_go_smoke.py` directly to
   debug a Go-core launch issue. Does the file self-select the Go
   core, or does it require `STRIATUM_MULTI_REPO_DAEMON_CORE=go` /
   `CORE=go` in the environment?
3. CI runs the `CORE=python` job. Are the Go-only tests
   `pytest.mark.skipif(daemon_core != "go")` or are they collected
   and skipped via fixture, and is the skip reason readable?

The RFC text is silent on these. Suggested direction: state the skip
contract (which marker, what reason text, what env vars trigger
which behavior) in the V1.5 deltas or in `HOW_TO_HUMAN.md`. The V1
claude review F7 lines 261-291 spent significant text on
doc-honesty for the operator-facing surface; the same standard
should apply to the test-discoverability surface.

### F-DX-4 — CI job naming and surfacing is not committed in the RFC (low)

The V1.5 text says CI is "two explicit jobs (`CORE=python`,
`CORE=go`)" which is the right shape. But the RFC does not name the
jobs, does not say whether they appear as separate GitHub Actions
job statuses on the PR check list, and does not commit a failure-
mode story (e.g., does the `CORE=go` job block PR merge, or is it
advisory during Phase 1 V1.5?).

For first-time-CI-debugging ergonomics, a job named
`test-multi-repo (CORE=go)` on the PR check list is materially
different from a single job that internally branches. The RFC
should commit to the job naming and the gating policy. Suggested
direction (not in scope): one sentence in §V1.5 Deltas or in §10 CI
matrix naming the two job names and the gating policy (blocking
vs. advisory) for V1.5.

### F-DX-5 — `PostgresAuthorizer` denial reason vocabulary is not enumerated (low)

The V1.5 text says denial reasons "line up one-for-one with
`src/striatum/daemon_rpc/capability.py`." This is the right
contract. But the RFC does not enumerate the denial reasons the V1.5
Go authorizer surfaces. For an operator debugging a denied call
without access to the Python reference, the documented set matters.

The V1 claude review F8 lines 295-334 flagged the same class of
issue at the registry level: "advertised but unimplemented" is
worse than a smaller surface. The same standard applies here for
denial vocabulary: an operator should not have to read
`capability.py` to know what `scope_mismatch` vs.
`capability_missing` vs. `revoked` vs. `expired` mean from the Go
core.

Suggested direction: enumerate the denial reasons in the RFC V1.5
deltas section, ideally with the Python source pinned by file +
function. The "clients cannot tell the two cores apart from the
refusal envelope" guarantee is much stronger if both vocabularies
are stated in one place the operator can read.

### F-DX-6 — Transaction conflict and lock-wait error surface is unspecified (low)

The V1.5 text commits to `READ COMMITTED` + `FOR UPDATE` on
`striatumd.audit_chain_head`. The first-time-user error path is not
discussed:

- Under high concurrency, the `FOR UPDATE` row lock serializes the
  insert path. Operators may see slow audit appends. The RFC does
  not say what the daemon logs, what the RPC envelope `refusal_code`
  is (if any) when the lock wait exceeds a timeout, or whether
  there is any retry / backoff in the Go side.
- If the singleton `striatumd.audit_chain_head` row does not exist
  yet (cold start), the `FOR UPDATE` would return zero rows. The
  RFC does not say how that is bootstrapped or how the Go core
  responds.

The V1 codex review F4 lines 44-49 framed this as a chain-fork
issue. V1.5 closes the fork at the design level. The new operator
question is "what does it look like when the lock contends." For
ergonomics_dx this is the kind of paper-thin operator surface that
turns into a Stack-Overflow-style runbook question on first
deployment.

Suggested direction: state the lock-wait error surface and the
cold-start behavior in the V1.5 deltas section. A one-paragraph
"operator-visible failure modes" subsection under F4 would close
this.

### F-DX-7 — `pgx/v5` is the first third-party Go dependency; operator-visible install impact is acknowledged but not narrated (low)

RFC line 510 says: "`pgx/v5` (the first third-party Go runtime
dependency for this repository — `go/go.mod` now requires
`github.com/jackc/pgx/v5 v5.7.2`, with five indirect modules)." This
is the right honesty. For first-time-user ergonomics, the operator-
visible impact deserves one more sentence:

- A contributor cloning the repo and running `make -C go build` for
  the first time will now hit a `go mod download` of six modules
  including transitive deps. The V1 build was network-free.
- If the corporate network blocks `proxy.golang.org`, the V1.5 build
  fails where V1 succeeded.

The V1 gemini review §2 lines 22-25 and V1 claude review TB-Glue-2
lines 202-217 both called the empty `go.sum` a supply-chain positive
for Phase 1. Trading that for `pgx/v5` is the right call (it fixes
the bigger ergonomics problems with `psql` shell-out per F5), but
the trade-off deserves to be documented for operators in
`HOW_TO_HUMAN.md` ("the Go core's first build downloads
github.com/jackc/pgx/v5 and five indirect modules; on offline
networks set GOPROXY=off and pre-populate the module cache") not
buried in the RFC.

### F-DX-8 — `RFC 0039 V1.5 Deltas` section is the only authoritative spec; status block is stale (informational)

The RFC's Status line and Implementation Plan steps 3-6 still read
as "deferred to a Phase 2 dogfood." The V1.5 section adds correctness
work in service of Step 4 (per its preamble: "V1.5 closes those
gaps and is the merge slice before mutating routes land in Step 4").
For a first-time reader of the RFC the relationship between V1 (per
the Status line), V1.5 (the new section), and Step 4 (Phase 2) is
not signposted. A short paragraph in the Status line — "V1.5 closes
F1/F4/F5 correctness gaps and rewires F2/F3 ergonomics before
mutating routes; Steps 3-6 remain deferred" — would orient the
first-time RFC reader before they hit the Implementation Plan.

Not a blocker for V1.5; flagged for completeness.

## Required-check coverage gaps (document_only constraint)

The review prompt enumerated six "Required checks (all lanes)":

- **F1 authorizer wired** — `rg -n "AllowAllAuthorizer"
  go/cmd/striatumd/` returns no production-launch hits. **Not run**;
  V1.5 RFC text (line 545) commits the design but the implementation
  is unverified from inputs.
- **F2 launch works** — a real `pytest tests/_harness/...` (or
  equivalent) launches the Go daemon end-to-end. **Not run**; V1.5
  RFC text describes the design.
- **F3 matrix wired** — `make test-multi-repo CORE=go` is a real
  target and passes locally. **Not run**; V1.5 RFC text describes
  the design.
- **F4 race test in-tree** — a regression test that fails on the
  un-fixed audit append exists and passes on the fixed version.
  **Not run**; V1.5 RFC text (lines 528-531) names
  `go/pkg/db/audit_race_test.go` and `tests/test_daemon_go_audit.py`.
- **F5 driver swapped** — no `exec.Command("psql", ...)` under
  `go/pkg/db/`; `go.mod` lists the chosen driver; supply-chain note
  in HANDOFF. **Not run**; V1.5 RFC text (lines 506-518) describes
  the design.
- **Tests pass** — `make test` green; `go test ./...` green under
  `go/`. **Not run**.

These are the load-bearing acceptance checks. A second-pass build
review with `access_scope = code` must run them before the V1.5
slice is merged.

## What is right (so the next round knows what not to undo)

- The V1.5 deltas section is structured F5 → F4 → F1 → F2 → F3, with
  the explicit rationale "F5 lands before F4 and F1 because those two
  correctness fixes need the parameter-binding and transaction
  support of the new driver" (RFC line 502). This is the right
  dependency order; first-time readers of the spec can follow why
  the driver swap goes first.
- The "Denial reasons line up one-for-one" commitment for F1 is the
  right ergonomics-correctness contract.
- The `RETURNING audit_id` re-wiring for F4 closes the V1 envelope-
  shape regression at the same time as the chain-fork race — one
  change, two ergonomics fixes.
- The explicit "two CI jobs" decision for F3 is operator-debuggable
  in a way an in-process parametrization is not.
- `STRIATUMD_GO_BIN` is retained as a developer override even after
  the Makefile pins `go/bin/striatumd` — that preserves the V1
  ergonomics escape hatch for contributors who built the binary
  somewhere else.
- The simple-protocol-with-bound-parameters choice in F5 lets the
  existing multi-statement migration files keep working unchanged,
  which is the right "least surprise" trade-off for operators who
  built mental models around the SQL surface.

## Verdict

`accept_with_findings`. Severity **low**.

Acceptance rationale under the ergonomics_dx posture: V1.5 closes the
four V1-blocking ergonomics defects at the design level (flag wiring,
binary path, CORE=go matrix wiring, doc-implementable flag set). The
findings above are residual surface concerns — flag naming,
welcome-field carry-over, test/CI discoverability, denial vocabulary
and lock-wait error surface, the supply-chain trade-off narrative —
that should be closed in the next review round (with
`access_scope = code` so the Required-check ripgrep verifications can
run) and/or in a follow-up doc-honesty pass through
`docs/HOW_TO_HUMAN.md` once the implementation lands.

The reason the verdict is not bare `accept` is two-fold:

1. The Required checks are unverifiable from inputs alone; the
   document evidence is consistent and well-argued but the
   implementation has not been audited under this review's scope.
2. F-DX-2 (daemon_core in welcome) and F-DX-5 (denial vocabulary)
   are small but stand-alone ergonomics carry-overs that a V1.5
   "ergonomics slice" should plausibly address before the next merge
   gate.

None of the findings block V1.5 from being the right merge slice
before Step 4. They are quality-pass items the next review iteration
or the HOW_TO_HUMAN.md update should close.
