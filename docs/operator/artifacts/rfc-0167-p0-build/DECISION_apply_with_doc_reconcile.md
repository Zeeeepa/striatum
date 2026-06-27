---
schema_version: striatum.decision.v1
decision_id: "DECISION-rfc-0167-p0-build-doc-reconcile"
run_id: "run_a4c3e73e4f7fca11826ba96b7823f4e3"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "Accept RFC 0167 P0 build; reconcile spec.md/D263 to state the A45 CLI rewire is implemented in P0 (not follow-up)"
created_at: "2026-06-24T17:16:37Z"
---

# Accept RFC 0167 P0 build; reconcile spec.md/D263 to state the A45 CLI rewire is implemented in P0 (not follow-up)

Decision ID: `DECISION-rfc-0167-p0-build-doc-reconcile`
Run ID: `run_a4c3e73e4f7fca11826ba96b7823f4e3`
Outcome: `accepted_with_follow_up`

## Rationale

The 2nd-cycle reviewer needs_revision is VALID but narrow: a doc/code consistency gap, not a code defect. The implemented A45 / RFC0167 §F F-2 operator-bootstrap CLI rewire (go/cmd/striatum/operator_bootstrap.go now mints + presents the session-bound operator token, raw token written 0600, never embedded) is correctly IN P0 scope per SPEC §9 item 4 + A45. The contradiction the reviewer flagged is that docs/reference/spec.md still describes `operator bootstrap` as a read-only local composite and decision-log D263 says the CLI rewire remains follow-up, while the code already did the rewire. Implementation is otherwise GREEN: cd go && go build ./... + go vet ./... pass; the non-PG owner-bundle/reservations/read-authority/routes/CLI suites + make check-docs pass; the PG two-role pgtests skip cleanly without STRIATUM_PG_TEST_URL and run live in the verify stage (the authoritative gate). The code_change single revision cycle is exhausted, so this routes to the operator. Override (not continue, which re-runs the reviewer) to proceed to apply, accepting the verdict as superseded by this decision.

## Follow-Up

Reconcile docs to match the implemented A45 rewire BEFORE integration to main: (1) docs/reference/spec.md — `operator bootstrap` is no longer read-only; it mints + presents the session-bound operator token (raw token 0600, never embedded). (2) decision-log D263 — the operator-bootstrap CLI rewire is IMPLEMENTED in RFC 0167 P0, not deferred. The apply lane folds this in; the operator verifies spec.md/D263 consistency (and the verify stage's sealed pgtests prove the security properties) before the build integrates.
