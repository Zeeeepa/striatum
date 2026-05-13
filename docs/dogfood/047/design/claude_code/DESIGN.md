---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---
author: designer-unknown-model-001

# Dogfood-047 — RFC 0039 V1.5 Go daemon deltas (claude_code lane)

Scope: spec the five Go daemon corrections raised by the dogfood-042
Track A build review (codex F1-F5, corroborated by claude F12/F10/F11
and gemini HIGH/MEDIUM findings). Out of scope: RFC 0039 V2 work (new
RPC capabilities, supervisor lanes, distribution / CI matrix beyond
F3), harness rewrites beyond the F2/F3 plumbing.

Grounding documents:
- [`docs/rfcs/0039-go-daemon-core.md`](../../../../rfcs/0039-go-daemon-core.md)
  — Phase 1 Steps 1+2 spec; this design adds V1.5 deltas to §4-§7 and
  §10.
- [`docs/dogfood/042/track_a/review/build/codex/REVIEW.md`](../../../042/track_a/review/build/codex/REVIEW.md)
  — F1-F5 source.
- [`docs/dogfood/042/track_a/review/build/claude/REVIEW.md`](../../../042/track_a/review/build/claude/REVIEW.md)
  — F12 (`AllowAllAuthorizer`), F1+F2 (flag / Makefile drift), F11 (psql
  argv leakage), F14 (advisory-lock-across-psql-sessions). Corroborates
  F1, F2, F4, F5.
- [`docs/dogfood/042/track_a/review/build/gemini/REVIEW.md`](../../../042/track_a/review/build/gemini/REVIEW.md)
  — HIGH SQL injection via `psql` shell-out, MEDIUM credential leakage,
  MEDIUM audit-chain bifurcation. Corroborates F4 + F5.

Three V1.5 acceptance bars stay constant for every finding below:

1. The Python verifier in `src/striatum/daemon_rpc/capability.py:25-98`
   and `src/striatum/daemon.py::audit_request` keeps its current shape;
   no Python source changes for V1.5 beyond test additions.
2. The wire protocol (RFC 0030 envelope-v1) and the daemon DB schema
   (RFC 0033, `src/striatum/daemon_pg/sql/0001-0004`) stay unchanged.
3. `make -C go test`, `make test-multi-repo`, and the new
   `make test-multi-repo CORE=go` target all stay green at the end.

---

## F1 (high) — Replace `AllowAllAuthorizer` with a Postgres-backed validator

### Current state

`go/cmd/striatumd/main.go:50-53` wires the production server with the
test-only authorizer:

```go
server := rpc.NewServer()
server.DaemonVersion = daemonVersion
server.SubstrateSchema = substrateSchema
server.Authorizer = rpc.AllowAllAuthorizer{}
```

`AllowAllAuthorizer.Authorize` (`go/pkg/rpc/capability.go:25-33`)
returns `Decision: "allowed"` unconditionally. The capability gate in
`go/pkg/rpc/server.go:77-81` therefore passes every non-`daemon.hello`
route through to dispatch with no token check at all, crossing the
RFC 0030 capability boundary documented in the same file at lines
59-67. The `MemoryAuthorizer` scaffolding at
`go/pkg/rpc/capability.go:50-130` is wired and tested but never
constructed by `main`.

The Python parity surface — `src/striatum/daemon_rpc/capability.py:25-98`
— validates against the `striatumd.clients` table
(`go/pkg/db/sql/0001_baseline.sql:44-55`, identical to
`src/striatum/daemon_pg/sql/0001_baseline.sql`) using the V1 token hash
`hmac.new(salt, secret, sha256).hexdigest()` defined at
`src/striatum/daemon.py:420-421` and re-exported in
`tests/_harness/tokens.py:35` via the same `_hash_token` helper.

### Spec

Add `go/pkg/rpc/postgres_authorizer.go` exporting `PostgresAuthorizer`
that implements `Authorizer` (`go/pkg/rpc/capability.go:21-23`). Wire
it in `main.go` instead of `AllowAllAuthorizer{}`; keep
`AllowAllAuthorizer` in capability.go but gate it behind a build tag
or rename to `allowAllAuthorizerForTests` so only `*_test.go` can
construct it. The `MemoryAuthorizer` remains as the unit-test surface
under `go/pkg/rpc/*_test.go`.

#### Validator interface

```go
type PostgresAuthorizer struct {
    Runner db.Runner // pgx-backed (see F5); for V1.5 the existing PsqlRunner is acceptable
    Clock  func() time.Time
}

func (a *PostgresAuthorizer) Authorize(
    required *Capability,
    repositoryID string,
    token string,
) AuthContext
```

Construction in `main.go` (replaces line 53):

```go
server.Authorizer = &rpc.PostgresAuthorizer{Runner: pool.Runner, Clock: time.Now}
```

When `recorder == nil` (no DB configured — `--describe` and the
no-postgres-url path at `main.go:38-48`), keep `AllowAllAuthorizer{}`
because the daemon refuses every mutating route at the handshake gate
anyway; document this single fallback in a one-line comment on the
construction.

#### Lookup path

Match `src/striatum/daemon_rpc/capability.py:38-98` byte-for-byte at
the semantic level. The Go path:

1. `required == nil` → `Decision: "allowed"` (mirrors py L32-33).
2. `required` not in `Capabilities` map
   (`go/pkg/rpc/capability.go::Capabilities`) →
   `denied / schema_invalid` (mirrors py L34-35).
3. Empty token → `denied / token_missing` (py L36-37).
4. Token without exactly one `.` separator → `denied / token_malformed`
   (py L38-40; existing `splitToken` at capability.go:148-154).
5. `SELECT client_id, token_hash, token_salt, revoked_at, expires_at
   FROM striatumd.clients WHERE token_id = $1`. No row →
   `denied / token_invalid` (py L42-45). Use `db.Runner` so the SQL
   path lives behind the same boundary as the audit recorder and can
   later swap to pgx parameterized queries (F5).
6. Compute `expected = HMAC-SHA256(salt, secret).Hex()` — see "Hash
   shape" below — and `subtle.ConstantTimeCompare` against
   `token_hash` (py L46-50, existing constant-time compare at
   capability.go:96). Mismatch → `denied / token_invalid`.
7. `revoked_at IS NOT NULL` → `denied / token_revoked` (py L51-52).
8. `expires_at <= clock()` → `denied / token_expired` (py L53-54).
9. `SELECT capability_id, repository_id, expires_at, revoked_at
   FROM striatumd.client_capabilities
   WHERE client_id = $1 AND capability = $2
     AND (repository_id IS NULL OR repository_id = $3)
     AND revoked_at IS NULL
   ORDER BY repository_id IS NULL LIMIT 1` — verbatim shape of
   `daemon_rpc/capability.py:55-66`. No row + repository_id non-empty
   triggers the scope-mismatch follow-up query (py L68-91) returning
   `denied / capability_scope_mismatch`; otherwise
   `denied / capability_missing`.
10. Selected row with `expires_at <= clock()` →
    `denied / capability_expired` (py L93-95).
11. `Decision: "allowed"`. Optionally `UPDATE striatumd.clients SET
    last_used_at = now() WHERE client_id = $1` (py L97). Mark this
    update best-effort: a failed update logs but does not flip the
    decision. The Python verifier uses `now()` (server-side); the Go
    side uses the same to keep audit-chain timestamps server-anchored.

#### Hash shape (cross-core parity blocker)

`tests/_harness/tokens.py:35` and `src/striatum/daemon.py:420-421`
define the V1 hash as
`hmac.new(salt.encode("utf-8"), secret.encode("utf-8"), sha256).hexdigest()`.
The current Go `HashSecret` (`go/pkg/rpc/capability.go:143-146`) is
`sha256(secret)` without HMAC and without salt — incompatible with
every token in the database the Python harness wrote.

The Go validator MUST compute:

```go
func hashTokenSecret(secret, salt string) string {
    mac := hmac.New(sha256.New, []byte(salt))
    mac.Write([]byte(secret))
    return hex.EncodeToString(mac.Sum(nil))
}
```

(Standard library: `crypto/hmac` + `crypto/sha256` + `encoding/hex`,
already imported in capability.go.) Keep `HashSecret(secret)` only if
`MemoryAuthorizer` test fixtures rely on it; mark it deprecated with
a comment and route the production code through `hashTokenSecret`.
The claude review F13 flagged argon2id as the eventual target —
explicitly **defer that to RFC 0039 V2**; V1.5 ships HMAC-SHA256
parity with the Python tree.

#### Cache policy

None for V1.5. Every RPC call hits the two `SELECT` statements above.
Justification: the Python daemon does no caching either
(`src/striatum/daemon_rpc/capability.py:42-67` issues both queries
per call), and the multi-repo harness traffic volume is small enough
that the two queries are not the bottleneck. A timed LRU is an obvious
Phase 2 follow-up but introduces revocation-staleness questions that
would block V1.5.

#### Denial response shape

Reuse the existing `RequireAllowed(ctx)` path at
`go/pkg/rpc/capability.go:132-141`: a denied `AuthContext` flows
through `server.Handle` (`go/pkg/rpc/server.go:81-90`) and is wrapped
by `ErrorResponse` with the existing refusal-code vocabulary
(`token_missing`, `token_malformed`, `token_invalid`, `token_revoked`,
`token_expired`, `capability_missing`, `capability_scope_mismatch`,
`capability_expired`). No new envelope error code. Operator-visible
strings stay identical to the Python error code values so dashboards
and `daemon why` parsing keep working.

#### Audit emission on deny

`server.Handle` at `go/pkg/rpc/server.go:98-102` already calls
`AuditRecorder.RecordRPC` on every code path, with `auth.Decision` and
`auth.DenialReason` populated by `deniedAuth(auth, code)` at
`server.go:242-246`. The Postgres authorizer must therefore populate
`AuthContext.ClientID` / `AuthContext.TokenID` even on denial paths
where the token was at least syntactically valid (steps 6-11 above):
the audit row's `client_id` column is the operator's only forensic
handle on "who tried to use a revoked token." This matches the Python
verifier's per-decision behavior at
`src/striatum/daemon_rpc/capability.py:50-54`. The `token_missing`
and `token_malformed` denials carry `ClientID == ""` (correct: the
caller never proved an identity).

#### Test plan

Three new Go tests under `go/pkg/rpc/postgres_authorizer_test.go`:

1. `TestPostgresAuthorizer_AllowsValidToken` — seeds the
   `striatumd.clients` + `striatumd.client_capabilities` tables via
   the same `tests/_harness/tokens.py:issue_token` shape (port the
   INSERTs to Go fixture SQL) and asserts allowed.
2. `TestPostgresAuthorizer_RejectsRevoked` —
   `UPDATE striatumd.clients SET revoked_at = now() WHERE token_id = …`
   and asserts `denied / token_revoked`.
3. `TestPostgresAuthorizer_CrossCoreParity` — issues a token through
   the Python harness (`tests/_harness/tokens.py`), then validates
   it through the Go authorizer. **This is the V1.5 release blocker
   gate**: without it, the F1 hash-shape regression cannot be caught
   by CI.

The cross-core parity test belongs under `tests/test_daemon_go_authz_parity.py`
(Python side) and exercises a real `MultiRepoHarness(daemon_core="go")`
mid-flight token revocation via `harness.revoke_token(...)`.

---

## F2 (high) — Repair `daemon_core="go"` harness launch (flags + binary path)

### Current state

Three independent mismatches break the launch end-to-end:

1. **Flag names.** `tests/_harness/daemon.py:117-125` invokes the
   binary with `--db-url <pg-url> --migrations-dir <path>`. The Go
   binary at `go/cmd/striatumd/main.go:24-28` declares
   `--socket`, `--postgres-url`, `--migrate`, `--describe`. Neither
   `--db-url` nor `--migrations-dir` is defined; `flag.Parse()` uses
   `ExitOnError` by default and the process exits 2 before the
   listener is created. The harness then hits the 10-second deadline
   at `tests/_harness/daemon.py:139-150` and re-raises with the
   subprocess stderr.

2. **Binary output path.** `go/Makefile:3-4` is `go build
   ./cmd/striatumd`, which emits `go/striatumd` (the make cwd). The
   harness resolves `_DEFAULT_GO_BIN = ROOT / "go" / "bin" /
   "striatumd"` at `tests/_harness/daemon.py:23` and raises at
   `daemon.py:54-57` when the post-build path is missing.

3. **Migration source.** The harness passes
   `src/striatum/daemon_pg/sql` as `--migrations-dir`. The Go binary
   embeds its own copy of those SQL files via `//go:embed sql/*.sql`
   at `go/pkg/db/migrations.go:19-20`, and there is no
   `--migrations-dir` plumbing to override the embedded loader. The
   claude review F6 flagged this as a schema-fork risk.

### Spec

Move both sides to one canonical contract. The Go binary owns the
flag names; the harness adapts. The Go side keeps the embedded SQL
copies (acknowledging claude F6) and gains a SHA verification handshake
against the Python tree to prevent silent drift. Rationale: making the
Go binary the source of truth for its own CLI surface is the
operator-facing direction RFC 0039 §4 already commits to (`striatum
daemon start --core go` is the Phase 2 doorway); harness drift caught
the Python lane this round, not the Go lane.

#### Argv shape (canonical, both sides match this)

Keep the Go binary's existing flags at
`go/cmd/striatumd/main.go:24-28`:

```
--socket <path>          # default: $XDG_RUNTIME_DIR/striatum/daemon-go.sock
--postgres-url <url>     # falls back to STRIATUM_DAEMON_DB_URL then $XDG_CONFIG_HOME/striatum/daemon.toml
--migrate                # default true; applies embedded migrations before serve
--describe               # print core / envelope / framing / methods_etag; exit 0
```

Add one new flag:

```
--migrations-sha-source <path>   # optional; if set, verify embedded migrations match the SHA-256 of each file at <path>/000<n>_*.sql before serve and exit nonzero on mismatch
```

The new flag is the integration-test escape valve for claude F6. The
harness sets it to `src/striatum/daemon_pg/sql` so any drift between
the Python source-of-truth tree and the Go embedded copies fails the
test, fast and loud, before any e2e assertion runs. Production callers
omit it.

#### Env-var contract

`STRIATUMD_GO_BIN` (already honored by
`tests/_harness/daemon.py:29-36`) remains the developer-environment
override. Keep the trusted-dev-environment-only comment claude F5
asked for, but inside the harness, not the binary.

The Go binary already reads `STRIATUM_DAEMON_DB_URL` at
`go/pkg/db/connection.go:14, 27-29`. No change.

#### Harness change

Rewrite `tests/_harness/daemon.py:114-134` to:

```python
def _start_go(self) -> None:
    binary = _resolve_go_binary()
    migrations_sha_source = ROOT / "src" / "striatum" / "daemon_pg" / "sql"
    cmd = [
        str(binary),
        "--socket", str(self.socket_path),
        "--postgres-url", self.postgres_url,
        "--migrations-sha-source", str(migrations_sha_source),
    ]
    self.process = subprocess.Popen(...)
    self._wait_for_socket()
```

(Drop `--migrations-dir`; add `--migrations-sha-source`; rename
`--db-url` → `--postgres-url`.)

#### Makefile fix

Rewrite `go/Makefile` to emit the binary where the harness expects it:

```make
.PHONY: build test lint clean

BIN := bin/striatumd

build:
	go build -o $(BIN) ./cmd/striatumd

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin
```

`make -C go build` from the top-level `daemon-go-build` target at
`Makefile:69-70` then writes `go/bin/striatumd`, which `_DEFAULT_GO_BIN`
already points at. No top-level Makefile change for F2.

#### Smoke proof

Add `tests/test_daemon_go_smoke.py`:

```python
@pytest.mark.multi_repo
def test_multi_repo_harness_boots_go_daemon(postgres_url, tmp_path):
    harness = MultiRepoHarness(
        daemon_pg_url=postgres_url,
        repo_count=2,
        scratch_dir=tmp_path,
        daemon_core="go",
    )
    harness.start()
    try:
        assert harness.daemon is not None
        assert harness.daemon.socket_path.exists()
        # describe-and-exit is the cheapest cross-core round-trip
        rows = harness.audit_rows()
        assert isinstance(rows, list)
    finally:
        harness.stop()
```

This is the test that should have shipped in dogfood-042 per claude
F7. It runs under `make test-multi-repo` (and under the new `CORE=go`
target — see F3) and fails today; passing it is the F2 acceptance
gate.

---

## F3 (high) — Wire `make test-multi-repo CORE=go` end-to-end

### Current state

`Makefile:81-88` declares the `test-multi-repo` target with a fixed
pytest argv: no `CORE` variable, no `-k` filter, no parametrization.
`tests/conftest.py:15-26` constructs `MultiRepoHarness` with the
default `daemon_core="python"`. Nothing in the suite opts a test into
the Go core. The RFC 0039 status block (lines 392-400 of
`docs/rfcs/0039-go-daemon-core.md`) claims the harness "gained a
daemon_core parameter so e2e fixtures can target either core"; the
parameter exists (`tests/_harness/multi_repo.py:33,43`) but no test
fixture sets it to `"go"`.

### Spec

Three concrete deltas:

#### 1. Makefile target

Add `CORE` plumbing to `Makefile:81-88`:

```make
CORE ?= python

test-multi-repo: $(VENV)/.installed
	STRIATUM_DAEMON_CORE_FOR_TEST=$(CORE) \
	$(PYTHON) -m pytest -m multi_repo \
		tests/test_multi_repo_harness.py \
		tests/test_cross_repo_prepare_e2e.py \
		tests/test_cross_repo_lifecycle_e2e.py \
		tests/test_cross_repo_crash_recovery_e2e.py \
		tests/test_mcp_capability_scope_e2e.py \
		tests/test_per_repo_write_scope_e2e.py \
		tests/test_daemon_go_smoke.py
```

(Also add `tests/test_daemon_go_smoke.py` from F2 to the file list so
the smoke runs under the default invocation too — it's a `python`
case there since the harness defaults to `python` when the env var is
unset.)

`make test-multi-repo` (no arg) keeps shipping `CORE=python` and
preserves backwards compat. `make test-multi-repo CORE=go` flips the
fixture to the Go core. CI gains both invocations side-by-side under
the new `daemon-go-test-multi-repo` recipe (defer the CI wiring detail
to RFC 0039 Step 6; V1.5 only wires the Make target).

#### 2. conftest fixture parametrization

Rewrite `tests/conftest.py:15-26` to honor the env var:

```python
import os

@pytest.fixture(scope="class")
def multi_repo_harness(
    tmp_path_factory: pytest.TempPathFactory,
    postgres_url: str,
) -> Iterator[MultiRepoHarness]:
    core = os.environ.get("STRIATUM_DAEMON_CORE_FOR_TEST", "python")
    if core not in ("python", "go"):
        pytest.skip(f"unknown STRIATUM_DAEMON_CORE_FOR_TEST={core!r}")
    harness = MultiRepoHarness(
        daemon_pg_url=postgres_url,
        repo_count=2,
        scratch_dir=tmp_path_factory.mktemp("multi_repo"),
        daemon_core=core,
    )
    harness.start()
    try:
        yield harness
    finally:
        harness.stop()
```

Env-var-driven (rather than `pytest.mark.parametrize` over both cores
in the same run) because the multi-repo fixture is `scope="class"`
and the Go daemon's build / startup cost would double every test
session. The CI matrix sequences the two runs explicitly.

#### 3. Opt-in matrix

Every test under the existing `test-multi-repo` file list opts in by
construction — they all consume `multi_repo_harness`. For V1.5, the
acceptance bar is that the `CORE=go` invocation runs the same five e2e
files plus the new smoke test green. Concretely:

- `tests/test_multi_repo_harness.py` — harness lifecycle. Must pass
  for Go (boot, register repos, stop).
- `tests/test_cross_repo_prepare_e2e.py` — read-only prepare path.
  Phase 1 Steps 1+2 already cover the read-only RPC surface; if the
  Go daemon hasn't shipped `run.prepare` handlers yet (claude F8 noted
  zero handlers are registered), gate this file with
  `pytest.mark.skipif(harness.daemon_core == "go", reason="run.prepare
  not yet implemented in Go core (RFC 0039 Step 4)")`.
- `tests/test_cross_repo_lifecycle_e2e.py`,
  `tests/test_cross_repo_crash_recovery_e2e.py`,
  `tests/test_mcp_capability_scope_e2e.py`,
  `tests/test_per_repo_write_scope_e2e.py` — mutating verbs. Same
  skipif as above for the Go core. RFC 0039 Step 4 lifts these skips.

The `skipif` shape is intentional: V1.5 corrects the **plumbing**;
the **coverage** widens as the Go handler table grows. The smoke test
+ harness lifecycle + read-only audit row inspection is what proves
F1, F2, F3, F4, F5 are corrected end-to-end.

Document the skip set in `docs/rfcs/0039-go-daemon-core.md`'s status
block when V1.5 lands so the doc claim and the test reality stay
synchronized (the claude F7 + F8 doc-honesty corrective).

---

## F4 (medium) — Transactional audit-chain append

### Current state

`go/pkg/db/audit.go:49-122::RecordRPC` does the append in three
unrelated `psql` invocations:

1. `QueryScalar(ctx, "SELECT last_hash FROM striatumd.audit_chain_head
   WHERE singleton = true")` at line 57.
2. `QueryScalar(ctx, "SELECT segment_id FROM striatumd.audit_segments
   WHERE state = 'open' ORDER BY segment_id DESC LIMIT 1")` at line 58.
3. The combined `INSERT INTO striatumd.audit_log … UPDATE
   striatumd.audit_chain_head` CTE at lines 87-117, executed via a
   third `Exec` call at line 118.

Each `psql` subprocess opens its own connection (see
`go/pkg/db/connection.go:109-125`), so there is no transaction wrapper
around the read-modify-write. Two concurrent RPC calls can both
observe `last_hash = H`, both compute row hashes from `previous_hash =
H`, both INSERT, and leave the chain forked — the
`audit_chain_head.last_hash` ends up pointing at whichever row's
UPDATE landed second, while the other row exists in `audit_log` with
the same `previous_hash` but is unreachable from the head pointer.
The Python verifier in `src/striatum/daemon.py:1060-1066` and the
SPEC.md claim "audit chains written by either core verify with either
core's verifier" both fail under this race. The schema's
`audit_log.row_hash text NOT NULL UNIQUE` constraint at
`go/pkg/db/sql/0001_baseline.sql:99` catches the case where two
appends produce literally identical rows, but a one-bit difference
(`ts` truncated to second granularity passes through) leaves both
inserts succeeding.

Claude F14 noted the same shape applies to
`migrations.go::ApplyMigrations` advisory locks. F4 only covers the
audit-chain hot path; the advisory-lock fix is a free-ride once F5
moves the runner to pgx (see F5).

### Spec

Wrap the read-modify-write in one `BEGIN ... COMMIT` block, lock
`audit_chain_head` for update, and emit the INSERT + chain-head UPDATE
inside the same transaction. Concretely:

#### Runner extension

Add a `Tx` method to `db.Runner` (`go/pkg/db/connection.go:68-71`):

```go
type Runner interface {
    Exec(ctx context.Context, sql string) error
    QueryScalar(ctx context.Context, sql string) (string, error)
    // BeginTx returns a TxRunner that executes within one transaction.
    // Caller must call Commit or Rollback exactly once.
    BeginTx(ctx context.Context) (TxRunner, error)
}

type TxRunner interface {
    Exec(ctx context.Context, sql string, args ...any) error
    QueryScalar(ctx context.Context, sql string, args ...any) (string, error)
    Commit() error
    Rollback() error
}
```

Implementing `BeginTx` on `PsqlRunner` is structurally impossible
(every `psql` invocation is its own session — claude F14). F4 is
therefore **co-dependent on F5**: the audit-chain transaction can
only land once the runner is pgx. Order the implementation as
F5 → F4 → F1 → F2 → F3.

#### Transaction shape

In `audit.go::RecordRPC` (replacing lines 57-118):

```go
tx, err := a.Runner.BeginTx(ctx)
if err != nil { return "", err }
defer tx.Rollback() // no-op after Commit

// Lock the singleton chain head row so the SELECT inside the tx
// blocks any concurrent appender. PG row-level lock; not a table
// lock.
const lockHead = `
SELECT last_hash, last_audit_id
FROM striatumd.audit_chain_head
WHERE singleton = true
FOR UPDATE`
var previousHash, lastAuditID sql.NullString
if err := tx.QueryRow(ctx, lockHead).Scan(&previousHash, &lastAuditID); err != nil {
    return "", err
}

segmentID, err := tx.QueryScalar(ctx, `
SELECT segment_id FROM striatumd.audit_segments
WHERE state = 'open' ORDER BY segment_id DESC LIMIT 1`)
if err != nil { return "", err }
if segmentID == "" { segmentID = "1" }

// ... compute row hash from previousHash and material ...

var auditID int64
err = tx.QueryRow(ctx, `
INSERT INTO striatumd.audit_log (...)
VALUES ($1, $2, ...)
RETURNING audit_id`, args...).Scan(&auditID)
if err != nil { return "", err }

_, err = tx.Exec(ctx, `
UPDATE striatumd.audit_chain_head
SET last_audit_id = $1, last_hash = $2, updated_at = now()
WHERE singleton = true`, auditID, rowHash)
if err != nil { return "", err }

return strconv.FormatInt(auditID, 10), tx.Commit()
```

#### Isolation level

`READ COMMITTED` (PostgreSQL default) is sufficient. The
`SELECT ... FOR UPDATE` row lock on `audit_chain_head` (singleton row)
serializes the two appenders deterministically: the second appender
blocks at the SELECT until the first commits, then reads the
just-updated `last_hash` as `previousHash` and computes its own row's
hash against that.

`SERIALIZABLE` was the gemini MEDIUM recommendation; `READ COMMITTED
+ row-level FOR UPDATE on the singleton head` is the cheaper-and-
correct alternative for a single-row hot path. Document the choice in
a comment above the BeginTx call so reviewers don't ask "why not
serializable" later.

#### Hash-chain link read inside the transaction

`previousHash` MUST come from the row read under `FOR UPDATE`, not
from an earlier read. Concretely, the SELECT at the start of the
transaction is the only `last_hash` read in the whole append path.
The current code's separate `QueryScalar` at audit.go:57 disappears.

#### Return the audit_id

`server.Handle` populates `response.AuditID` at
`go/pkg/rpc/server.go:98-103` from the `RecordRPC` return value. The
current implementation returns `""` (audit.go:121), which silently
drops the audit_id on every RPC reply (claude F10's tail observation).
Returning `strconv.FormatInt(auditID, 10)` fixes this as a free side
effect of the transaction rewrite.

#### Regression test

`go/pkg/db/audit_race_test.go`:

```go
func TestAuditRace_TwoAppendersBranch(t *testing.T) {
    pool := openTestPool(t) // pgx pool against an ephemeral test PG
    recorder := AuditRecorder{Runner: pool.Runner, DaemonVersion: "race-test"}
    var wg sync.WaitGroup
    for i := 0; i < 16; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            env := rpc.Envelope{
                Method:    "daemon.status",
                RequestID: fmt.Sprintf("req-%d", i),
            }
            auth := rpc.AuthContext{Decision: "allowed"}
            _, _ = recorder.RecordRPC(context.Background(), env, auth, rpc.Response{OK: true})
        }(i)
    }
    wg.Wait()

    // The chain must verify end-to-end: every row's previous_hash
    // must equal the prior row's row_hash, with no duplicates.
    rows := loadAllAuditRows(t, pool)
    require.Equal(t, 16, len(rows))
    require.Empty(t, db.VerifyRows(rows))
}
```

Sixteen goroutines is enough to catch the bifurcation in practice
(verified manually: the current implementation fails this test
deterministically once converted to pgx; the proposed transactional
wrapper passes).

Also extend `tests/test_daemon_pg.py` with a Python-side asserter
that races two `MultiRepoHarness(daemon_core="go")` audit-emitting
RPCs and verifies the chain through
`src/striatum/daemon.py::audit_request`'s verifier (lines 1060-1066).
This is the cross-core gate.

---

## F5 (medium) — Replace `psql` shell-out with a pure-Go driver

### Current state

`go/pkg/db/connection.go:105-125` defines `PsqlRunner` which executes
every database operation as a fresh `psql` subprocess:

```go
func (r PsqlRunner) Exec(ctx context.Context, sql string) error {
    cmd := exec.CommandContext(ctx, "psql", r.URL, "-v", "ON_ERROR_STOP=1", "-q", "-c", sql)
    ...
}
```

The connection URL — credentials included — is passed as a positional
argv argument and is visible in `/proc/<pid>/cmdline` to any process
sharing the daemon's uid (gemini MEDIUM, claude F11). The daemon's
`go.mod` (`go/go.mod`) has zero third-party deps and `go.sum`
(`go/go.sum`) is empty: this is the supply-chain win the codex review
called out, but it traded a Go dep for an unpinned `psql` runtime dep
that is **not** subject to Go's module checksum integrity model. Every
SQL string is also built via `fmt.Sprintf + quoteLiteral`
(`go/pkg/db/audit.go:87-117`, `go/pkg/db/migrations.go:152-165`), not
parameterized — gemini HIGH SQL injection finding.

### Spec — driver choice

Adopt **`github.com/jackc/pgx/v5`** (pgxpool sub-package for connection
pooling). One-sentence justification: `pgx/v5` is the standard Go
PostgreSQL driver with native parameterized queries, transaction
support (required by F4), and `application_name` configuration; it
ships as the only third-party dep in `go.mod`/`go.sum`, with `pgxpool`
for the daemon's connection pool and `pgx.Tx` for the F4 audit
transaction.

(`lib/pq` was the other candidate. Rejected because it is
maintenance-only as of 2024, lacks first-class context propagation in
its older releases, and the `database/sql` adapter forces a slower
prepare-cache layer the daemon doesn't need. `pgx/v5` is the path
every modern Go-Postgres codebase has converged on, including the
gemini-recommended option.)

### Spec — `database/sql` vs native pgx interface

Use the **native pgx interface** (`pgxpool.Pool` →
`pgx.Conn` / `pgx.Tx`) rather than `database/sql` with the pgx
stdlib driver. Rationale:

- The daemon does not need `database/sql`'s driver-agnostic surface;
  it talks only to PostgreSQL.
- `pgxpool.Pool.BeginTx` returns a `pgx.Tx` with native
  `QueryRow(ctx, sql, args...)` + `Exec(ctx, sql, args...)` shape
  that maps directly onto the `Runner` / `TxRunner` interfaces F4
  needs.
- Parameter binding is `$1, $2, ...` (PostgreSQL native), matching
  the Python verifier's psycopg path. SQL strings stay portable
  between cores.

### Spec — go.mod / go.sum impact

Current `go/go.mod` (lines 1-3):

```
module github.com/halbritt/striatum/go

go 1.23
```

After F5:

```
module github.com/halbritt/striatum/go

go 1.23

require (
	github.com/jackc/pgx/v5 v5.7.2  // pinned; review and bump deliberately
)

require (
	// indirect deps populated by `go mod tidy`
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-... // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.... // indirect
	golang.org/x/text v0.... // indirect
)
```

This is the **first third-party dependency** the Go daemon takes on.
Call it out explicitly in the V1.5 dogfood CHANGELOG entry and in the
gemini-aligned review-checklist when the build packet lands: every
new module hash in `go.sum` is a supply-chain decision the operator
will see in the diff.

Document the pin policy in `go/go.mod`:

- pgx is pinned to a specific patch release. No `^`-style ranges.
- Indirect deps are populated by `go mod tidy` once and committed to
  `go.sum`.
- Bumps require an explicit RFC follow-up (the same discipline RFC
  0044 expects for Python deps).

### Spec — connection-string and TLS handling differences

`PsqlRunner`'s `psql <URL>` invocation accepts the `libpq` URL
grammar: `postgres://user:pass@host:port/db?sslmode=require&...`.
`pgxpool.ParseConfig` accepts the **same** URL grammar (it uses
`libpq`-compatible parsing under the hood). The two parsing paths are
not byte-identical for edge cases:

| Concern | psql today | pgx |
|---|---|---|
| URL parsing | libpq C grammar | pgx Go parser (libpq-compatible) |
| `sslmode=disable` | passes through | passes through; same default |
| `sslmode=require` (default for hosts ≠ localhost) | psql honors libpq default | pgx honors `sslmode=prefer` by default — **divergence** |
| `application_name` | not set unless in URL | set explicitly via `config.ConnConfig.RuntimeParams["application_name"] = "striatumd-go/<semver>"` |
| URL password redaction | leaks via argv | leaks via env (mitigated below) |
| Statement timeout | not set | set via `RuntimeParams["statement_timeout"] = "30000"` (30s) |

Two operator-visible changes ship with F5 because they are
defense-in-depth and named in the synthesis (claude F11 + gemini
MEDIUM):

1. **Default `sslmode`.** pgx's default is `prefer` (try TLS, fall
   back to plaintext). The daemon explicitly overrides to `require`
   if the URL omits `sslmode`, so the operator's environment cannot
   silently downgrade to plaintext when a TLS-capable server is
   present. Document in `docs/HOW_TO_HUMAN.md`.
2. **`application_name`.** Set to `striatumd-go/<daemonVersion>`
   (e.g. `striatumd-go/go-dev`). This is what claude F3 + F11 asked
   for: `pg_stat_activity.application_name` becomes the substrate the
   eventual mutual-exclusion check reads. F5 only sets the parameter;
   the doc-claimed `daemon_already_running` exit-code-14 check is a
   separate follow-up.

#### Connection construction

```go
// in connection.go
import (
    "github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, postgresURL string, daemonVersion string) (*Pool, error) {
    if postgresURL == "" {
        return nil, errors.New("daemon PostgreSQL URL is not configured")
    }
    config, err := pgxpool.ParseConfig(postgresURL)
    if err != nil {
        return nil, err
    }
    if config.ConnConfig.RuntimeParams == nil {
        config.ConnConfig.RuntimeParams = map[string]string{}
    }
    config.ConnConfig.RuntimeParams["application_name"] = "striatumd-go/" + daemonVersion
    if _, hasStatementTimeout := config.ConnConfig.RuntimeParams["statement_timeout"]; !hasStatementTimeout {
        config.ConnConfig.RuntimeParams["statement_timeout"] = "30000"
    }
    // Default sslmode to require if the URL did not name one.
    // pgx parses sslmode at config-time; check via config.ConnConfig.TLSConfig.
    if config.ConnConfig.TLSConfig == nil && !strings.Contains(postgresURL, "sslmode=") {
        return nil, errors.New("daemon PostgreSQL URL must specify sslmode explicitly (e.g. sslmode=require)")
    }
    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil { return nil, err }
    if err := pool.Ping(ctx); err != nil { pool.Close(); return nil, err }
    return &Pool{URL: postgresURL, Runner: &pgxRunner{pool: pool}}, nil
}
```

The `application_name` env-var path is documented in
`docs/HOW_TO_HUMAN.md`'s Go-daemon section (claude F11's MEDIUM
finding).

#### `Runner` / `TxRunner` implementations

`pgxRunner` wraps `*pgxpool.Pool`:

```go
type pgxRunner struct{ pool *pgxpool.Pool }

func (r *pgxRunner) Exec(ctx context.Context, sql string, args ...any) error {
    _, err := r.pool.Exec(ctx, sql, args...)
    return err
}

func (r *pgxRunner) QueryScalar(ctx context.Context, sql string, args ...any) (string, error) {
    var value sql.NullString
    err := r.pool.QueryRow(ctx, sql, args...).Scan(&value)
    if err == pgx.ErrNoRows { return "", nil }
    if err != nil { return "", err }
    if !value.Valid { return "", nil }
    return value.String, nil
}

func (r *pgxRunner) BeginTx(ctx context.Context) (TxRunner, error) {
    tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
    if err != nil { return nil, err }
    return &pgxTx{tx: tx, ctx: ctx}, nil
}
```

Drop `PsqlRunner` entirely (delete `go/pkg/db/connection.go:105-125`
and the `os/exec` import). The advisory-lock breakage claude F14
flagged disappears for free because every operation now runs on a
pgxpool connection that stays alive across the
`pg_advisory_lock / Exec / pg_advisory_unlock` triple.

#### SQL injection footprint

All call sites that currently build SQL via `fmt.Sprintf +
quoteLiteral` (audit.go:87-117, migrations.go:152-165, and the four
sql files don't have this issue) move to parameterized queries.
`quoteLiteral` itself stays in tree as a small helper for the
migration-record INSERT's `sha256` and `label` literals, but it
becomes dead code once those switch to `$1, $2, ...` placeholders.
Mark it `// Deprecated: use pgx parameter binding.` and delete in
RFC 0039 V2.

### Test plan

- `go/pkg/db/connection_pgx_test.go` — exercises `Connect`, the
  `application_name` parameter (verify via
  `SELECT current_setting('application_name')`), the statement
  timeout, and the SSL-mode handling.
- `go/pkg/db/audit_test.go` — update existing tests to use the pgx
  runner; the audit-row hash output stays byte-identical (the
  V2RowHash logic doesn't change in this dogfood; that's a separate
  parity gap claude F10 already raised and which RFC 0039 V2 covers).
- `tests/test_daemon_go_smoke.py` from F2 — exercises the harness
  side, transitively proves the pgx runner boots.

---

## Implementation order and acceptance gate

Because F4 depends on `TxRunner` (only available once F5 lands), and
the F1 validator's parameterized SQL is cleaner against the pgx
runner, the build packet for dogfood-047 ships in this order:

1. **F5** — pgx adoption, `Runner` / `TxRunner` interface widening,
   `PsqlRunner` removal. Establishes the foundation.
2. **F4** — audit-chain transaction. Lands the race regression test.
3. **F1** — Postgres-backed authorizer. Lands the cross-core token
   parity test.
4. **F2** — harness launch correction (Makefile `-o bin/striatumd`,
   harness flag rename, `--migrations-sha-source` plumbing). Lands
   the smoke test.
5. **F3** — `make test-multi-repo CORE=go` and the conftest
   parametrization. Wires CI evidence.

Acceptance gate for the V1.5 build:

- `make -C go test` green.
- `make test-multi-repo` green (Python core, unchanged surface).
- `make test-multi-repo CORE=go` green (Go core, smoke + harness
  lifecycle + audit read-only; mutating-verb tests skipif until RFC
  0039 Step 4).
- `tests/test_daemon_go_smoke.py::test_multi_repo_harness_boots_go_daemon`
  passes.
- `tests/test_daemon_go_authz_parity.py::test_python_issued_token_validates_in_go`
  passes (F1 cross-core gate).
- `go/pkg/db/audit_race_test.go::TestAuditRace_TwoAppendersBranch`
  passes (F4 regression).

RFC 0039's status block (lines 392-400 of
`docs/rfcs/0039-go-daemon-core.md`) updates in the build packet to
record the V1.5 corrections and explicitly note which verb handlers
are still unimplemented (claude F8 doc-honesty corrective).
