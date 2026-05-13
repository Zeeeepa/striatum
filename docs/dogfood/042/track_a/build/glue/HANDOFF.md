author: implementer-claude-opus-001

# Track A Implement Handoff: Python Glue + Harness Extension + Docs

This handoff records the claude_code-side scope shipped for Track A,
RFC 0039 Phase 1 Steps 1+2 (Go daemon core). The codex-side scope (the
`go/` source tree itself) ships separately; the two packets are
disjoint by design and may land in either order.

## Scope shipped

### Harness extension (RFC 0035 plumbing)

- `tests/_harness/daemon.py`
  - `DaemonProcess` gained a keyword-only `daemon_core: Literal["python","go"]`
    parameter (default `"python"`); existing call sites are unchanged in
    behavior.
  - For `daemon_core="go"`, the harness resolves the binary via
    `STRIATUMD_GO_BIN` (override) or `<repo>/go/bin/striatumd` (default)
    and runs `make -C go build` once when the binary is missing. It
    refuses with a clear error when `make` / `go` are absent and the
    override env var is also absent.
  - The Go invocation passes `--socket=<scratch>/runtime/striatumd.sock`,
    `--db-url=<ephemeral-pg-url>`, and
    `--migrations-dir=<repo>/src/striatum/daemon_pg/sql` so the Go core
    runs the same Python-owned migration set without a `//go:embed`
    drift class (per synthesis §3.2).
  - The startup wait loop, stop / kill semantics, and socket-path
    resolution are shared between cores.
- `tests/_harness/multi_repo.py`
  - `MultiRepoHarness.__init__` gained the kw-only `daemon_core` parameter
    (default `"python"`); `start()` and `restart_daemon()` thread it into
    each `DaemonProcess` instantiation.
  - New `daemon_core` read-only property exposes the chosen core to test
    code (useful for `pytest.mark.skipif` shape gates).
- The other harness modules (`pg`, `repos`, `mcp`, `scope`, `tokens`,
  `audit`, `__init__`) needed no changes — the `daemon_core` distinction
  is fully encapsulated in the daemon launcher.

### Documentation

- `docs/HOW_TO_HUMAN.md` — added a "Running the Go daemon (developer
  preview, RFC 0039 Phase 1)" subsection under "Daemon V2 storage
  substrate (RFC 0033)". Covers `make -C go build`, the binary path,
  the `STRIATUMD_GO_BIN` override, the Phase 1 read-only verb set, the
  "stop the Python daemon first" coexistence rule (PostgreSQL-layer
  mutex with exit code 14 `daemon_already_running`), and the
  `MultiRepoHarness(daemon_core="go")` opt-in.
- `docs/SPEC.md` — extended the daemon section to record RFC 0039 as a
  second daemon implementation under the same RFC 0030 envelope-v1
  contract and the same RFC 0033 PostgreSQL substrate, with the
  V1-closed `{python, go}` core set, the PostgreSQL-layer mutex, the
  Phase 1 read-only verb list, the cross-language v2 audit-row hash
  parity guarantee, and the explicit deferral of Steps 3-6.
- `docs/UBIQUITOUS_LANGUAGE.md` — added a `daemon core` term row in the
  Core Terms table immediately after `Striatum daemon`, naming the
  closed `{python, go}` set and the V1 default.
- `docs/rfcs/0039-go-daemon-core.md` — status line bumped to
  `proposed (Phase 1 Steps 1+2 landed in dogfood-042; Steps 3-6
  deferred to a Phase 2 dogfood)` and a status callout inserted at the
  top of the Implementation Plan section pointing readers to the
  Track A synthesis.

## Out of scope (deliberately not shipped this packet)

The synthesis §7.2 also lists two Python-side fixtures whose paths fall
outside this work packet's `write_scope.allowed_paths`. They are
deferred to a follow-up packet rather than smuggled in:

- `tests/test_daemon_go_smoke.py` — the new
  `@pytest.mark.requires_go_daemon` e2e smoke test that drives
  `daemon.hello`, `daemon.welcome`, `daemon.describe`, the read-only
  verb set, the capability-denial audit-row check, and the
  cross-language Python-verifier audit assertion. Path is
  `tests/test_daemon_go_smoke.py`, which is not under
  `tests/_harness/`; this packet's scope is harness-only.
- `tests/_harness/audit_fixture.py` — the small generator script that
  emits the canonical Python `v2_row_hash` fixture consumed by the Go
  unit tests. Not in scope; the codex packet's
  `go/pkg/db/testdata/v2_row_hash_fixture.json` will reference an
  agreed shape, and the generator can land alongside the smoke test.

The `striatum daemon start --core go` Python CLI flag, mutating-verb
parametrization across `daemon_core`, supervised-process plumbing, and
distribution / CI matrix wiring are deferred to a Phase 2 dogfood per
RFC 0039 §9 and the synthesis §8.

## Inputs / contracts consumed

- [`docs/dogfood/042/track_a/DESIGN_SYNTHESIS.md`](../../DESIGN_SYNTHESIS.md)
  — §3 (`pkg/db` shape), §3.3 (audit-row v2 hash), §5.2 (harness
  `daemon_core` shape), §7.2 (this packet's split).
- [`docs/rfcs/0039-go-daemon-core.md`](../../../../rfcs/0039-go-daemon-core.md)
  — §4 selection mechanism (Phase 2 CLI flag stays out of Phase 1),
  §6 supervision (Phase 2), §7 test parity, §9 phased migration.
- [`docs/rfcs/0035-multi-repo-test-harness-for-cross-repo-workflows.md`](../../../../rfcs/0035-multi-repo-test-harness-for-cross-repo-workflows.md)
  — `MultiRepoHarness` shape and existing daemon-launch contract.
- [`docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md`](../../../../rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md)
  — envelope-v1 method contract referenced by the Phase 1 read-only
  verb list.
- [`docs/rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md`](../../../../rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md)
  — PostgreSQL substrate, `src/striatum/daemon_pg/sql/` migration
  source.

## Verification

- The harness changes are backward-compatible by construction: the
  `daemon_core` parameter is keyword-only with default `"python"`, so
  every existing `MultiRepoHarness(daemon_pg_url=...)` and
  `DaemonProcess(scratch_dir=..., postgres_url=...)` call site keeps
  its prior behavior. The two existing call sites in
  `multi_repo.py` (`start()` and `restart_daemon()`) were updated to
  forward the new parameter.
- No mutating CLI verb, schema, or workflow contract was touched.
- Per the Track A implementer role note (and dogfood-038 OPERATOR_REPORT
  intervention #5), I did not run `make test` end-to-end; the Python
  edits are syntax-clean and the touched modules are import-only at
  test-collection time. The Go-binary launch path will be exercised
  for real once the codex packet lands `go/bin/striatumd` and the
  follow-up `tests/test_daemon_go_smoke.py` lands.

## Coexistence with the codex Track A packet

- The codex packet writes the Go source tree under `go/` (synthesis
  §7.1). It does not touch any path in this packet's
  `write_scope.allowed_paths`.
- This packet only references the Go binary at runtime (via
  `<repo>/go/bin/striatumd` or `STRIATUMD_GO_BIN`); when the binary
  is missing it shells out to `make -C go build`. The harness
  therefore degrades cleanly during the window where the codex
  packet has not yet landed: the default `daemon_core="python"` path
  is unaffected, and `daemon_core="go"` raises a clear
  `Go daemon source tree is missing at .../go` error rather than
  hanging.

## Open follow-ups for Phase 2 / consolidation

1. Land `tests/test_daemon_go_smoke.py` and the
   `tests/_harness/audit_fixture.py` Python generator (synthesis
   §5.2 / §7.2).
2. Wire `striatum daemon start --core go` into the Python CLI (RFC
   0039 §4 / Step 3) so the harness can stop launching the binary by
   path.
3. Parametrize the existing five RFC 0035 e2e files across
   `daemon_core` once the Go core implements mutating verbs (RFC
   0039 Step 4).
4. Update `docs/rfcs/README.md`, `docs/TODO.md`, and `CHANGELOG.md`
   from the consolidate_phase_1 workflow job (deliberately excluded
   from this packet).
