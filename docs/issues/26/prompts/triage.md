# Triage -- GH #26 scope

You are the triager for this issue workflow. Produce only the declared
scope artifact for this workflow. Do not implement source changes.

## Read

1. `docs/issues/26/SPEC.md`
2. `docs/rfcs/0073-daemon-doctor-blob-parity.md` — the proposing RFC.
3. `go/pkg/reads/doctor.go` and `go/pkg/reads/doctor_blob.go` — the
   existing Go-side blob diagnostics that should reach the operator.
4. `src/striatum/daemon_pg/handlers/reads/doctor.py:doctor_payload` —
   the Python handler that produces `daemon_diagnostics`. Decide where
   the blob block merges in.
5. `src/striatum/daemon_pg/client_admin.py:read_doctor_pg` — the call
   path from `striatum daemon doctor`.
6. `src/striatum/daemon_rpc/client.py` — the helpers Python uses to
   call into the Go daemon (relevant only if Option A is chosen).
7. `tests/cli/test_dispatch_daemon_doctor.py` — the regression-test
   surface to extend.

## Output

Write `docs/issues/26/SCOPE.md` with `striatum.synthesis.v1` front matter
and the exact `author:` line from the work packet. Include:

- the chosen option (A: Python delegates to Go via sub-RPC; B: Python
  computes blob block independently) with justification rooted in the
  call-path inspection, not just the RFC's pros/cons;
- the exact files in scope (Python doctor handler, possibly a new Go
  `reads.doctor_blob_block` if Option A; tests; CLI rendering if the
  non-JSON path needs updating);
- the exact files out of scope (do NOT touch the Go `HandleDoctor`
  beyond adding the sub-method endpoint if Option A; do NOT change
  the daemon_diagnostics shape — the blob block lives next to it,
  not inside it);
- an acceptance checklist with one numbered check per "Acceptance /
  Definition of done" bullet in `docs/issues/26/SPEC.md`;
- verification commands (`make smoke`, `make pg-test`, a manual
  `daemon doctor` round-trip with and without `--repo`, before and
  after `STRIATUM_BLOB_ENDPOINT` is unset);
- risks and conflicts with parallel issue workflows (#25 touches
  `src/striatum/cli/`, #27 touches PG migrations / artifacts trigger —
  no expected overlap with this one, which is squarely in
  `daemon_pg/handlers/reads/`).
