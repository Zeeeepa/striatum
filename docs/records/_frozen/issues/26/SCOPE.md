---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/26/SPEC.md", "docs/rfcs/0073-daemon-doctor-blob-parity.md", "docs/ROADMAP.md", "docs/TODO.md", "docs/DECISION_LOG.md", "docs/POSTGRES_TRANSITION.md", "docs/BLOB_TRANSITION.md", "docs/SPEC.md", "docs/INDEX.md", "AGENTS.md"]
---

author: triager-unknown-model-001

# GH #26 - SCOPE

Bound scope for GH #26, "surface RFC 0072 blob diagnostics through
`striatum daemon doctor`."

The implementation must add a `data.blob` block to the
`daemon doctor` response (JSON and non-JSON), sourcing it from the Go
daemon's existing `blobDoctorBlock` rather than duplicating S3 probe
logic in Python. The block lives next to `data.daemon_diagnostics`,
not inside it, and the existing `daemon_diagnostics` shape is not
modified.

This is a narrow doctor-visibility fix that closes the silent-gap
pattern from the GH #22/#23/#24 operator session. It is not a
broader unification of the Go / Python doctor handler split (RFC
0048 follow-on territory, called out as non-goal in RFC 0073).

## 1. Issue covered

- GH #26 / RFC 0073 -- `daemon doctor` does not surface the blob
  block that the Go daemon's `HandleDoctor` already computes
  (`go/pkg/reads/doctor.go:70`, `go/pkg/reads/doctor_blob.go:33-119`).
  Operators inspecting `--json` see `data.blob == null` even on a
  daemon configured with `STRIATUM_BLOB_ENDPOINT`.

## 2. Chosen approach -- Option A (focused sub-method)

Pick **Option A** with a focused sub-method (Option A-prime). The
Python doctor handler does not call into the Go daemon today; instead
the CLI's `striatum daemon doctor` calls Python `doctor_payload`
in-process via `daemon_admin.read_doctor` ->
`daemon_admin.read_doctor_pg`. For the blob block, route a single
extra RPC call through `daemon_rpc.client.call_unix` to a new focused
Go method `reads.doctor_blob_block` that returns only the
`blobDoctorBlock(ctx, runner, repository_id)` map. Merge the response
into the dispatch result under the `blob` key so it surfaces at
`.data.blob`.

### 2.1 Why Option A over Option B (rooted in call-path inspection)

1. **`doctor_payload` runs in the CLI process today, not in the
   daemon process.** `src/striatum/daemon_pg/client_admin.py:421-471`
   opens its own PG connection and invokes `doctor_payload(ctx,...)`
   directly. There is no "doctor calls doctor inside the daemon"
   cycle for the CLI path -- a sub-RPC from `read_doctor_pg` to the
   Go daemon's blob block is just CLI -> daemon, the same shape as
   any other read RPC.
2. **`blobDoctorBlock` is non-trivial and lives on the Go side
   already.** It owns: the `packageBlobClient` singleton wired at
   daemon startup (`go/pkg/reads/reads.go:102-114`), the
   `Reachable` / `BucketExists` / `PutBytes` / `GetBytes` round
   trip (`go/pkg/reads/doctor_blob.go:44-118`), a fixed probe key,
   and four distinct `bucket_status` codes (`ok`, `missing`,
   `not_provisioned`, `head_failed`, plus internal `put_failed`,
   `get_failed`, `round_trip_mismatch`). Re-implementing this in
   Python would duplicate a substantial blob-probe surface that is
   already covered by Go tests (`go/pkg/reads/doctor_test.go`,
   `go/pkg/blob/config_test.go`).
3. **Python has no in-process equivalent of `packageBlobClient`.**
   Option B would require either (a) adding a new `daemon.config`
   RPC just to read the blob env vars back out of the daemon
   process, or (b) re-reading `STRIATUM_BLOB_ENDPOINT` and friends
   from the CLI process environment -- which is wrong because the
   CLI and daemon are different processes with different envs (the
   exact mismatch RFC 0073 calls out: today operators have to
   `cat /proc/<pid>/environ`). Either way we end up with a new RPC,
   so the cheap path is to add the focused `reads.doctor_blob_block`
   RPC and let Go's already-configured `packageBlobClient` answer.
4. **The "doctor calls doctor cycle risk" RFC 0073 names is bounded
   by using a focused sub-method.** Calling the full `doctor` RPC
   would (eventually, when the Python handler is itself ported into
   the daemon under RFC 0048) reach back into `HandleDoctor`, which
   itself calls `blobDoctorBlock` -- one extra round, not a true
   loop, but conceptually muddy. A focused
   `reads.doctor_blob_block` (returns ONLY the map produced by
   `blobDoctorBlock`, no schema_version / problems / stale_leases
   noise) makes the call shape unambiguous and cycle-free even
   under future RFC 0048 unification.
5. **Single source of truth.** When the bucket-status enum or
   probe semantics evolve (e.g., adding multipart probes,
   per-region buckets), one Go function changes and both the Go
   `HandleDoctor` response and the Python-CLI-merged `data.blob`
   update together. Option B would have to chase every change in
   two places.

### 2.2 Out-of-scope variants

- **Adding a separate `striatum daemon blob-status` verb.** Rejected
  by RFC 0073 non-goals; the information belongs in `daemon doctor`
  next to the rest of the readiness state.
- **Reorganising the Go/Python doctor split (RFC 0048 follow-on).**
  Explicitly out of scope per RFC 0073 non-goals. Keep
  `daemon_diagnostics` shape unchanged; add `blob` as a sibling key.
- **Routing `daemon doctor` through `daemon_rpc_route.try_route`
  end-to-end.** That would replace `read_doctor_pg` with a full
  RPC dispatch and is a separate, larger change. The narrow fix is
  one sub-RPC for the blob block only.

## 3. Files in scope

The implementer may edit only the files needed to add the
`reads.doctor_blob_block` Go method, the Python merge point, the
CLI human-readable line, and focused tests.

- **ADD** `go/pkg/reads/doctor_blob_handler.go` (or extend
  `go/pkg/reads/doctor_blob.go`) -- a thin `HandleDoctorBlobBlock`
  that pulls `repository_id` out of the envelope (empty string when
  absent), invokes `blobDoctorBlock(ctx, runner, repositoryID)`,
  and returns the resulting `map[string]any` as the RPC response.
  No new logic; the function exists solely to expose
  `blobDoctorBlock` over RPC without dragging the rest of
  `HandleDoctor` along.
- **EDIT** `go/pkg/reads/reads.go` -- register the new handler:
  `server.Register("doctor.blob_block", makeHandler(runner,
  HandleDoctorBlobBlock))`. Keep the existing `doctor`
  registration unchanged.
- **EDIT** `src/striatum/daemon_pg/handlers/reads/doctor.py` --
  do not touch `doctor_payload`'s `daemon_diagnostics` shape. The
  blob merge happens one layer up, in `read_doctor_pg`, so this
  handler stays focused on audit-chain / supervisor invariants.
- **EDIT** `src/striatum/daemon_pg/client_admin.py:read_doctor_pg`
  (lines 421-471) -- after computing `data = doctor_payload(...)`
  and before returning, call a new helper
  `fetch_blob_doctor_block(repository_id)` that opens a Unix
  socket via `daemon_rpc.client.call_unix`, sends a `doctor.blob_block`
  RPC envelope, and returns the response payload. Merge as
  `data["blob"] = block`. Apply to both branches: the no-`--repo`
  branch (call with empty `repository_id`) AND the
  with-`--repo` branch (pass `repository_id`).
- **ADD** `src/striatum/daemon_pg/blob_doctor.py` (or extend an
  existing `daemon_pg/blob_*.py`) -- module that owns the
  `fetch_blob_doctor_block` helper. It must:
  - resolve the daemon socket path via the existing
    `daemon_required.resolve_socket_path` (or the equivalent
    runtime-dir lookup used by the rest of the CLI);
  - tolerate "socket not reachable" by returning
    `{"configured": false, "errors": ["daemon socket unreachable: <err>"]}`
    so `daemon doctor` does not hard-fail when the Go daemon is
    down (the rest of the doctor output should still render);
  - tolerate `method_unknown` from older daemon binaries by
    returning `{"configured": null, "errors": ["daemon binary predates RFC 0073"]}`
    -- this is the exact silent-gap pattern RFC 0073 is preventing,
    and it must surface loudly when the operator's daemon binary
    is too old.
- **EDIT** `src/striatum/cli/dispatch.py` (around lines 1461-1510
  where `_dispatch_daemon` assembles the `result` dict) -- after
  `daemon_diagnostics` is set, pull `result["daemon_diagnostics"]
  ["blob"]` (or the equivalent merge point from
  `read_doctor_pg`'s return value) and emit a one-line summary in
  the non-`--json` path. The non-JSON summary lives wherever
  `daemon doctor` already renders its human text; the implementer
  must locate that formatter (likely
  `src/striatum/cli/formatters/daemon.py` or inline in
  `_dispatch_daemon`'s `--json=False` branch) and add:
  - `blob: not configured` when `configured == false`;
  - `blob: unreachable: <first error>` when
    `configured == true and reachable == false`;
  - `blob: configured (endpoint=..., bucket=..., probe=ok)` when
    `configured == true and reachable == true and bucket_status == "ok"`;
  - `blob: configured (bucket=<x>, status=<s>)` for the
    intermediate states (`missing`, `not_provisioned`, `head_failed`).
  Endpoint string comes from the existing block (add to
  `blobDoctorBlock` only if not already exposed -- check first).
- **EDIT** `tests/cli/test_dispatch_daemon_doctor.py` -- add at
  least three regression tests pinning the new shape:
  1. `configured: false` path -- monkeypatch
     `fetch_blob_doctor_block` to return
     `{"configured": False}` and assert
     `result["daemon_diagnostics"]["blob"] == {"configured": False}`
     (or whatever the final merge key is) AND that the non-JSON
     formatter emits `blob: not configured`.
  2. `configured: true, reachable: true` path with `--repo` and
     `bucket_status: "ok"` -- assert the block carries `bucket`,
     `bucket_status`, `round_trip_ms`, `round_trip_sha256`.
  3. `method_unknown` fallback -- monkeypatch `call_unix` (or the
     wrapping helper) to raise the `RpcError("method_unknown",...)`
     shape and assert the merged block is the loud-fallback
     variant from the helper, not silent omission.
- **ADD** `go/pkg/reads/doctor_blob_handler_test.go` -- pin that
  `HandleDoctorBlobBlock` with `repository_id == ""` returns
  `{"configured": false}` when `packageBlobClient` is nil, and
  that with a configured client it includes `reachable`. (The
  deep S3 round trip is already covered by existing
  `doctor_test.go` -- this new test only verifies the thin
  handler wrapper.)
- **EDIT** `docs/DECISION_LOG.md` -- add a one-row decision entry
  recording "Option A with focused `reads.doctor_blob_block`
  sub-method; Python merges block into `data.blob` via one RPC
  round-trip." Allocate a new D-id.
- **EDIT** `docs/CLI_REFERENCE.md` only if it documents the
  `daemon doctor` JSON shape; add the new `data.blob` field and
  its three-state variants.
- **EDIT** `CHANGELOG.md` under Unreleased -- one bullet citing
  RFC 0073 and GH #26.

## 4. Files and directories out of scope

The implementer must not edit:

- `go/pkg/reads/doctor.go` `HandleDoctor` itself, beyond
  registering the new sibling method in `reads.go`. The existing
  shape (`schema_version`, `stale_leases`, `waiting_human`,
  `problems`, `blob`) is preserved; we do not move or rename
  fields. The blob block stays present on `HandleDoctor` for any
  caller that reads it directly.
- `go/pkg/reads/doctor_blob.go` `blobDoctorBlock` -- the probe
  logic is reused unchanged. Only ADD a thin handler wrapper.
- `src/striatum/daemon_pg/handlers/reads/doctor.py` -- the Python
  `doctor_payload` shape (`ok`, `schema_version`, `problems`,
  `problem_records`) stays as-is. The blob block does NOT go
  inside `daemon_diagnostics` (per RFC 0073: "the blob block
  lives at the same level as `daemon_diagnostics`").
- `src/striatum/daemon_pg/connection.py`, `roles.py`, and
  `repo_cutover_report.py` -- no schema, role, or cutover work.
- `src/striatum/daemon_pg/sql/` and `go/pkg/db/sql/` -- no new
  migrations. The `striatumd.repositories.blob_bucket` column
  already exists (RFC 0072 step 4) and is read by the existing
  `lookupRepoBlobBucketRead`.
- `go/cmd/striatumd/main.go` and `go/pkg/striatumd/` -- no
  daemon-process bootstrap changes; the existing
  `STRIATUM_BLOB_ENDPOINT` wiring to `packageBlobClient` is
  sufficient.
- `src/striatum/cli/parser.py` for new flags -- the SPEC is
  achievable without new CLI flags; `daemon doctor` and
  `daemon doctor --repo` are the existing surfaces.
- `src/striatum/legacy_sqlite/` -- no SQLite work.
- `src/striatum/corpus/`, `src/striatum/workflow_*`, MCP
  surfaces, web UI, recovery sweepers -- unrelated.
- `docs/rfcs/0072-blob-backed-artifact-storage.md` and other
  RFCs -- preserve provenance.
- `.striatum/`, `.venv/`, caches, build output, transcripts,
  private diagnostics, and example fixtures unrelated to blob
  doctor.

## 5. Acceptance checklist

The verify job should cite each ID below. One numbered check per
bullet in SPEC "Acceptance / Definition of done" (which is itself
pinned to RFC 0073 § Acceptance).

- [DoD-1] `striatum daemon doctor --json` on a daemon with
  `STRIATUM_BLOB_ENDPOINT` set returns
  `.data.blob == {"configured": true, "reachable": true, ...}`
  (or the unreachable variant
  `{"configured": true, "reachable": false, "errors": [...]}`).
  The block is sourced via the Go `reads.doctor_blob_block` RPC,
  not re-derived in Python.
- [DoD-2] `striatum daemon doctor --json` on a daemon WITHOUT
  `STRIATUM_BLOB_ENDPOINT` set returns
  `.data.blob == {"configured": false}`.
- [DoD-3] `striatum daemon doctor --json --repo <registered>` on
  a reachable-blob daemon returns a `.data.blob` block that ALSO
  carries `bucket` (string), `bucket_status` (one of `ok`,
  `missing`, `not_provisioned`, `head_failed`), and -- when
  `bucket_status == "ok"` -- the round-trip fields
  (`round_trip_ms`, `round_trip_sha256`). The per-repo lookup
  uses the existing `striatumd.repositories.blob_bucket` column,
  not any new schema.
- [DoD-4] `striatum daemon doctor` (no `--json`) prints exactly
  one human-readable summary line for the blob block:
  `blob: not configured`, `blob: unreachable: <error>`, or
  `blob: configured (endpoint=..., bucket=..., probe=ok)` for the
  ok path. Intermediate bucket statuses surface their status
  string (e.g., `blob: configured (bucket=<x>, status=missing)`).
  The line appears in the same block as the existing
  `daemon_diagnostics` summary, not nested inside it.
- [DoD-5] A regression test in
  `tests/cli/test_dispatch_daemon_doctor.py` (or a sibling test
  module) pins both the `configured: false` and the
  `configured: true, reachable: true` paths, and additionally
  pins the `method_unknown` fallback message so future-old-binary
  daemons do not silently drop the block.
- [DoD-6] `make smoke` and `make pg-test` still pass on the
  implementer's host. `make lint` and `make typecheck` pass.
  The Go test target `cd go && go test ./pkg/reads/...` also
  passes (covers the new handler wrapper test).

## 6. Verification commands

Run these at minimum:

```bash
make lint
make typecheck
pytest tests/cli/test_dispatch_daemon_doctor.py
cd go && go test ./pkg/reads/... ./pkg/blob/...
```

If PostgreSQL is available, also run:

```bash
make pg-test
make smoke
```

Manual verification (with blob endpoint configured -- e.g., the
local Garage from BLOB_TRANSITION.md):

```bash
# 1. With STRIATUM_BLOB_ENDPOINT set on the daemon, no --repo:
striatum daemon doctor --json | jq '.data.blob'
# Expect: {"configured": true, "reachable": true, "errors": []}
striatum daemon doctor 2>&1 | grep -E '^blob:'
# Expect: blob: configured (endpoint=..., probe=ok)

# 2. With --repo on a registered repository that has blob_bucket
#    provisioned:
striatum daemon doctor --json --repo "$PWD" | jq '.data.blob'
# Expect: {"configured": true, "reachable": true,
#          "bucket": "<name>", "bucket_status": "ok",
#          "round_trip_ms": <int>, "round_trip_sha256": "<sha>"}

# 3. With --repo on a registered repository WITHOUT blob_bucket
#    (operator has not run adopt --apply-blob-creation):
striatum daemon doctor --json --repo "$PWD" | jq '.data.blob'
# Expect: bucket: null, bucket_status: "not_provisioned"

# 4. Unset STRIATUM_BLOB_ENDPOINT on the daemon (restart) and
#    rerun without --repo:
striatum daemon doctor --json | jq '.data.blob'
# Expect: {"configured": false}
striatum daemon doctor 2>&1 | grep -E '^blob:'
# Expect: blob: not configured
```

## 7. Risks and likely conflicts

### 7.1 Parallel issue workflows

- **GH #25** -- repo list misleading `repo_not_migrated`. Edits
  live in `src/striatum/cli/` (repo list path). The only file
  this workflow also touches in `src/striatum/cli/` is
  `dispatch.py` (for the non-JSON formatter line). Even there,
  #25's edits are in the `repo` subcommand dispatch / formatter,
  not the `daemon doctor` dispatch around lines 1461-1510. Risk
  of merge conflict is **low**; mechanical resolution at worst.
- **GH #27** -- artifacts trigger / blob backfill. Edits live in
  `src/striatum/daemon_pg/sql/` (new migration) and
  `go/pkg/db/sql/`. This workflow does NOT touch any SQL or
  migration files. No expected overlap.
- **No expected overlap with `go/pkg/reads/doctor.go`** -- this
  workflow only registers a new sibling method in
  `go/pkg/reads/reads.go` and adds a new handler file. Other
  workflows are not editing the `reads` package.

### 7.2 Recommended order

This workflow (#26) can ship **independently and in parallel**
with #25 and #27. There is no dependency:

- #25 is CLI repo-list formatter; orthogonal to daemon doctor.
- #27 is artifacts-trigger / backfill on the publish path;
  orthogonal to the doctor read path.
- #26's own dependency (RFC 0072 step 5 -- the
  `blobDoctorBlock` function and the Go `HandleDoctor`
  registration) is already landed.

### 7.3 Implementation risks

- **Doctor must not hard-fail when the Go daemon is unreachable.**
  The legacy `striatum daemon doctor` path is meant to work even
  when the daemon is down (it reports PG state, SQLite registry
  state, etc.). The new `fetch_blob_doctor_block` helper MUST
  catch socket errors and return a structured fallback rather
  than raise. The DoD-5 test for `method_unknown` covers this
  shape; add a sibling test for `socket unreachable` to keep
  the doctor robust.
- **Older daemon binaries (predating this workflow) will not
  recognise `reads.doctor_blob_block`.** They will return
  `method_unknown`. The helper must surface this loudly (e.g.,
  `{"configured": null, "errors": ["daemon binary predates RFC
  0073; rebuild and restart daemon"]}`) -- otherwise the
  silent-gap pattern RFC 0073 is closing reappears under
  version-skew. Pin this with the DoD-5 test.
- **Endpoint field exposure.** `blobDoctorBlock` today does not
  emit the configured endpoint as a top-level key; it embeds it
  only inside `errors`. The non-JSON one-liner
  (`blob: configured (endpoint=..., bucket=..., probe=ok)`) needs
  the endpoint. Either extend `blobDoctorBlock` to add an
  `endpoint` string field (single-line addition; no shape break)
  OR fetch the endpoint via the existing Go-side blob config
  surface. The cheaper path is the former; document it in the
  decision-log entry.
- **Tests that today assert the full `daemon doctor` payload
  shape may need updating** if they pin the absence of `blob`.
  Audit `tests/cli/test_dispatch_daemon_doctor.py` and any
  fixtures under `tests/integration/` that snapshot the doctor
  payload. The change is additive (`blob` key); existing
  consumers should keep working, but exact-shape snapshot tests
  will need a refreshed expectation.
- **Sub-RPC latency.** A blob round-trip probe under
  `--repo` performs an S3 PUT+GET. On a slow blob backend this
  adds the probe latency to every `daemon doctor --repo` call.
  This is the existing latency of `HandleDoctor` and is not new;
  document it in the decision-log entry if it affects operator
  runbooks.
- **RFC 0048 future unification.** If/when `doctor_payload` is
  ported into the Go daemon, the focused
  `reads.doctor_blob_block` keeps the merge point unambiguous:
  the ported handler can call `blobDoctorBlock` directly
  (in-process) and the Python CLI helper becomes a thin
  delegation to the unified `doctor` RPC. No churn to the
  on-wire `data.blob` shape.
