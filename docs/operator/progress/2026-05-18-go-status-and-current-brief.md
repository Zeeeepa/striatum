---
schema_version: "striatum.progress_note.v1"
artifact_kind: "progress_note"
note_date: "2026-05-18"
session_slug: "go-status-and-current-brief"
related_plan: "plan_rfc-0069-pg-only-daemon-global-surfaces"
related_brief: "brief_2026-05-17_go-daemon-remediation"
retrieval_priority: "normal"
---

# Go Status and Current Brief
author: coordinator-codex-gpt-5.5-001

Go `status` now returns the PostgreSQL/Python read-model shape for jobs,
verdict posture counts, claimable queue messages, blockers, process health,
supervisor stalls, phase progress, provenance mode, auto-finalize dry-run
visibility, and next actions.

RFC 0058 V1.5 also landed: `striatum operator current-brief` reads the
current operator brief locally, and `operator_brief` context-budget overruns
are schema errors.

Follow-up diagnostic cleanup: `daemon status --json` now reports PostgreSQL
migration privilege failures as structured CLI errors with repair hints, and
`daemon doctor --postgres-url` threads that explicit URL into secondary daemon
diagnostics instead of probing implicit legacy registry configuration.

Go `daemon.key.rotate` now owns the local Ed25519 fallback signing-key
rotation path and `daemon.hello` advertises the fallback public key when the
key is loadable. D112 keeps full reviewed-patch apply mutation and OS keyring
custody outside this slice by removing `apply.reviewed_patch` from the
production daemon RPC contract; stale calls audit as `method_unknown`.

Web and chat workflow-generation preview now use the daemon RPC
`workflow.generate.preview` route in production and keep the in-process
generator only for the explicit test-harness fallback.

RFC 0070 client-boundary cleanup also fenced the remaining production
`cross-repo` CLI direct-PostgreSQL fallback. If daemon RPC routing does not
handle a cross-repo command, production dispatch now fails closed instead of
opening daemon PostgreSQL directly.

RFC 0056 Phase B landed as `init --with-striatum-layout`: an opt-in
directory-only scaffold for `striatum/workflows/` and
`striatum/<workflow-slug>/`. It intentionally leaves workflow-file generation
and artifact-root `.gitignore` policy out of scope.

Go `cross_repo.cancel` parity now matches the Python participant-cancel
semantics for terminal participants, preparing participants without local run
ids, inactive participant repositories, and blocked-error details persisted to
`last_reconcile_error`. Missing cross-repo run ids now return the typed
daemon RPC `not_found` error instead of a plain internal error. The CORE=go
Unix-socket conformance suite now seeds live PostgreSQL cross-repo state,
calls `cross_repo.cancel`, and verifies the mixed canceled/blocked response,
stored participant/run state, and audit row.

Go `workflow.upgrade --add-phases` now matches the Python V1-to-V1.1
phase-inference path for preview/apply, synthesis-job insertion, cross-phase
edge rewriting, and non-terminal-run refusal.

Go `workflow.generate --shape multi_phase` now also matches the Python V1.1
generator path for ordered phases, per-track job remapping,
`phase_synthesis` gates, and cross-phase synthesis-to-entry edges.

RFC 0071's authority-matrix path is now settled by D108: keep the matrix
curated for authority/status classification, and enforce generated CLI route
labels plus runtime CLI fallback cells through architecture tests.
`daemon doctor --repo <path> --authority --json` now also mirrors the
verify-only repository cutover report and summarizes it in
`striatum.authority_report.v1`.

Phase 4 service cleanup removed the eager primary-service import of the
legacy `striatum.api` wrapper. The compatibility `invoke()` seam lazy-loads
that legacy wrapper only when explicitly called.
