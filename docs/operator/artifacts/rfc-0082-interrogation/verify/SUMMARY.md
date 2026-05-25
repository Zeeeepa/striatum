---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0082 interrogation sessions — aggregate verification

author: operator

aggregate_status: green

Operator-run aggregate (2026-05-25):

- `cd go && go build ./...`: OK; `go vet ./...`: OK.
- `go test ./...`: 30 packages ok, 0 fail. `go test -race ./...`: 30 ok, 0 fail.
- frontend `npm test`: 35 passed.
- `make python-trace-guardrail` (strict): PASS — blocked=0.
- `go generate ./...` round-trip: clean (contract + generated registry/routes/
  method-tables consistent).

## RFC 0082 Required Tests — all present and PASS (live-PG, STRIATUM_PG_TEST_URL set)

Run: `STRIATUM_PG_TEST_URL=postgres:///... go test ./pkg/mutations -run Interrogation -v`

- (1) `TestInterrogationLifecycle` — PASS
- (2) `TestInterrogationTargetingDeliversOnlyToTarget` — PASS
- (3) `TestInterrogationAuthorization` — PASS
- (4) `TestInterrogationOpenRequiresLiveTarget` — PASS
- (5) await_packet envelope discrimination — covered (claim/await tests)
- (6) `TestInterrogationMultiTurn` — PASS
- (7) **`TestInterrogationEndToEndPreservedContext` — PASS** (the intention bar:
  a reviewer interrogates a builder's PRESERVED context and receives a
  context-aware answer reflecting a builder-only fact never republished).
- (8) `TestInterrogationD028NoRawProviderOutput` — PASS
- (9) `TestMigrationSixteenInterrogationsIsOwnershipSafe` — PASS (migration 0016
  creates a new table + GRANTs the runtime role; no ALTER, no owner-FK).

The implementation matches the RFC 0082 intention: peer-addressed, multi-turn
interrogation of a live worker's preserved context, validated end-to-end.

Note: live-PG interrogation tests `t.Skip` without `STRIATUM_PG_TEST_URL`
(per RFC 0080); CI must set it (RFC 0080 CI wiring) for them to execute. The
new migration 0016 is ownership-safe but, like all schema additions, requires
owner application on the production daemon until RFC 0079 §5 / TODO F34 lands.
