---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0039", "v1.5", "design"]
---

author: reviewer-unknown-model-001

# RFC 0039 V1.5 Design Synthesis Review — ergonomics_dx

Target: [`docs/dogfood/047/DESIGN_SYNTHESIS.md`](../../DESIGN_SYNTHESIS.md).

Posture: **ergonomics_dx**. Acceptance means a first-time user can
discover the new affordances from `make` / docs alone, the failure
modes return surfaces a human can act on, and the Postgres-backed
authorizer's error shape is recognizably identical to the Python
daemon's. This review walks the prompt's specific-check list in order
and records what is sharp, what is fuzzy, and the three missing
operator-UX artifacts that pull this verdict off of bare `accept`.

## Specific-check pass

### F1 — PostgreSQL-backed RPC authorization (synthesis §F1)

All four named affordances are present and pinpointed, not described
abstractly:

- Authorizer file path: `go/pkg/rpc/auth_pg.go` (synthesis §F1, line
  "Add `go/pkg/rpc/auth_pg.go` with…").
- Validator interface: implements the existing
  `go/pkg/rpc/capability.go` `Authorizer` interface — so the wiring
  point in `go/cmd/striatumd/main.go` is the single line
  `&rpc.PostgresAuthorizer{Runner: pool.Runner, Clock: time.Now}`
  replacing `rpc.AllowAllAuthorizer{}`. The "fail closed if a serving
  daemon with database connectivity cannot construct the authorizer"
  rule is explicit, which is the right default for an ergonomics
  review (failure is loud, not silent).
- Denial envelope shape:
  `rpc.NewError(reason, "daemon RPC authorization failed", nil)`
  routed through the existing RFC 0030 error response in
  `Server.Handle`. The denial-reason vocabulary
  (`capability_scope_mismatch`, `capability_missing`) is named, the
  scope-mismatch follow-up query is described.
- Audit-on-deny hook: `AuthContext.ClientID`, `TokenID`,
  `RepositoryID`, `Capability`, `Decision`, `DenialReason` are listed
  by name and the synthesis correctly notes that `Server.Handle` already
  calls `AuditRecorder.RecordRPC` post-response, so denials flow through
  the F4 transactional path without a parallel codepath.

The validator details are tight: `subtle.ConstantTimeCompare`,
HMAC-SHA256 with `token_salt` as key. The "no positive/negative
cache" line is the right call for a first-time-user surface — there
is one thing to reason about, not two.

The single fuzzy edge here (folded into the operator-UX section
below): the synthesis asserts "The denial envelope shape does not
change" without quoting a side-by-side example of the JSON a client
actually receives. The intent is clear, but a fresh-context reader
cannot eyeball-verify shape parity with the Python daemon.

### F2 — Go harness launch contract (synthesis §F2)

Locked argv contract is exact, not hand-wavy:

```
--socket <path>
--postgres-url <url>
--migrations-sha-source <path>
```

with the `tests/_harness/daemon.py` invocation written out
verbatim. The synthesis correctly rejects both `--db-url` and a
revived `--migrations-dir`, choosing instead the test-facing
`--migrations-sha-source` flag — and that choice is justified
("keep the operator-facing Go CLI canonical while still failing fast
on embedded-SQL drift"), which is exactly the kind of operator-rationale
an ergonomics review wants visible.

`go/Makefile` is written out in full with `BIN := bin/striatumd`, so
the binary path the harness expects (`go/bin/striatumd`) matches the
build output. `STRIATUMD_GO_BIN` is explicitly called out as a
"trusted developer-environment override," which addresses the
documentation-nit raised in the dogfood-042 build review.

The smoke command is `make daemon-go-build && make test-multi-repo
CORE=go`. See **F-Ergo-2** below for the single ergonomics gap here.

### F3 — `make test-multi-repo CORE=go` (synthesis §F3)

Make-target shape is exact:

```make
CORE ?= python

test-multi-repo: $(VENV)/.installed
	STRIATUM_MULTI_REPO_DAEMON_CORE=$(CORE) \
	$(PYTHON) -m pytest -m multi_repo \
		tests/test_multi_repo_harness.py \
		...
		tests/test_daemon_go_smoke.py \
		tests/test_daemon_go_audit.py
```

The fixture parametrize point is named precisely (`tests/conftest.py`,
not "the conftest" or "a fixture"), and the env-var validation in the
fixture raises `pytest.UsageError` for invalid values — the right
ergonomic default for a knob a first-time user might typo.

The test-selection rule is the synthesis's strongest single sentence:
"every file already listed by `test-multi-repo` opts into the matrix
because it consumes that fixture." This means a first-time user does
not need a separate "which tests run under CORE=go?" mental model.
The skipif policy is also explicit and discoverable: any per-test
skip must cite "RFC 0039 Step 4" by name, so a reader greps one
string and sees the full set of pending mutating-route gaps.

The decision to run two explicit CI jobs (CORE=python and CORE=go)
rather than in-process pytest parametrization is justified ("preserve
local runtime while making the Go-core evidence intentional") — good
for evidence honesty, neutral for ergonomics.

### F4 — Transactional audit append (synthesis §F4)

Function signature change is exact: `RecordRPC(ctx, envelope, auth,
response) (string, error)` is preserved at the surface, with the
transactional behavior moving inside. This is the right shape for an
ergonomics review — no caller in `go/pkg/rpc/server.go` has to
change.

Isolation level is one chosen value, not a menu: `READ COMMITTED`
with row-level `SELECT ... FOR UPDATE` on the singleton
`striatumd.audit_chain_head` row. The rationale (a single-row hot
path, `SERIALIZABLE` adds retry noise without improving the
contended value) is one sentence and named — a first-time reader
can repeat it back.

Regression-test paths are exact and double-sided:
`go/pkg/db/audit_race_test.go` for Go-internal correctness and
`tests/test_daemon_go_audit.py` for the Python cross-core verifier
running against `MultiRepoHarness(daemon_core="go")`. The
"concurrent goroutines fire RPC, then verify a linear chain with no
duplicate `previous_hash` links" assertion is unambiguous.

### F5 — Pure-Go PostgreSQL driver (synthesis §F5)

One driver is chosen: `pgx/v5`. Lib/pq is not on the table. The
justification is one sentence and concrete: "current pure-Go
PostgreSQL driver with native parameter binding, connection pooling,
context-aware calls, and transaction support needed by F4." The
`go.mod` line is exact:

```go
require github.com/jackc/pgx/v5 v5.7.2
```

The first-third-party-dep callout is explicit and operationally
sharp: "This is the Go daemon's first third-party dependency, so the
build handoff must call out the new direct and indirect module hashes
as a supply-chain review point." A first-time reviewer of the build
packet will know exactly what to look for in `go.sum`.

### Implementation order (synthesis "Accepted Plan", lines 16–23)

Order is locked: **F5 → F4 + F1 → F2 + F3**. The cross-finding
rationale is one sentence: "F5 gates F4 and F1 because those fixes
need parameterized queries and one real transaction boundary; F2 and
F3 come after the daemon can authorize and audit requests correctly."

This passes the ergonomics test for a build agent reading the
synthesis as a TODO: the reader knows the first commit lands a
driver swap with no behavior change, then audit goes transactional,
then auth goes real, then the harness contract and Make target light
up the whole matrix. The dependency direction is unambiguous (F5
under F4; F5 under F1; F1+F4 under F2; F1+F4+F2 under F3) and no
ordering is left to inference.

## Operator UX — drafted error envelope examples (the gap)

The prompt's last specific check asks for drafted error envelope
examples for three failure modes. **None of the three is present in
the synthesis.** This is the load-bearing reason for
`accept_with_findings` over `accept`.

### F-Ergo-1 — Missing Go binary error surface is undrafted (low)

Synthesis §F2 establishes that `tests/_harness/daemon.py` honors
`STRIATUMD_GO_BIN` and otherwise builds the in-tree binary. It does
not draft what an operator (or pytest collector) sees when neither
is available — e.g., the Go toolchain is missing, `make -C go build`
fails, or `STRIATUMD_GO_BIN` points at a stale path.

The dogfood-042 build review (F2) showed that the harness currently
raises `RuntimeError(f"...{binary} is missing")` after `make`
returns. A V1.5 design plan owns the operator surface for "Go is not
built yet" — a first-time user invoking `make test-multi-repo
CORE=go` on a fresh checkout needs a one-line message such as:

```
make: *** [daemon-go-build] error: Go toolchain not on PATH.
Install Go 1.23+ or set STRIATUMD_GO_BIN to a prebuilt binary.
See docs/HOW_TO_HUMAN.md "Running the Go daemon".
```

The synthesis should commit to: (a) the operator-visible message
shape, (b) whether `make test-multi-repo CORE=go` shells through
`make daemon-go-build` as a prerequisite or fails fast with a
direct-Run message, and (c) whether `STRIATUMD_GO_BIN` pointing at a
missing file is a fatal harness error or a fallback to `make`.

This is the kind of gap where the synthesis is correct in scope but
silent on what a fresh operator actually sees — exactly the
ergonomics_dx surface the posture is meant to police.

### F-Ergo-2 — Postgres-token-rejected envelope is asserted, not shown (low)

Synthesis §F1: "The denial envelope shape does not change.
`rpc.RequireAllowed` returns `rpc.NewError(reason, "daemon RPC
authorization failed", nil)`, and `Server.Handle` emits the existing
RFC 0030 error response."

A first-time-user surface is exactly the kind of place where "shape
does not change" should be quoted with a JSON example, side-by-side
with the Python daemon's equivalent. The synthesis names the denial
reasons (`capability_missing`, `capability_scope_mismatch`) but does
not show the wire-level envelope. A reader cannot confirm "yes, an
existing client checking `error.code == "capability_missing"` keeps
working" without leaving the synthesis to read RFC 0030.

Suggested addition (out of scope for this review): one fenced JSON
block at the end of §F1 showing the response for a rejected request,
e.g.,

```json
{"v": 1, "id": "req_…", "error":
 {"code": "capability_missing",
  "message": "daemon RPC authorization failed"}}
```

with a one-line note "identical wire shape as Python; verify in
`tests/test_daemon_go_audit.py` by asserting against a single
`expected_error` literal." This anchors the shape-parity claim to
something a reviewer can grep for.

### F-Ergo-3 — Transaction-conflict-on-audit-append envelope is undrafted (low)

Synthesis §F4 chooses `READ COMMITTED` + `FOR UPDATE` on a singleton
row. The named consequence is "concurrent goroutines serialize on
that row and the linear chain holds." The unnamed consequence — what
an operator sees if `statement_timeout` (set by §F5) trips while a
goroutine is blocked on the `FOR UPDATE` waiter — is the third
drafted-envelope gap.

Two plausible shapes, neither chosen by the synthesis:

- The blocked transaction times out → pgx surfaces a
  `context.DeadlineExceeded` (or `lock_not_available` if a `NOWAIT`
  is added) → `RecordRPC` returns an error → `Server.Handle` ends
  the request with what error code?
- The blocked transaction waits past the RPC's own deadline → client
  sees a connection close, no envelope.

For ergonomics, the operator-visible surface should be a single,
named envelope shape — e.g.

```json
{"v": 1, "id": "req_…", "error":
 {"code": "audit_append_conflict",
  "message": "audit chain head contended; retry"}}
```

— and the synthesis should commit to "RPC clients see exactly this
code on contention; retries are the client's responsibility." Without
that commitment, two build agents could implement different
fallbacks and the cross-core verifier would not catch it.

## Discoverability checks (the prompt's three opening questions)

### F-Ergo-4 — `make test-multi-repo CORE=go` is not discoverable from `make help` (low)

The prompt explicitly asks: "Is `make test-multi-repo CORE=go`
discoverable from `make help` or `make`?" The synthesis adds the
target body in the top-level `Makefile` but does not show a
corresponding line in `make help`. A first-time user typing `make`
or `make help` on the fresh checkout will see `test-multi-repo` (if
already listed) but not the `CORE=` parameterization, and the
default (`CORE=python`) will hide that there is a Go path at all.

Recommendation (out of scope): a one-line `help` entry such as:

```
  test-multi-repo CORE=python|go   Run multi-repo e2e tests against the chosen daemon core (default: python)
```

This is the cheapest single ergonomics win in the V1.5 surface — a
build packet implementing F3 should be told to land it.

### F-Ergo-5 — `make daemon-go-build` is referenced but not defined in the synthesis (low)

Synthesis §F2 names the smoke command as `make daemon-go-build &&
make test-multi-repo CORE=go`. The `go/Makefile` body is shown in
full. The top-level `Makefile`'s `daemon-go-build` target is not.

This is small but real for a first-time user: the operator runs the
two-command incantation, hits "make: *** No rule to make target
`daemon-go-build`", and is now in detective mode. Either (a) the
synthesis should show the top-level Makefile addition alongside the
test-multi-repo target, or (b) the synthesis should say "the
existing top-level target is preserved." Right now the reader has
to grep the tree to know which.

This is the same class of doc-honesty gap the dogfood-042 build
review F7 flagged for `HOW_TO_HUMAN.md`. The cure is to keep
synthesis self-contained — every command in the smoke instructions
has its defining file shown.

### F-Ergo-6 — Postgres-backed authorizer error shape claim is one assertion, not a verifier (informational)

The synthesis asserts "Token validation matches
`src/striatum/daemon_rpc/capability.py`" without naming a specific
parity test that asserts byte-for-byte envelope equality. The
dogfood-042 audit-chain finding F10 showed that "matches" claims
without a fixture become silent divergences. F-Ergo-2's recommended
JSON-block addition would also be the natural place to name a
small `tests/test_daemon_go_authorizer_parity.py` (or add an
assertion to `tests/test_daemon_go_audit.py`) that loads a known
denial response and asserts shape equality with a fixture also
consumed by the Python authorizer tests. Not a blocker for V1.5;
worth recording so the build packet can scope it explicitly.

## What is right (so the build packet knows what not to undo)

- F1, F4, F5 each have all the prompt's required-named affordances
  in one synthesis section, with rationale that survives quoting out
  of context.
- The "pgx/v5" choice resolves the F5 driver ambiguity the
  dogfood-042 build round left open, and the `go.mod` line is
  copy-pasteable.
- The `--migrations-sha-source` flag is a strict improvement over
  reviving `--migrations-dir`: it preserves operator CLI shape, it
  fails-fast on embedded-SQL drift, and it is test-facing so the
  production daemon's command line stays short.
- The implementation order is one sentence and load-bearing — F5
  before F4/F1, F2/F3 after the daemon can authorize and audit. A
  build agent reading the synthesis as a sequenced TODO will commit
  in the right order without having to derive it.
- The `pytest.UsageError` raise on invalid `CORE` value is a small
  but real ergonomics win — typos die loudly.
- The skipif-must-cite-"RFC 0039 Step 4"-by-name rule means a future
  contributor can `grep -r "RFC 0039 Step 4"` to find every pending
  Go-mutating-route gap in a single pass.

## Verdict

`accept_with_findings`. Severity **low**.

The synthesis is sharp on the surface affordances the prompt names —
F1's authorizer module, F2's locked argv and Makefile, F3's `CORE`
parameter, F4's transaction shape, F5's driver and `go.mod` line are
each pinpointed, not described abstractly. The implementation order
is locked with one-sentence cross-finding rationale that survives
quoting. A build agent can read the synthesis and start typing
without leaving it.

The verdict is not `accept` because the prompt explicitly asks for
drafted error envelope examples for three named failure modes, and
the synthesis ships none. The three envelopes (missing Go binary,
Postgres token rejected, transaction conflict on audit append) are
the operator-visible surface the ergonomics_dx posture exists to
police — when "the shape does not change" or "concurrent requests
serialize" is asserted without a quoted JSON block, a first-time
operator cannot eyeball-verify the claim, two build agents can
disagree on the runtime surface, and the cross-core verifier has
nothing to grep for. F-Ergo-1, F-Ergo-2, F-Ergo-3 fold this
literally back into the synthesis as three short fenced blocks plus
one short commitment per block.

Two smaller discoverability gaps (F-Ergo-4, F-Ergo-5) round out the
findings: `make test-multi-repo CORE=go` is not visible from `make
help`, and the smoke command's `make daemon-go-build` prerequisite
is referenced but the top-level `Makefile` addition is not shown.
Both are one-line fixes in the synthesis text itself.

None of the findings invalidate the V1.5 plan or alter the
implementation order. They are write-once additions to the synthesis
that the build packet can carry through without rework. Accept now,
land the three envelopes plus the two discoverability lines in the
next revision of the synthesis (or carry them into the build packet's
acceptance checklist), and the ergonomics_dx surface lands with the
rest of the V1.5 slice.
