---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0039", "go-daemon", "build", "track_a"]
---

author: reviewer-claude-opus-006

# Track A Build Review — Go Daemon Phase 1 (claude, threat_model posture)

Targets (under `write_scope.allowed_paths` for the codex + claude_code
build packets that landed together):

- `go/` (codex packet) — `cmd/striatumd/main.go`,
  `pkg/rpc/{envelope,registry,capability,server}.go`,
  `pkg/db/{connection,migrations,audit}.go`,
  `pkg/db/sql/000{1,2,3,4}_*.sql`, `go.mod`, `go.sum`, `Makefile`.
- `tests/_harness/daemon.py` and `tests/_harness/multi_repo.py`
  (claude_code packet) — `daemon_core` parameter wiring.
- `docs/HOW_TO_HUMAN.md`, `docs/SPEC.md`, `docs/UBIQUITOUS_LANGUAGE.md`,
  `docs/rfcs/0039-go-daemon-core.md`, top-level `Makefile`
  (claude_code packet) — doc + Make-target glue.
- Inputs: [DESIGN_SYNTHESIS](../../../DESIGN_SYNTHESIS.md),
  [design threat review](../../design/threat/REVIEW.md), RFC 0030,
  RFC 0033, RFC 0035, RFC 0039.

Lane angle (per the prompt's per-lane split): **Python ↔ Go integration
correctness**, harness extension, doc honesty, and the
Python-subprocess-spawns-Go-binary trust boundary. The codex review
owns Go-side systems concerns and the gemini review owns the adversarial
supply-chain angle.

Posture: **threat_model**. Per the review-policy instruction, acceptance
requires each new trust boundary and attack surface to be acknowledged
or mitigated. The asymmetry of this review is that several boundaries
are unmitigated **because they are non-functional**, not because they
are exposed — that is its own form of integration risk and is the
primary reason this verdict is `accept_with_findings`.

## Trust-boundary enumeration (Phase 1, Steps 1+2 only)

The build adds or instantiates the following boundaries beyond the
design-review enumeration (B1–B7 in
[`../../design/threat/REVIEW.md`](../../design/threat/REVIEW.md)):

- **TB-Glue-1.** Python harness (`tests/_harness/daemon.py::_start_go`)
  spawns the Go binary as an untrusted subprocess and waits for the
  socket. The trust boundary is process spawn + flag wiring + socket
  rendezvous.
- **TB-Glue-2.** Python harness auto-builds the Go binary via
  `make -C go build` when `STRIATUMD_GO_BIN` is unset. The trust
  boundary is the in-tree Go module's reproducibility and the
  developer's `go` toolchain.
- **TB-Glue-3.** Doc surface (`HOW_TO_HUMAN.md`, `SPEC.md`,
  `UBIQUITOUS_LANGUAGE.md`, RFC 0039 status) is the contract operators
  read to decide which core to run, where the binary lives, what flags
  it takes, and what verbs it answers. The trust boundary is
  doc-implementation honesty: a doc that disagrees with the code is a
  silent guarantee the operator may rely on.

Each is acknowledged below with the build's mitigation (where present)
and the integration defects observed in this round.

## TB-Glue-1 — Python harness ↔ Go binary subprocess

The harness extension (`_start_go`) is structurally correct in shape:
keyword-only `daemon_core: Literal["python","go"]` parameter with
default `"python"`, separate `_start_python` and `_start_go` code
paths, shared `_wait_for_socket` loop, shared `stop` / `kill`
semantics, socket-path resolution in one place
(`scratch_dir/runtime/striatumd.sock`). The `daemon_core` parameter is
threaded end-to-end through `MultiRepoHarness.__init__`,
`MultiRepoHarness.start`, `MultiRepoHarness.restart_daemon`, and a
read-only `daemon_core` property is exposed for `pytest.mark.skipif`
shape gates. Backward compatibility is intact by construction: every
existing call site that did not pass `daemon_core=` still gets the
Python core.

### F1 — Harness ↔ Go flag wiring is broken (medium)

The harness invokes:

```
striatumd --socket <path> --db-url <pg-url> --migrations-dir src/striatum/daemon_pg/sql
```

(`tests/_harness/daemon.py:117-125`).

The Go binary defines exactly two flags that overlap (`go/cmd/striatumd/main.go:24-28`):

- `--socket` ✓
- `--postgres-url` (not `--db-url`)
- `--migrate` (boolean)
- `--describe`

Neither `--db-url` nor `--migrations-dir` exists in the Go binary. Go's
`flag.Parse()` uses the default `ExitOnError` handler, so the Go
process prints "flag provided but not defined: -db-url" and exits with
status 2 before the socket is created. The harness then hits its
10-second `_wait_for_socket` deadline, the subprocess is already gone,
and `proc.communicate(timeout=1)` raises with the stderr from the Go
binary.

**Net effect: `MultiRepoHarness(daemon_core="go")` cannot launch the
Go daemon at all in the current tree.** The "test parity" acceptance
criterion in RFC 0039 §7 and §11 ("`MultiRepoHarness(daemon_core="go")`
boots the Go daemon and runs all five e2e test files green") is
unverifiable end-to-end as shipped. The defect is symmetric: the
synthesis (§2 `main.go`, §5.2 harness) named the flag pair
`--db-url` + `--migrations-dir`; the claude packet implemented those
names on the call site; the codex packet shipped `--postgres-url` and
embedded migrations.

This is the primary integration defect of the round and the largest
single reason to gate Phase 2 work on a follow-up rather than treating
Steps 1+2 as functionally landed.

Suggested resolution direction (not in scope for this review): either
rename the Go flag to `--db-url` and add a `--migrations-dir <path>`
flag that overrides the embedded loader, OR rename the harness call
site to `--postgres-url` and drop `--migrations-dir`. Whichever side
moves, the synthesis (§2.1) and the doc surface (TB-Glue-3 below) need
to track.

### F2 — `make -C go build` does not produce the binary the harness loads (medium)

`go/Makefile` defines `build` as:

```
go build ./cmd/striatumd
```

With no `-o` flag, this writes the binary to `go/striatumd` (the cwd
of the `make` invocation, which is `go/` because the top-level
`Makefile` does `$(MAKE) -C "$(MAKEFILE_DIR)/go" build`). The harness,
however, resolves the default binary at
`<repo>/go/bin/striatumd` (`tests/_harness/daemon.py:23,
_DEFAULT_GO_BIN`) and explicitly raises after `make` returns if the
path is missing:

```python
if not binary.exists():
    raise RuntimeError(f"`make -C {_GO_DIR} build` completed but {binary} is missing")
```

So even if F1 were fixed, the harness still cannot find the just-built
binary. The synthesis Makefile (§1.2) specified `BIN := bin/striatumd`
with `go build -o $(BIN) -ldflags ...`. The shipped Makefile lacks
both. The codex handoff explicitly notes the binary was emitted at
`go/striatumd` and was deleted after verification.

Suggested resolution (out of scope): align the Makefile to the
synthesis (`-o bin/striatumd`) OR change `_DEFAULT_GO_BIN` in the
harness. Pair this with F1.

### F3 — Doc-claimed coexistence enforcement is not implemented; harness can stack daemons (low)

`docs/SPEC.md` and `docs/HOW_TO_HUMAN.md` both state:

> Mutual exclusion is enforced at the PostgreSQL layer: a daemon
> refuses to start with exit code 14 `daemon_already_running` when
> `pg_stat_activity` already lists a `striatumd-*` connection.

The Go binary's startup path (`main.go:30-69`) performs no
`pg_stat_activity` check. The `application_name` is never set on the
DB connection (see F11), so even the Python daemon's own future
symmetric check has no signal to look for. The `daemon_already_running`
exit code is unused.

For the harness boundary the residual risk is bounded — each test
spawns its own ephemeral PostgreSQL via
`tests/_harness/pg.create_ephemeral_database` — but the doc surface is
making a guarantee operators may rely on while choosing between cores.
Pair fix with F11.

### F4 — `socket_path` lifecycle is silently shared between cores (low)

The harness writes `scratch_dir/runtime/striatumd.sock` regardless of
core, and the Go listener removes any pre-existing path before binding
(`go/pkg/rpc/server.go:159-170`). In the harness, scratch dirs are
per-test so this is safe in practice. The risk is the precedent set
for outside-harness operators: pointing the Go binary at a socket path
the Python daemon already binds will silently steal the file.

Threat-model implication is small (operator already on-host as the
same uid), and the design threat review F11 already flagged the
asymmetric ownership check. Worth noting that the harness inherits the
asymmetry: nothing in `daemon.py` rejects a `daemon_core="go"` start
when an earlier Python daemon hasn't been stopped.

### F5 — `STRIATUMD_GO_BIN` override has no developer-environment comment (informational)

The design review (F12) recommended a one-line "trusted developer
environment only" comment on the override dispatch. The shipped
`_resolve_go_binary` has none. This is a documentation nit, not a
defect — the harness path is test-only — but it is a cheap signal to
prevent the override from quietly acquiring different semantics if
someone reuses this dispatch in a less trusted context.

## TB-Glue-2 — Auto-build via `make -C go build`

The auto-build is gated on `shutil.which("make")` and
`shutil.which("go")` being present and raises a clear message
otherwise. `STRIATUMD_GO_BIN` is honored before any build attempt.
`subprocess.run([..., "make", "-C", _GO_DIR, "build"], check=True,
cwd=ROOT)` invokes `make` with a fixed argv, so there is no shell
expansion or env injection through the Python side. Good.

Module supply chain is checked by the gemini lane; from the Python ↔
Go-integration angle the codex packet's choice to use only the Go
standard library (`go.mod` has no `require` block; `go.sum` is empty)
is a positive — `make -C go build` performs no network fetch and no
module checksum verification path is reachable. This intentionally
restricts the threat model of the Phase 1 build to the developer's
local Go toolchain.

### F6 — Schema-fork risk via embedded SQL copies (medium)

The codex packet shipped `go/pkg/db/sql/0001_baseline.sql` through
`0004_dogfood_surgical_recovery.sql` as **byte-for-byte copies** of
`src/striatum/daemon_pg/sql/*.sql`, embedded via `//go:embed sql/*.sql`
(`go/pkg/db/migrations.go:19-20`). The DESIGN_SYNTHESIS §1.2 and §3.2
explicitly rule this out for Phase 1:

> No `//go:embed` of SQL in Phase 1. The Go daemon loads migrations
> directly from `src/striatum/daemon_pg/sql/*.sql` resolved via
> `--migrations-dir` (default: upward search for `go.mod` then
> `../src/striatum/daemon_pg/sql`). Embedding is a Step 6 distribution
> concern and is deferred.

And:

> No `//go:embed` in Phase 1 — keeping Python as the single source of
> truth removes the drift class entirely. Embedding lands with Step 6
> release packaging, with build-time SHA verification against the
> Python source.

The shipped tree has **no SHA verification** between the Go embedded
copies and the Python originals — `migrations_test.go::TestMigrationsAreOrdered`
asserts each migration hashes to *something*, but not that the hash
matches the Python source. A future migration committed only to
`src/striatum/daemon_pg/sql/` will silently diverge until someone
hand-syncs the Go tree; conversely a hand-edit to a Go-side file will
hash differently from the Python verifier's expectation only if the
audit-chain row hash is checked, and the audit verifier doesn't check
migration file hashes.

Today the four files are bit-identical (verified by `diff` over the
four pairs at review time). The defect is structural: this is the
exact drift class the synthesis named. Pair this with F2 (Makefile)
to either move to a `--migrations-dir`-default-to-Python-tree posture
(synthesis design) or land Step 6 SHA verification now.

## TB-Glue-3 — Doc honesty

Doc honesty is the leading symptom of the integration drift above.

### F7 — `HOW_TO_HUMAN.md` Go-daemon section is non-executable as written (medium, paired with F1+F2)

The "Running the Go daemon (developer preview)" section instructs
operators to run:

```bash
make -C go build
ls go/bin/striatumd
...
./go/bin/striatumd \
  --socket "${XDG_RUNTIME_DIR:-/tmp}/striatum/daemon.sock" \
  --db-url "$STRIATUM_DAEMON_DB_URL" \
  --migrations-dir src/striatum/daemon_pg/sql
```

None of these commands will run successfully against the shipped
codex packet:

- `make -C go build` produces `go/striatumd`, not `go/bin/striatumd`
  (F2).
- `--db-url` is rejected — the binary uses `--postgres-url` (F1).
- `--migrations-dir` is rejected — the binary uses an embedded copy
  (F1 + F6).

This is the doc-honesty boundary in TB-Glue-3 failing for an operator
who follows the docs as written. The defect is uniform across the
codex / claude_code packet split: each side built to the synthesis,
the synthesis was not implemented uniformly, and there is no
integration smoke test (the synthesis's `tests/test_daemon_go_smoke.py`
was explicitly deferred — see "Out of scope" in
`docs/dogfood/042/track_a/build/glue/HANDOFF.md`).

### F8 — Read-only verb list is documented but only `daemon.hello` and `daemon.describe` are handled (medium)

`docs/HOW_TO_HUMAN.md`, `docs/SPEC.md`, and the new RFC 0039 status
callout all claim the Phase 1 method registry exposes seven verbs:
`daemon.hello`, `daemon.welcome`, `daemon.describe`, `daemon.status`,
`daemon.version`, `audit.show`, `repo.list`.

In code (`go/pkg/rpc/registry.go:76-128`), the `methodEntries` slice
includes those seven **plus** the full mutating-verb catalogue
(`run.prepare`, `run.start`, `session.register`, `ack`, `block`,
`heartbeat`, `publish_artifact`, `complete`, `release`,
`recovery.*`, `dogfood.publish_on_behalf`,
`dogfood.surgical_recovery`, `supervise.*`, `apply.*`, `repo.add`,
`repo.remove`, `daemon.token.*`, `cross_repo.*`, `daemon.key.rotate`,
`daemon.shutdown`, `daemon.migrate`). The server router
(`server.go:173-182`) routes only `daemon.describe` internally and
otherwise looks up `s.Handlers[envelope.Method]`. `s.Handlers` is
populated nowhere in `main.go`.

This is **exactly the "false capability promise" the synthesis named
in §6 ("advertise only implemented routes; false capability promises
are worse than a smaller surface").** Concrete consequences:

- `daemon.describe` returns the full catalogue (and the
  `methods_etag` hashes it), so a client introspecting the daemon
  sees mutating verbs advertised. Calling any of them with a valid
  capability gate would pass the authorizer (see F12) and only fail
  at the handler-lookup with `method_unknown`. The seam between
  "advertised but unimplemented" and "advertised and implemented" is
  invisible to clients.
- `daemon.status`, `daemon.version`, `audit.show`, `repo.list` —
  the four doc-claimed read-only verbs beyond hello/describe — are
  registered but have **no handler**, so they return `method_unknown`
  at runtime. The doc surface and the etag both claim they exist; the
  daemon refuses them.
- The `methods_etag` will not match the Python daemon's etag (Python's
  registry presumably advertises a different subset and shape),
  breaking the synthesis §5.2 step "assert the registry is exactly the
  Phase 1 read-only set."

Severity here is the bridge between TB-Glue-3 and TB-Glue-1: docs
promise a read-only verb surface that the daemon doesn't actually
service.

### F9 — `daemon.welcome` does not include `daemon_core` (low)

Synthesis §2.4 specified `daemon.welcome` returns
`{daemon_version, daemon_core: "go", envelope, framing, substrate,
substrate_schema, methods_etag, sealed_apply}`. The Go implementation
(`go/pkg/rpc/server.go:184-208`) returns every field **except**
`daemon_core`. A Python client cannot distinguish which core
answered, which defeats the operator-visibility motivation in the
UBIQUITOUS_LANGUAGE entry ("operator selects the running core out of
band in Phase 1; the Python CLI flag … lands in Phase 2"). Trivial
addition; no security impact in Phase 1.

## Audit-chain parity (Boundary 4 carried forward from design review)

The design review's Finding F9 flagged three canonical-JSON edge
cases (ASCII escapes, null vs absent, integer encoding) and made the
parity fixture (`go/pkg/db/testdata/v2_row_hash_fixture.json`) a
release blocker. The build packets shipped neither the fixture nor
the Python generator (both are listed as out of scope in the
glue handoff). The Go audit implementation thus has **no enforced
parity check against the Python verifier**.

Inspecting `go/pkg/db/audit.go::V2RowHash`:

### F10 — Audit row hash uses `map[string]any` + `encoding/json.Marshal` defaults; parity with Python is not verified and is likely broken on three classes of input (high)

```go
func V2RowHash(row map[string]any) (string, error) {
    material := map[string]any{
        "ts": row["ts"], ... "previous_hash": row["previous_hash"],
        "segment_id": row["segment_id"],
    }
    return CanonicalHash(material)
}

func CanonicalHash(payload any) (string, error) {
    body, err := json.Marshal(payload)
    ...
}
```

Three concrete divergences from
`src/striatum/daemon_pg/audit.py::v2_row_hash` (which uses
`src/striatum/db.py::json_dumps`, i.e. `json.dumps(..., sort_keys=True,
separators=(",", ":"), ensure_ascii=True)`):

1. **HTML escape of `<`, `>`, `&`.** Go's `encoding/json.Marshal`
   defaults to escaping `<`, `>`, and `&` as `<`, `>`,
   `&`. Python's `json.dumps` does not. **Any audit row whose
   `method`, `denial_reason`, `client_id`, or other material string
   field contains `<`, `>`, or `&` will hash differently on the two
   cores.** The Python verifier will then refuse the chain, breaking
   the SPEC.md claim "audit chains written by either core verify with
   either core's verifier." `denial_reason` is the most likely
   field to acquire a `<` or `>` (e.g. "method <unknown>" style
   messages). The Go encoder must call `json.Encoder` with
   `SetEscapeHTML(false)` or use a custom canonical encoder. The
   synthesis §3.3 explicitly named this as the path requiring a
   canonical encoder over an explicit struct — the implementation
   chose `map[string]any` + default `Marshal` instead.
2. **`ensure_ascii=True` parity.** Python emits non-ASCII characters
   as `\uXXXX` escapes by default. Go's `encoding/json` emits the raw
   UTF-8. Any non-ASCII content (operator name with accents, log
   message in CJK, etc.) hashes differently. Same severity as item 1.
3. **`segment_id` typing.** `audit.go:58-61` reads
   `segment_id` from `pg_stat_activity` via a string scalar query,
   defaults to the string `"1"` if empty, then puts the string into
   `material["segment_id"]`. Python's verifier, depending on row dict
   construction, may carry `segment_id` as an `int`. `json.Marshal`
   emits `"1"` (with quotes) for the Go side; `json.dumps` emits
   `1` (no quotes) for a Python int. Hash diverges.

Additionally:

- `material["ts"]` is a string formatted as RFC3339 truncated to
  seconds. The Python verifier may construct `ts` differently (e.g.
  ISO 8601 with microseconds, or a `datetime` object). The
  byte-level shape of `ts` between the two cores is uncoordinated.
- `material["exit_code"]` is set from `exitCode` (`"10"` string or
  `"NULL"` string), not an integer. Python likely uses an int or
  None. Hash diverges.
- `nullString` returns `nil` for empty strings (encoded as JSON
  `null`); the Python row constructor may instead **omit** the key.
  The design review F9 said the Go canonical encoder must commit to
  one policy and the fixture must exercise both branches. There is
  no fixture and no commitment.

The audit chain insert in
`audit.go::AppendAuditRow` further builds the `INSERT` SQL via
`fmt.Sprintf` and `quoteLiteral`, and **discards the returned
`audit_id`**:

```go
sql := fmt.Sprintf(`
WITH inserted AS (
  INSERT INTO striatumd.audit_log (...) VALUES (...)
  RETURNING audit_id, row_hash
)
UPDATE striatumd.audit_chain_head ... FROM inserted ...
`, ...)
if err := a.Runner.Exec(ctx, sql); err != nil { return "", err }
return "", nil
```

`PsqlRunner.Exec` calls `psql -c <sql>` and discards stdout. The
function returns the empty string for `audit_id`, so
`response.AuditID` will always be empty for the Go core. The Python
client therefore cannot correlate a server-acknowledged audit row to
a follow-up `audit.show` call by `audit_id`. This is more of an
RFC 0030 envelope-shape regression than a security defect but it is
observable to clients.

Severity is **high** here despite the rest of the review being
medium-ceiling because the audit chain is the property that gives
both cores their cross-language guarantee, and the SPEC.md text
asserting that guarantee is unverified and at least three independent
ways wrong.

## Secondary findings (TB-Glue spillover into the Go binary's
process-level posture)

These are Go-side findings the prompt told me to leave to the codex
lane, but they materially affect the Python-side claim "the harness
can run the same e2e suite," so I'm recording them under the
integration lens.

### F11 — Go binary uses `psql` subprocess; PG URL is on the command line (medium)

`go/pkg/db/connection.go::PsqlRunner` invokes
`psql <URL> -v ON_ERROR_STOP=1 -c <SQL>` via `exec.CommandContext`.
Three implications for the Python integration:

- **Credential leakage to `ps`.** The harness passes the ephemeral
  PostgreSQL URL on `--postgres-url` (well, would, if F1 were fixed).
  The Go binary then re-exposes it as a positional argv to every
  `psql` invocation. The URL contains the password and is visible in
  `/proc/<pid>/cmdline` (readable by the same uid via `ps`) for the
  lifetime of each `psql` invocation. The design review F4 named this
  hazard for the daemon argv itself; the Go binary multiplies it by
  invoking `psql` repeatedly. Pair fix: use `psql`'s `-d` flag with a
  password from `PGPASSWORD` env or move to `pgx`/`lib/pq` as the
  synthesis specified.
- **No `application_name`.** Synthesis §3.1 required
  `application_name=striatumd-go/<semver>` so the
  `pg_stat_activity` mutual-exclusion check (F3, design F11) has
  anything to look at. `PsqlRunner` does not set it.
- **No statement timeout, no idle-in-transaction timeout, no version
  floor check, no host restriction, no Doctor summary.** Each was
  named in the synthesis as a defense; none is implemented.

The synthesis explicitly chose `pgxpool` for these reasons. The shipped
implementation chose subprocess invocation of `psql`. The decision is
not recorded in the codex handoff or anywhere in the build artifacts.

### F12 — `main.go` ships with `AllowAllAuthorizer{}` (medium)

`go/cmd/striatumd/main.go:53` sets:

```go
server.Authorizer = rpc.AllowAllAuthorizer{}
```

`AllowAllAuthorizer.Authorize` returns `Decision: "allowed"`
unconditionally (`go/pkg/rpc/capability.go:25-33`). The
`MemoryAuthorizer` infrastructure with constant-time hash compare,
revocation, expiry, and capability-scope checks exists but is
unwired. Combined with F8 (registry advertises every mutating verb),
this means the Go daemon process as built has **no capability
enforcement at all** for any registered method.

The build is not exposed to the network (Unix socket at 0600), so the
attack surface is the same as "any local user the daemon's uid trusts
already." But the **doc surface** (SPEC.md, UBIQUITOUS_LANGUAGE entry)
implies capability tokens still gate access. The implementation does
not match.

### F13 — `MemoryAuthorizer` uses SHA-256, not argon2id (low for Phase 1, high for Phase 2)

`capability.go::HashSecret` is `sha256.Sum256(secret)`. Synthesis §2.3:

> Token hashes are argon2id with the same parameters as the Python
> daemon, so a token issued by either core validates in both.

SHA-256 plus a constant-time compare is what's implemented. Token
interop with the Python daemon is therefore broken (Python presumably
stores argon2id digests). In Phase 1 this is latent — no tokens are
checked because of F12 — but Step 4 cannot land until this is
reconciled.

### F14 — Migration advisory lock is held in a session that does not survive (medium)

`migrations.go::ApplyMigrations` acquires `pg_advisory_lock(332933)`
by calling:

```go
runner.Exec(ctx, "SELECT pg_advisory_lock(332933)")
```

`PsqlRunner.Exec` spawns a fresh `psql` per call (F11). PostgreSQL
session-level advisory locks are released when the session exits,
which happens the instant the `psql` subprocess closes. The next
`Exec` (the migration body, the `INSERT INTO
striatumd.schema_migrations`, etc.) runs in a different session
without the lock. The deferred `pg_advisory_unlock` runs in yet
another session and is a no-op against an already-released lock.

**Net: the cross-core migration mutex named in the synthesis §3.2
("Lock: PostgreSQL advisory lock key `332933`, matching the Python
migration runner") is non-functional.** Two simultaneous
`ConnectAndMigrate` calls (Python core + Go core; or two Go cores)
race freely. The PostgreSQL row-level constraints in
`striatumd.schema_migrations` would catch a duplicate version insert,
but only after both sides have executed the migration body.

This is the most consequential PG-layer correctness bug in the build:
it disagrees with both the synthesis and the Python migration runner
and undoes a defense the operator may rely on. Severity is **medium**
in Phase 1 (single developer, opt-in) and would graduate higher in
Phase 2 once both cores are in operator use simultaneously.

### F15 — `ListenUnix` does not check parent-dir mode or detect stale-vs-live socket (low)

`server.go::ListenUnix` removes any pre-existing socket path
unconditionally and re-binds at 0600. Design review §B1 specifically
named the parent-dir world-writable check and the live-socket probe
as Phase 1 mitigations. Neither is implemented. The harness scratch
dir is 0700 by `os.MkdirAll(..., 0o700)` so the harness path is safe;
the doc-claimed standalone use under
`${XDG_RUNTIME_DIR:-/tmp}/striatum/daemon.sock` is not.

## What is right (so the next round knows what not to undo)

- The `daemon_core` parameter wiring on the Python side is minimal,
  default-preserving, and threaded end-to-end (`DaemonProcess`,
  `MultiRepoHarness`, `restart_daemon`). The synthesis §5.2 shape
  matches the implementation byte-for-byte.
- The envelope layer (`go/pkg/rpc/envelope.go`) implements the
  refusal-code vocabulary from RFC 0030 (`version_incompatible`,
  `schema_invalid`, etc.) and uses `json.Number` for integer decode
  to avoid float-coercion at the wire boundary.
- Handshake state-machine: the server tracks `handshakeSeen` per
  connection and refuses non-hello routes with `version_incompatible`
  pre-handshake, matching the synthesis §2.4.
- Duplicate request-id detection (`s.markRequest`) is implemented
  and tested.
- `RedactURL` (`connection.go::RedactURL`) covers both the userinfo
  password form and the query-parameter form (`password`, `pass`,
  `token`, `sslpassword`) — design review F6 addressed.
- The `daemon_core` term landed in `docs/UBIQUITOUS_LANGUAGE.md` with
  clear V1 closed set and default.
- The RFC 0039 status block honestly says **Phase 1 Steps 1+2** and
  defers Steps 3-6, so the doc layer correctly signals "developer
  preview" — the body claims are then where the honesty problem
  starts.
- The Go module has no third-party `require` block (standard library
  only) in Phase 1, which materially shrinks the supply-chain attack
  surface for this round.

## Verdict

`accept_with_findings`. Severity **medium**.

Acceptance rationale under the threat_model posture: every new trust
boundary is enumerated and each defect is acknowledged. None of the
defects open a *new exploit class* against the threat model in
isolation — Phase 1 is opt-in, opt-in-developer-only, and not on the
network. The reason the verdict is not bare `accept` is that the
integration boundary (TB-Glue-1, TB-Glue-3) is not just incompletely
mitigated; it is **non-functional as shipped**: the harness cannot
launch the Go daemon (F1+F2), the docs instruct operators to use
flags the binary doesn't recognize (F7), and the read-only verbs the
docs claim are answered by the Go daemon are unhandled (F8).
Acceptance with findings is the right outcome because the work is
load-bearing for Phase 2 *and* the synthesis sets a clear corrective
direction the next packet can take.

The findings cluster into three classes for the consolidation packet:

- **Block Phase 2 entry on these:** F1 (flag wiring), F2 (Makefile
  -o), F6 (`//go:embed` vs single-source SQL or SHA verification),
  F10 (audit-chain canonical JSON parity — pull the deferred fixture
  in or this becomes a release blocker per design F9), F14 (advisory
  lock is broken across `psql` sessions).
- **Doc-honesty pass:** F7, F8, F9, F11 (PG URL on argv). The
  HOW_TO_HUMAN section either gets a "this is the design target, not
  the current build" warning, or — preferred — the build catches up
  before the doc lands.
- **Step 4 / Phase 2 latent:** F12 (`AllowAllAuthorizer`), F13
  (argon2id token interop), F15 (socket parent-dir + stale detection).
  These are not Phase 1 hazards but cannot land in Phase 2 as-is.

No new trust boundary is unaddressed at the *design* level. The build,
however, did not realize the design's mitigations on the boundaries
that matter most for cross-core integration. That is the load-bearing
delta this review records.
