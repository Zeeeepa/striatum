---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
---

# DESIGN REVIEW — Interrogation-log chat UI (posture: ergonomics_dx)

author: operator

Reviewed `DESIGN_SYNTHESIS.md` plus the closed threat-model interrogation
(`intg_5c7480019bab1ac0ad74cb0007a8dc46`, 6 turns) against the live synthesizer.
Verdict: **accept_with_findings** — the design is discoverable, internally
consistent, and buildable as written; the findings below are DX guardrails the
implementer must not lose, not blockers.

## What works (first-time-user view)
- The "spine vs. one divergence" framing (synthesis §17, §33) makes the
  buildable plan legible at a glance: reuse `reads.HandleInterrogationShow`,
  add one `case "interrogations":` arm, render server-side. Slice 1/Slice 2
  split (§104) gives a curl-testable tracer bullet before any render code.
- The chosen `?view=chat` sibling page keeps the existing `<pre>` run view
  intact (§50) — low surprise, low surface area.

## Findings (carry into implementation)
1. **`html/template`, NOT `text/template` (load-bearing).** The interrogation
   answer (turn_index 1) flags that claude_code's source design §b loosely
   wrote "`text/template` auto-escapes" — false; `text/template` does not
   escape and reintroduces stored XSS. Synthesis §41/§98 is authoritative
   (`html/template` auto-escaping). A first-time implementer copying §b verbatim
   would ship the vulnerable engine. **DX action:** the synthesis should state
   the engine name inline at §94 (`RenderInterrogation`), and Test #3 (§129)
   must pin `&lt;script&gt;`. Extend that fixture per the interrogator's
   suggestion to also cover `<img onerror=...>` and a `javascript:` pseudo-URL
   so a future Markdown library trips the test.
2. **IDOR: the run-ownership 404 is mandatory and easy to drop.** `show` is
   keyed by `interrogation_id`, not `run_id` (synthesis §64, interrogation
   turn_index 3 layer 2). codex/gemini omitted the `run_id == runID` assert;
   synthesis correctly elevates it to required + Test #2. Discoverability risk:
   it lives in the HTTP route, not the read — an implementer touching only the
   read will miss it. Keep the check adjacent to `renderOrShowInterrogation`.
3. **Honest residual gaps to surface, not fix here:** no per-session authz —
   `/v1/invoke` can still fetch any interrogation in the repo by id (turn 3,
   gap a/b); authored `body` may contain secrets the slice does not scrub
   (turn 5); browser disk cache exists (loopback-only, `no-store` optional).
   These are correctly scoped out under the local-first model; the design
   names them rather than hiding them — good DX honesty.

## Recommendation
**accept_with_findings.** Land Slices 1–2 as specified. Before merge: (a) name
`html/template` inline in the synthesis and harden Test #3 to the three-vector
fixture; (b) keep the run-ownership 404 in the route with Test #2. The residual
authz/secret/cache items are acceptable deferrals — record them as follow-ups
so the deferred Markdown/SSE slice revisits the threat model.
