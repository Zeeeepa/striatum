---
schema_version: striatum.finding.v1
artifact_kind: finding
severity: medium
verdict_intent: accept_with_findings
---

# GH #26 Verification Review

author: reviewer-unknown-model-001

Final verdict: `accept_with_findings`

## Scope

Fresh-context review of GH #26 / RFC 0073 — surfacing the RFC 0072 blob
diagnostics block through `striatum daemon doctor`. The verify prompt
(`docs/issues/26/prompts/verify.md`) explicitly directs the reviewer to read
the SPEC, SCOPE, RFC, build HANDOFF, **and the changed files named by the
handoff**, so source inspection is in-scope despite the
`document_only` review policy. The build commit reviewed is `a2b8c05`.

From the compliance/license posture: the change introduces no new
third-party dependencies, no hosted-service calls, no telemetry, no
transcript capture, no external persistence, and no new license headers
or attribution surfaces. Authorship line on the build artifact uses the
required lowercase byline. No unresolved compliance or license issues.

## Acceptance Verification

### DoD-1 — `data.blob` carries `{configured: true, reachable: true, ...}` when `STRIATUM_BLOB_ENDPOINT` is set

Met. The block is sourced from the existing Go
`blobDoctorBlock(...)` (`go/pkg/reads/doctor_blob.go:33-119`, unchanged) via
the new focused RPC handler `HandleDoctorBlobBlock`
(`go/pkg/reads/doctor_blob_handler.go:13-16`), which is registered as
`doctor.blob_block` in `go/pkg/reads/reads.go:115,118`. Python merges the
block at `src/striatum/daemon_pg/client_admin.py:446,469` and surfaces it
at the top-level `data.blob` via
`src/striatum/cli/dispatch.py:1530-1535`. Single source of truth (Go) per
the SCOPE Option A-prime rationale.

### DoD-2 — `data.blob = {"configured": false}` when `STRIATUM_BLOB_ENDPOINT` is unset

Met. `blobDoctorBlock` returns `{"configured": false}` when
`packageBlobClient` is nil (`go/pkg/reads/doctor_blob.go:34-36`), and
the new handler-level Go test
`TestHandleDoctorBlobBlockReturnsNotConfiguredWhenClientMissing`
(`go/pkg/reads/doctor_blob_handler_test.go:10-24`) pins that exact return
through the RPC wrapper.

### DoD-3 — `--repo` block carries `bucket`, `bucket_status`, round-trip fields

Met. The Python merge passes `repository_id` through to
`fetch_blob_doctor_block(repository_id)`
(`src/striatum/daemon_pg/client_admin.py:469`), and the Go side already
implements per-repo bucket lookup, head, and round-trip
(`go/pkg/reads/doctor_blob.go:52-118`). The four documented
`bucket_status` enum values (`ok`, `missing`, `not_provisioned`,
`head_failed`) and the round-trip fields (`round_trip_ms`,
`round_trip_sha256`) are preserved on the wire because the handler
returns the map produced by `blobDoctorBlock` verbatim. The repo-OK
shape is pinned by
`test_daemon_doctor_merges_repo_blob_ok_block_and_formats_summary`
(`tests/cli/test_dispatch_daemon_doctor.py:187-238`).

The `--repo` path is also newly threaded through `_dispatch_daemon`
itself: `args.doctor_repo` now flows into `daemon_admin.read_doctor`
(`src/striatum/cli/dispatch.py:1512,1522`). Before this change the
dispatch always passed `repo=None`, which would have blocked DoD-3
end-to-end even with the merge in place.

### DoD-4 — non-`--json` form prints a one-line blob summary

Met with finding. `_format_blob_doctor_summary`
(`src/striatum/cli/dispatch.py:1629-1652`) handles the
`not configured`, `unreachable`, `configured (..., probe=ok)`, and
intermediate-bucket-status cases. The summary is appended to the result
in non-JSON mode by `_format_daemon_doctor_human`
(`src/striatum/cli/dispatch.py:1620-1626`). See Finding F2 below for the
missing `endpoint=` field and Finding F3 for the layout choice of
emitting the JSON envelope plus summary line in the non-JSON branch.

### DoD-5 — regression test for `configured: false`, `configured: true, reachable: true`, and `method_unknown` fallback

Met.
`test_daemon_doctor_merges_blob_configured_false_and_formats_summary`
(`tests/cli/test_dispatch_daemon_doctor.py:161-184`) pins the unconfigured
path through `_dispatch_daemon` and the summary string `blob: not configured`.
`test_daemon_doctor_merges_repo_blob_ok_block_and_formats_summary`
(`tests/cli/test_dispatch_daemon_doctor.py:187-238`) pins the configured
`--repo` round-trip path. `test_fetch_blob_doctor_block_method_unknown_is_loud`
(`tests/cli/test_dispatch_daemon_doctor.py:241-251`) pins the loud
`method_unknown` fallback at the helper layer. The
`socket unreachable` fallback that the SCOPE specifically flagged as a
risk has the implementation
(`src/striatum/daemon_pg/blob_doctor.py:32-33`) but is not
independently asserted; see Finding F4.

### DoD-6 — `make smoke`, `make pg-test`, `make lint`, `make typecheck`, `go test ./pkg/reads/... ./pkg/blob/...`

Per build handoff (`docs/issues/26/build/HANDOFF.md:14-25`) all six
checks passed on the implementer host; only the pre-existing deprecated
`needs` workflow warnings appeared in `make smoke`. The handoff lists
each command explicitly. The verify environment did not re-execute
these (no fresh PG instance available in this review session); the
handoff's claim is taken at face value, consistent with the
compliance-posture review policy.

## Adversarial Probes

### P1 — Unconfigured path: `data.blob == {"configured": false}` when env unset (and only that)

Passes. The Go probe short-circuits at `doctor_blob.go:34-36` with no
extra keys when `packageBlobClient` is nil; the Python helper passes
that map through unchanged; the merge writes it to `data.blob` at the
top level. The Python `method_unknown` and `socket unreachable`
fallbacks layer additional `errors:` keys onto a `configured` payload
(`blob_doctor.py:25-33`) — that is by design (silent-gap defense) and
does **not** affect the live unconfigured path being probed here.

### P2 — Reachable path: `data.blob.reachable == true`, round-trip completes (or structured error)

Passes. The reachable-with-`--repo` shape is pinned by the test at
`tests/cli/test_dispatch_daemon_doctor.py:187-238`, asserting all four
round-trip fields. Structured-error variants (`head_failed`, `missing`,
`not_provisioned`, `put_failed`, `get_failed`, `round_trip_mismatch`)
are produced by the unchanged Go probe and therefore preserved.

### P3 — Cycle safety: doctor doesn't recurse into doctor

Passes. The Python path
(`client_admin.read_doctor_pg` → `fetch_blob_doctor_block` →
`call_unix("doctor.blob_block")` → `HandleDoctorBlobBlock` →
`blobDoctorBlock`) never re-enters `HandleDoctor` or the Python
`doctor_payload`. The new handler is a thin pass-through:

```go
func HandleDoctorBlobBlock(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
    repositoryID, _ := envelope.Params["repository_id"].(string)
    return blobDoctorBlock(ctx, runner, repositoryID), nil
}
```

(`go/pkg/reads/doctor_blob_handler.go:13-16`). The SCOPE Option A-prime
rationale (avoid calling the full `doctor` RPC) is honored.

### P4 — JSON shape preservation: `daemon_diagnostics` unchanged byte-for-byte, blob lives outside

**Fails** — see Finding F1. The blob block is duplicated: it appears
both at `data.blob` (correct, per RFC 0073 and SCOPE) and at
`data.daemon_diagnostics.blob` (a new key inside `daemon_diagnostics`).
This contradicts SCOPE §4 ("the blob block does NOT go inside
`daemon_diagnostics`") and SPEC adversarial probe 4 ("byte-for-byte
unchanged"). The change is additive and non-destructive, but
shape-snapshot consumers will observe the new key.

## Findings

### F1 — `daemon_diagnostics` shape carries a duplicate `blob` key (medium)

**Where**: `src/striatum/daemon_pg/client_admin.py:446,469` writes
`data["blob"] = fetch_blob_doctor_block(...)` into the dict that becomes
`daemon_diagnostics` on the wire.
`src/striatum/cli/dispatch.py:1530-1533` then *also* lifts that same
block to top-level `result["blob"]`, but does not pop it from
`daemon_diagnostics`.

**Why it matters**: SCOPE §4 explicitly forbids adding `blob` inside
`daemon_diagnostics` ("the blob block does NOT go inside
`daemon_diagnostics` (per RFC 0073: 'the blob block lives at the same
level as `daemon_diagnostics`')"). SPEC adversarial probe 4 reinforces
this as "byte-for-byte unchanged." The implementation duplicates the
block across both levels, breaking the probe.

**Remediation**: Two equivalent fixes; pick one.

1. In `read_doctor_pg`, return the blob block as a sibling rather than
   nesting it. Concretely, change the return shape so the caller
   receives `{"daemon_doctor": {...}, "blob": {...}}` instead of
   merging blob into the dict that becomes `daemon_diagnostics`; then
   `_dispatch_daemon` reads `blob` from that sibling. This is the
   straightforward shape-preserving fix.
2. In `_dispatch_daemon`, after copying the blob to top-level, do
   `daemon_diagnostics.pop("blob", None)` so the inner dict is
   restored to its pre-RFC-0073 shape. Lower-effort but couples
   dispatch to the implementation detail of `read_doctor_pg`.

Either way, update the regression tests to assert that
`result["daemon_diagnostics"]` does **not** contain a `blob` key.

### F2 — DoD-4 endpoint string never appears in the human summary (low)

**Where**: `src/striatum/cli/dispatch.py:1645-1649`. The "ok" branch
emits `blob: configured (endpoint={endpoint}, ...)` only when
`blob.get("endpoint")` is truthy, but `blobDoctorBlock`
(`go/pkg/reads/doctor_blob.go`) never sets an `endpoint` key.

**Why it matters**: SPEC DoD-4 and RFC 0073 both list
`blob: configured (endpoint=..., bucket=..., probe=ok)` as the canonical
ok-path summary. The SCOPE §7.3 risk register flagged this exact gap and
explicitly recommended extending `blobDoctorBlock` with a top-level
`endpoint` string. The implementer took the fallback path
(`blob: configured (bucket=..., probe=ok)` without endpoint) without
extending the Go probe.

**Remediation**: Add a single line in `blobDoctorBlock` after
`report["reachable"] = true` to expose the configured endpoint, e.g.
`report["endpoint"] = packageBlobClient.Endpoint()` (or the equivalent
accessor on the blob client). Then the existing
`_format_blob_doctor_summary` branch at
`dispatch.py:1646-1648` already produces the canonical summary
unchanged. Add an assertion to
`test_daemon_doctor_merges_repo_blob_ok_block_and_formats_summary`
that the summary contains `endpoint=`.

### F3 — Non-JSON output still emits the full JSON envelope plus the summary line (informational)

**Where**: `src/striatum/cli/dispatch.py:1620-1626`.
`_format_daemon_doctor_human` joins
`json_dumps({"ok": True, "data": result})` and the summary line with a
newline, so non-`--json` output is now `{...big JSON object...}\nblob: ...`.

**Why it matters**: DoD-4 says "Non-`--json` form prints a one-line
summary." The pre-RFC behavior was already to dump JSON when result is
a dict (see `dispatch.py:104` — the legacy `print(json_dumps(...))`
branch), so the implementer chose to preserve that and append a
human-readable line. That preserves backward compatibility for scripts
parsing daemon-doctor stdout, but it is not what DoD-4 literally asks
for, and it is not what the SCOPE Verification Commands section
(`docs/issues/26/SCOPE.md:308-332`) expects (those commands `grep -E
'^blob:'` against stderr/stdout — that still works, but the noise above
is non-trivial).

**Remediation**: Either (a) accept this as a deliberate
backward-compat choice and update the SPEC / RFC wording to reflect
that the summary is appended rather than replacing the JSON, or (b)
print the summary line first and the JSON below it, or (c) suppress
the JSON dump in the non-JSON branch for `daemon doctor` only and emit
a fuller human-readable rendering (which is a larger change beyond
this RFC). Option (a) is the cheapest if operators do not complain.

### F4 — `socket unreachable` fallback is implemented but not regression-tested (low)

**Where**: `src/striatum/daemon_pg/blob_doctor.py:32-33`.

**Why it matters**: SCOPE §3 risk 7.3 named "doctor must not hard-fail
when the Go daemon is unreachable" as the primary robustness invariant,
and asked for a sibling test of the `socket unreachable` fallback
alongside `method_unknown`. The `method_unknown` test exists; the
socket-unreachable one does not. Without it, a future refactor of
`_call_with_handshake` could silently let the `OSError` propagate and
reintroduce the silent-gap pattern RFC 0073 was opened to close.

**Remediation**: Add a third helper-level test that monkeypatches
`_call_with_handshake` to raise `OSError("connection refused")` and
asserts the returned block is
`{"configured": False, "errors": ["daemon socket unreachable: connection refused"]}`.

### F5 — Scope-requested doc updates omitted (low, process)

**Where**: `docs/issues/26/build/HANDOFF.md:27-31` reports that
DECISION_LOG.md, CLI_REFERENCE.md, and CHANGELOG.md updates listed in
SCOPE §3 were skipped because the work-packet write scope did not allow
those paths.

**Why it matters**: The decision-log entry (Option A-prime choice with
focused `reads.doctor_blob_block` sub-method) is the durable
provenance record SCOPE §3 asked for, and the CHANGELOG entry is part
of the release-discipline contract (RFC bump → CHANGELOG entry). This
is a triage-vs-write-scope misalignment, not an implementer fault.

**Remediation**: A follow-up packet with `allowed_paths` extended to
`docs/DECISION_LOG.md`, `docs/CLI_REFERENCE.md`, and `CHANGELOG.md`
captures the missing entries. No new code work required.

## Test/Verification Assessment

The three new regression tests are well-scoped:

- The `configured: false` test exercises the full `_dispatch_daemon`
  path and asserts both the JSON merge **and** the human-readable
  summary string. Good integration coverage.
- The `--repo` reachable test asserts each round-trip field
  (`bucket`, `bucket_status`, `round_trip_ms`, `round_trip_sha256`)
  and the summary string. Note that this test's summary assertion
  (`blob: configured (bucket=striatum-repo-1, probe=ok)`) implicitly
  encodes the missing `endpoint=` gap from F2 — once F2 is remediated
  the assertion will need to expand to include the endpoint.
- The `method_unknown` helper test is targeted at the right
  abstraction (the helper, not dispatch); good. It would benefit from
  a sibling for `socket unreachable` (F4).

The new Go handler test
(`go/pkg/reads/doctor_blob_handler_test.go`) is the minimum useful
shape — one not-configured path plus the registry check. The deeper
S3 round-trip coverage continues to live in
`go/pkg/reads/doctor_test.go` and `go/pkg/blob/config_test.go`, and the
RPC wrapper does not need to duplicate it.

## Compliance / License Posture Summary

No new third-party imports, no hosted-service or telemetry endpoints,
no transcript capture, no external persistence. Authorship lines on
the build artifact use the lowercase byline. All edits land in
existing project-owned files. There are no unresolved license,
attribution, or compliance issues, and the artifact passes the
compliance-posture acceptance threshold.

The functional findings above are tracked as accept-with-findings
rather than reject because the live-path behavior of DoD-1, DoD-2,
DoD-3, and the regression suite is correct, and the duplicate-key
shape regression (F1) and missing endpoint (F2) are mechanical
follow-ups that do not invalidate the silent-gap defense RFC 0073
was opened to provide.
