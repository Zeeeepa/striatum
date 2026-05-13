---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["threat_model", "rfc-0039", "track_a", "design"]
---

author: reviewer-claude-opus-001

# Track A Design Review — Go Daemon Phase 1 (threat_model posture)

Target: `docs/dogfood/042/track_a/DESIGN_SYNTHESIS.md` — RFC 0039 Phase 1
Steps 1+2 (envelope-v1 RPC skeleton + PostgreSQL substrate).

Posture: threat_model. Enumerate the trust boundaries and attack surfaces
the artifact introduces; acceptance requires each is acknowledged or
mitigated.

## Trust boundary enumeration

The synthesis introduces or touches the following trust boundaries:

1. **CLI client ↔ daemon** over Unix domain socket (envelope-v1 framing).
2. **Daemon ↔ PostgreSQL** via `pgxpool`; credential acquisition from
   flag/env/conf.
3. **Daemon ↔ on-disk migration files** under
   `src/striatum/daemon_pg/sql/*.sql`.
4. **Daemon ↔ audit chain** (`striatumd.audit_log` +
   `striatumd.audit_chain_head`); cross-language hash parity with the
   Python verifier.
5. **Go daemon ↔ Python daemon** mutual exclusion on a shared substrate
   (one operator, two implementations).
6. **Test-harness ↔ Go binary** (`STRIATUMD_GO_BIN`, `make -C go build`)
   — runtime execution of a developer-chosen binary during the e2e
   smoke.
7. **Pre-handshake reachable surface** — `daemon.hello` is callable
   without a capability token.

Each is acknowledged below with the design's mitigation and any residual
risk worth flagging before implementation.

## Boundary 1 — RPC socket and capability gate

**Mitigations present.** Socket bound at `0600`; parent-directory
world-writable check; stale-socket probe via `unix.Connect`; bounded
1 MiB scanner line cap; handshake-required state machine; explicit
registry (no `init()`-time registration so a misplaced import can't
silently expose a verb); closed capability vocabulary parsed up-front
with `capability_unknown` refusal; argon2id token verification
parameter-matched to the Python daemon; revocation honored by a single
`clients ⨝ client_capabilities` query at check time; every capability
denial path writes an audit row **before** returning the refusal
(brute-force attempts cannot bypass the chain).

**Capability table correctness.** The closed set
`{read, write, review, claim, apply, admin, recovery, surgical_recovery}`
matches the RFC 0030 + RFC 0032 / surgical-recovery vocabulary. Parsing
the full set in Phase 1 even though only `read` gates any registered
verb is defensible — Step 4 inherits a working surface and the cost is
zero. The only registered Phase 1 verb requiring a capability is
`daemon.describe`/`daemon.status`/`audit.show`/`repo.list`/`daemon.version`
on `read`; `daemon.hello` is intentionally pre-handshake.

**Residual risk — finding F1 (low):** capability refusal codes
`capability_token_missing` vs `capability_token_invalid` vs
`capability_token_expired` vs `capability_token_revoked` partition the
denial space along axes that may permit client-existence enumeration
(distinguishing "no such client/token" from "client exists but token
is expired/revoked" reveals which token hashes are known to the
substrate). Phase 1 inherits this from the Python design, so it is not
a regression, but Step 4 (when issuance/rotation lands) should consider
collapsing distinguishable denials into a single `capability_denied`
on the wire while keeping the fine-grained code on the audit row.

**Residual risk — finding F2 (low):** the bounded 1 MiB line cap
mitigates large-frame parser DoS. Slow-read / Slowloris-style attacks
(a client opening many connections and dribbling bytes) are not
discussed. Single-machine Unix-socket deployment makes this a minor
concern, but a `ReadDeadline` on the connection (or a connection-count
cap with idle timeout) would be a cheap addition before Step 4 opens
mutating verbs.

**Residual risk — finding F3 (low):** `daemon.hello` is reachable
pre-auth and `daemon.welcome` discloses `{daemon_version,
daemon_core, envelope, framing, substrate, substrate_schema,
methods_etag, sealed_apply}`. This is intentional version-skew
negotiation per RFC 0030, but the threat model should explicitly
accept: an attacker with socket access (i.e., already on-host as the
same user) learns the core implementation, substrate schema version,
and registry etag without authenticating. Acceptance noted; no change
recommended for Phase 1.

## Boundary 2 — PostgreSQL credentials

**Mitigations present.** URL resolution precedence is explicit:
`--db-url` → `STRIATUM_DAEMON_DB_URL` → `~/.config/striatum/daemon.conf`.
Pool errors are wrapped with a redactor that strips `password=…`
fragments before any string surfaces to the CLI or audit row. The
daemon refuses non-loopback hosts unless `--allow-remote-pg` is passed
(D083 single-machine posture). The daemon connects as a role with only
`INSERT` on `striatumd.audit_log` + `UPDATE` on
`striatumd.audit_chain_head`; everywhere else it is `SELECT`-only.
Defense-in-depth on the DB side is the strongest mitigation in the
package.

**Finding F4 (low) — argv credential leakage.** A password embedded in
`--db-url postgresql://user:secret@localhost/...` appears in `ps aux`
and `/proc/<pid>/cmdline` (readable by the same uid). The synthesis
documents redaction at error-string surfaces but does not specify
behavior at process-listing surfaces. Suggested implementation note:
either refuse `--db-url` whose userinfo contains a password (forcing
env/conf paths) or document the operator hazard in `HOW_TO_HUMAN.md`.
The env-var path is materially safer than argv on Linux because
`/proc/<pid>/environ` is mode 0400 and is not readable through `ps`.

**Finding F5 (low) — conf file permission check unspecified.** The
synthesis names `~/.config/striatum/daemon.conf` as a credential source
but does not say whether the daemon refuses to read it when
group/world-readable (the SSH/PostgreSQL `.pgpass` convention). For
parity with established Postgres tooling, refuse-on-loose-permissions
is recommended; at minimum, log a warning. Not blocking — Phase 1
deployments are single-developer.

**Finding F6 (low) — redactor URL-format coverage.** The synthesis
mentions stripping `password=…` fragments. PostgreSQL connection URLs
also carry credentials in userinfo (`postgresql://user:secret@host/db`).
The Phase 1 `Doctor` summary returns a "redacted URL"; implementation
must redact both the query-parameter form and the userinfo form. Add
a unit test that asserts neither form leaks past `Doctor` or any error
string. The synthesis notes the redactor exists; this is a coverage
note.

## Boundary 3 — migration loader

**Mitigations present.** Advisory lock key `332933` matches the Python
runner so concurrent migrations across cores cannot race. SHA-256
verification against `striatumd.schema_migrations` blocks hand-edits to
a frozen migration with stable `schema_drift_detected`. Startup
refuses if the recorded schema is newer than the binary supports.
Filesystem-only sourcing in Phase 1 keeps Python as single source of
truth — embedding is appropriately deferred to Step 6 with build-time
SHA verification.

**Finding F7 (low) — SHA stability across checkouts.** SHA-256 over
the literal file bytes is sensitive to line-ending and trailing-newline
differences. Contributors on Windows checkouts with CRLF
auto-conversion (or repository `.gitattributes` quirks) could trip
`schema_drift_detected` against a substrate originally written by
LF-terminated checkout, or vice versa. Recommend the implementer pin
a stable normalization (e.g., normalize to LF before hashing, or
commit a `.gitattributes` entry forcing LF on `*.sql`) and unit-test
the hash against the byte-canonical fixture the codex packet ships.

**Finding F8 (low) — missing-file behavior.** The design specifies
behavior on SHA mismatch but not on a previously-applied migration
whose file is absent at startup. The conservative posture (refuse,
match the drift exit code) should be made explicit so an operator who
truncates `src/striatum/daemon_pg/sql/` cannot quietly continue with a
half-known schema.

## Boundary 4 — audit chain integrity

**Mitigations present.** The chain head is updated under
`SELECT ... FOR UPDATE` within the same short transaction as the
insert, so the chain cannot fork under concurrent writers. v2 is the
only shape the Go daemon appends; v1 rows are refused on append, and
v1 verification is delegated to the Python verifier for historical
segments. The cross-language hash parity test (Python fixture →
`go/pkg/db/testdata/v2_row_hash_fixture.json`) is called out as a
release blocker, which is the right gating posture.

**Canonical JSON correctness.** The synthesis lands the
`sort_keys=True`, `separators=(",", ":")` convention from
`src/striatum/db.py::json_dumps` and explicitly rejects
`map[string]any` on the hash path in favor of an explicit struct +
sorted-keys canonical encoder. This is the right call.

**Finding F9 (medium-leaning-low) — canonical-JSON edge cases not
enumerated.** Three implementation traps for byte-equality across
Python and Go that the synthesis does not call out:

- **ASCII escapes.** Python `json.dumps` defaults to `ensure_ascii=True`
  (non-ASCII characters serialize as `\uXXXX`). Go's standard
  `encoding/json` does **not** by default; the canonical encoder must
  match Python's `ensure_ascii=True` behavior or non-ASCII content in
  `denial_reason` / `method` / `client_id` will hash differently.
- **Null vs absent.** The material-fields list includes optional
  values (`denial_reason`, `repository_id`, `request_id`). The Python
  side, depending on how the row dict is constructed, may emit `null`
  or omit the key. The Go canonical encoder must commit to one
  policy and the Python verifier fixture must exercise both
  branches. This is the single highest-leverage place a Phase 1 bug
  could land an undetectable cross-core hash divergence.
- **Integer encoding.** If any field becomes a 64-bit timestamp
  (e.g., `ts` as ns), Go's `encoding/json` round-trips through
  `float64` for `interface{}` decoding; the canonical encoder over
  a typed struct (as the synthesis specifies) avoids this, but the
  Python fixture must use `json.dumps` on a Python `int`, not on a
  `float`. Worth a fixture row that includes the maximum int64 to
  prove the path is exercised.

Recommendation: the Go packet's `v2_row_hash_fixture.json` should
include rows exercising each of these three traps, not just a
representative canonical row. Severity stays low because the parity
test is a release blocker by design, but the fixture must be designed
to actually catch these.

**Finding F10 (low) — `audit.show` is not self-audited.** "Consistent
with the Python daemon", per §3.3, and called out in the synthesis.
The threat-model implication: anyone holding `read` can enumerate the
entire audit log without leaving a trace in that same log. Acceptable
for Phase 1 (the alternative recursively self-audits and is its own
problem), but the team should know that audit-log read access is a
silent capability and treat `read` accordingly when issuance lands in
Step 4.

## Boundary 5 — Python / Go daemon coexistence

**Mitigations present.** Ownership enforced at the PostgreSQL layer
via `pg_stat_activity application_name LIKE 'striatumd-%'`. The Go
daemon exits with code 14 `daemon_already_running` if any active
session matches. The lock lives in the place both cores already share.

**Finding F11 (low — already acknowledged in the artifact).** The
synthesis explicitly states the Python daemon's symmetric check is
deferred to "a small follow-up before Step 4" and Phase 1 relies on
operators stopping the Python daemon first. Threat-model implication:
the asymmetry creates an operator-error window in which both daemons
can attempt the same substrate, with mutual-exclusion enforced only
on the Go side. For Phase 1 (Go daemon is opt-in for developers
running it directly), the residual risk is bounded — the advisory
lock on migrations and the `FOR UPDATE` on `audit_chain_head` prevent
the two highest-impact races. Acceptable as designed; flag in the
landing notes so the Python-side check does not slip past Step 4.

## Boundary 6 — test-harness binary execution

**Finding F12 (low — scope-bounded).** `STRIATUMD_GO_BIN` is honored
by the harness as an override for the auto-built `./go/bin/striatumd`
path. This is test-time, developer-machine scope, and ships behind the
`@pytest.mark.requires_go_daemon` marker. No CI matrix change in Phase
1. Threat-model posture: not a production attack surface. Acceptable.
A one-line comment in the harness dispatch function noting "trusted
developer environment only" would prevent the override from quietly
acquiring different semantics if the harness ever runs in a less
trusted context.

## Boundary 7 — pre-handshake reachable surface

Covered under Boundary 1, Finding F3. No additional findings.

## Out-of-scope items the threat model relies on

These items are explicitly deferred but materially affect the Phase 2
threat picture. Calling them out so they don't drift:

- **Step 3 (CLI `--core go` flag, launcher discovery).** Launcher
  search semantics for the Go binary will themselves be a trust
  boundary (PATH precedence, suid concerns). Out of scope for
  acceptance; on the list for the Step 3 dogfood.
- **Step 4 (mutating verbs + apply-receipt signing + capability-token
  issuance).** Every finding above marked "address in Step 4" lands
  here. The token-issuance design is where F1, F4, F5 should be
  re-evaluated against the live issuance flow.
- **Step 5 (supervised processes via `os/exec` + `creack/pty`).** This
  is the real subprocess trust boundary the prompt asked about; Phase
  1 has no subprocesses beyond the harness fixture (F12). The deferral
  is appropriate; supervised-process design should be reviewed under a
  threat_model posture in its own packet.
- **Step 6 (distribution: cross-compile, release binaries, `//go:embed`
  of SQL with build-time SHA, top-level Makefile targets, CI matrix
  flip).** Supply-chain hygiene for `go.sum`, `GOSUMDB`, vulnerability
  scanning, and binary provenance is not enumerated in Phase 1 because
  Phase 1 only builds on developer machines. **Finding F13 (low):**
  the synthesis pins `go.mod` versions and forbids CGO but does not
  state a position on `GOPROXY`/`GOSUMDB`/`govulncheck` integration.
  Acceptable for Phase 1 (no published binary); flag for the Step 6
  threat review.

## Verdict

`accept_with_findings`.

The synthesis is a tight scope (envelope-v1 + Postgres substrate,
read-only verb surface only) and the trust boundaries it touches are
each acknowledged and mitigated. The contested decisions (canonical
JSON sort policy, filesystem-only migration source, advertise-only
implemented verbs, `pg_stat_activity` liveness, `striatumd` binary
name) all favor the more conservative and verifiable option. Defense
in depth is taken seriously — capability-denial auditing before
refusal, role-restricted DB user, advisory-lock + SHA-pinned
migrations, panic-recovery audit, cross-language hash fixture as a
release blocker.

Findings F1–F13 are nits, coverage notes, or acknowledged residuals;
none block landing. The highest-leverage one is F9 (canonical-JSON
edge cases in the parity fixture) — the codex implementer should
explicitly design the `v2_row_hash_fixture.json` rows to exercise
non-ASCII content, null-vs-absent optional fields, and int64
boundary values, not just a representative case. The asymmetric
Python-side ownership check (F11) should not be allowed to slip past
the Step 4 dogfood.

No trust boundary in the Phase 1 surface is unaddressed.
