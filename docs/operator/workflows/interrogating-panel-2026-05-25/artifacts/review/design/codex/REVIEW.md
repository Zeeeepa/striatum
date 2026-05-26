---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
---
# Threat-Model Review
author: operator

Posture: threat_model.

Finding: accept with findings. The synthesis is acceptable if its guardrails are
implementation requirements, not preferences.

Trust boundaries and attack surfaces:
- The chat page renders peer-session turn bodies, attacker-controlled authored
  text. The synthesis rejects client-side `app.js` and chooses server-side
  rendering because `html/template` auto-escapes while preserving D028's
  curated-field guarantee.
- The interrogation answer confirms the load-bearing distinction: `html/template`
  is required; `text/template` would not escape and would reintroduce stored XSS.
  Markdown/rich formatting remains deferred, so links, code blocks, raw tags, and
  `javascript:` text stay inert plain text with `white-space: pre-wrap`.
- The read/UI endpoint is an IDOR risk because `interrogation.show` is keyed by
  `interrogation_id`; the synthesis requires the nested route to verify
  `interrogation.run_id == runID` and 404 before writing any turn bytes.
- The interrogation answer clarifies the scope model: repository isolation comes
  from injected `repository_id`; the route-level run-ownership check closes the UI
  cross-run leak; per-session authorization and same-repository `/v1/invoke`
  access are out of scope for this local-first slice.
- Field exposure stays bounded to existing curated interrogation metadata and turn
  `kind`/`body`/ordering. Provider stdout/stderr, tool payloads, transcripts, and
  private diagnostics are not introduced unless a future change expands the read
  projection and template.

Required follow-through: tests must pin run-ownership 404 and render escaping,
including the `html/template` vs `text/template` hazard plus raw HTML,
attribute-injection, and Markdown-link payloads. Future Markdown, SSE, export, or
stricter multi-session tenancy work needs a fresh threat model.

Recommendation: accept_with_findings.
