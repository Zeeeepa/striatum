---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["dogfood-007", "rfc-0013"]
---

# RFC 0013 V1 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-08

Verdict intent: **accept_with_findings**.

The synthesis is implementation-ready. Vanilla JS + tiny markdown
helper is the right shape for V1. Two findings to fold in.

## D020 / D028 Compliance

- No external URLs in shipped assets; CSP header reinforces.
- No transcripts / browser logging; service stays read-only.

## Findings

### F1 (low) — `/v1/artifacts/<id>/raw` is a new RFC 0012 endpoint

**Issue.** Synthesis § 5 introduces `/v1/artifacts/<id>/raw` as a
necessary extension. RFC 0012 V1 didn't ship it. Acceptable —
RFC 0013 sits on top of RFC 0012 and can extend the V1 surface
since the new endpoint is read-only and follows the same envelope
shape (or rather, streams raw bytes for body content).

**Recommendation.** Document the new endpoint explicitly in RFC
0013's V1 Implementation Slice and in SPEC's "Local Service"
section, plus a regression test against an artifact whose
`repo_path` doesn't exist (404 with envelope).

### F2 (low) — Tiny Markdown helper risks XSS

**Issue.** The hand-rolled Markdown helper is deliberately minimal,
but if any of its handlers leave raw HTML through (e.g., in
`code` or fenced-block content), an artifact whose body contains
`<script>` could execute. The CSP `script-src 'self'` blocks
external scripts but inline-script-via-injection is still possible.

**Recommendation.** Escape HTML special characters
(`<`, `>`, `&`, `"`, `'`) at the input boundary of the helper
before any pattern matching. Add a unit test that an artifact
body containing `<script>alert(1)</script>` renders as visible
text, not executes.

## Acceptance

**accept_with_findings.** F1 + F2 fold cleanly into the build.
