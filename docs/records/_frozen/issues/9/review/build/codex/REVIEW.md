---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
tags: ["security", "csrf", "origin", "content-type", "dry-run", "rfc-0050"]
---

author: reviewer-unknown-model-001

# Build Security Review -- GH #9-#11

**Logical Name:** build_review_codex
**Posture:** security
**Artifact:** `docs/issues/9/review/build/codex/REVIEW.md`

## Verdict

**accept.** I reviewed the GH #9-#11 security-hardening implementation for
CSRF, Origin/Referer, Content-Type, override-verdict context binding, and
`recovery auto-publish --dry-run` side effects. I found no actionable security
gaps in the implemented scope.

## Issue Acceptance Checklist

- **GH #9 -- CSRF on `/v1/invoke`: accepted.**
  Strict Content-Type parsing is centralized in
  `src/striatum/service.py:1022` and is enforced by both JSON body helpers at
  `src/striatum/service.py:2596` and `src/striatum/service.py:3511`, returning
  `415` before body parsing for non-JSON media types. Same-origin enforcement
  runs before POST dispatch at `src/striatum/service.py:1321`; `/v1/invoke`
  remains covered even when `--web` is off via
  `src/striatum/service.py:3392`. The Origin/Referer gate fails closed for
  missing, `null`, malformed, cross-origin, and DNS-rebinding-shaped requests,
  while allowing valid Bearer-token API clients at
  `src/striatum/service.py:3397`.
- **GH #10 -- override modal DOM tampering: accepted.**
  The server mints a process-local token bound to
  `(run_id, job_id, session_id)` at `src/striatum/service.py:1126` and renders
  it into the job page at `src/striatum/service.py:1907` and
  `src/striatum/web/templates/job_detail.html:61`. The client parses the URL
  context at `src/striatum/web/static/override_verdict.js:56`, includes
  `web_context` in the `/v1/invoke` body at
  `src/striatum/web/static/override_verdict.js:62`, and refuses DOM/URL
  mismatch before submit at `src/striatum/web/static/override_verdict.js:171`
  and `src/striatum/web/static/override_verdict.js:225`. The server rejects
  missing, malformed, mismatched, or forged override context at
  `src/striatum/service.py:3449`.
- **GH #11 -- dry-run no side effects: accepted.**
  `auto_publish_stale_artifacts` keeps `expire_leases` out of the dry-run path
  at `src/striatum/cli/recovery.py:760`, queries wall-clock-stale active leases
  without mutating them at `src/striatum/cli/recovery.py:770`, returns preview
  rows before any publish/complete/event helper is reachable at
  `src/striatum/cli/recovery.py:884`, and gates `maybe_complete_run` behind the
  live path at `src/striatum/cli/recovery.py:971`.

## Verification

- `pytest tests/test_invoke_csrf_refused.py tests/test_override_modal_context_validation.py tests/test_recovery_dry_run_no_side_effects.py tests/test_override_modal_payload.py` -- **32 passed**.
- `pytest tests/test_recovery_panel_dry_run.py tests/test_recovery_auto.py tests/test_recovery_extended.py tests/test_override_rationale_regression.py tests/test_service.py tests/test_web_chat.py tests/test_web_workflow_edit.py tests/test_web_workflow_run.py tests/test_copy_on_click.py` -- **95 passed**.
- `make lint` -- **passed**.
- `make typecheck` -- **passed** with 216 source files checked.

## Findings

No findings.
