---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["security", "csrf", "origin", "content-type", "dry-run", "rfc-0050"]
---

author: reviewer-unknown-model-001

# Build Security Review -- GH #9-#11

**Logical Name:** build_review_codex
**Posture:** security
**Artifact:** `docs/issues/9/review/build/codex/REVIEW.md`

## Executive Summary

I reviewed the supplied issue specs and roadmap/TODO context for the RFC 0050 V2
security-hardening build scope. The GH #9-#11 issues identify the right attack
surface, but the mitigation contract is still loose enough that an implementation
could satisfy the written scope while leaving CSRF exposure or weakening audit
semantics. Verdict: **needs_revision**.

## Finding 1: Origin/Referer Enforcement Is Not Specified As Fail-Closed

- **Severity:** High
- **Scope:** GH #9 / `/v1/invoke`
- **Issue:** The mitigation text says to add "Origin / Referer enforcement" for
  non-GET requests when `web_enabled` is true, but it does not require a
  fail-closed policy for missing, `null`, malformed, or cross-origin headers.
  It also leaves room to skip the check when the local service exposes
  `/v1/invoke` without the web UI enabled.
- **Why this matters:** Content-Type validation blocks simple `text/plain` form
  CSRF, but the origin check is the second boundary for every browser-addressable
  mutating request. If missing or ambiguous Origin/Referer values are accepted,
  or if `/v1/invoke` is exempt outside `web_enabled`, the hardening can still
  leave a cross-site mutation path.
- **Required revision:** Specify and test that every unauthenticated non-GET
  request to `/v1/invoke` rejects absent, `null`, malformed, and cross-origin
  Origin/Referer evidence. The allowed origin set should be derived from the
  server's actual bind host and advertised UI origin, with loopback aliases
  handled deliberately rather than by substring or host-only matching.

## Finding 2: Content-Type Enforcement Needs Exact Parse Semantics

- **Severity:** Medium
- **Scope:** GH #9 / `_read_json_body`
- **Issue:** The issue asks for strict `Content-Type` validation, but does not
  define the parser contract. Common edge cases such as a missing header,
  `text/plain`, `application/jsonx`, comma-joined duplicate values, malformed
  media types, or valid parameters like `application/json; charset=utf-8` need
  explicit expected behavior.
- **Why this matters:** Ad hoc string checks are easy to get wrong. A permissive
  prefix/substring check could accept non-JSON media types, while an overly
  narrow equality check could break normal JSON clients and push callers toward
  bypass paths.
- **Required revision:** Add tests that reject missing/non-JSON/malformed media
  types before body parsing or CLI dispatch, and accept only a parsed
  `application/json` media type with ordinary parameters.

## Finding 3: Dry-Run "No Events Row" Can Conflict With Security Audit

- **Severity:** Medium
- **Scope:** GH #11 / recovery `auto-publish --dry-run`
- **Issue:** GH #11 says the dry-run regression test should assert "no
  `events` row, no lease, no artifact." The "no events row" wording is too
  broad for a command invoked through a local mutation bridge that should remain
  auditable.
- **Why this matters:** A literal implementation could suppress useful
  invocation/audit events just to satisfy the dry-run test, reducing forensic
  visibility for attempted CSRF or misuse. The security property needed here is
  no workflow-domain side effects, not no audit trail.
- **Required revision:** Define the no-side-effect guarantee as no artifact
  publication, no verdict/job/run mutation, no lease acquisition or queue
  mutation, and no recovery state transition. Permit append-only audit or
  invocation events that record the dry-run attempt, and make the regression test
  distinguish audit telemetry from domain mutation events.

## Verdict

**Needs revision.** The hardening work should proceed only after the acceptance
criteria close these gaps with fail-closed Origin/Referer behavior, precise
Content-Type parsing tests, and a dry-run side-effect contract that preserves
auditability.
