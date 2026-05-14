---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics", "dx", "rfc-0050", "v2", "markdown-rendering", "operator-surface"]
---

author: reviewer-unknown-model-002

# Build Review — Claude Ergonomics/DX

**Logical Name:** build_review_claude
**Posture:** ergonomics_dx (first-time-operator surface)
**Artifacts under review:**
- `docs/issues/9/SPEC.md`
- `docs/issues/10/SPEC.md`
- `docs/issues/11/SPEC.md`

## Executive Summary

The three GH-issue SPECs faithfully mirror the upstream gemini adversarial
finding bodies (RFC 0050 V2). Content is clear, consistently structured
(Attack → Impact → Mitigation), and each spec carries actionable remediation
options ranked by priority. As source material for an implementer driving the
v1.48.x security-hardening pass, they are usable as-is.

However, the operator-visible *surface* has one consistent papercut: the
title and source-metadata block at the top of each SPEC is indented by four
spaces, which Markdown renders as a code block rather than as an H1 heading
plus prose. Any first-time operator opening these files in GitHub, the
Striatum dashboard, or a standard Markdown previewer will see a monospaced
preformatted blob where they expect the title and labels.

Acceptance is conditional on cleaning up the front-matter indent so the
title affordance is discoverable and consistent with the rest of the doc.

## Per-Issue Acceptance Checklist

### GH #9 (CSRF on /v1/invoke) — `docs/issues/9/SPEC.md`

- [x] Issue source link present: L4 (`<https://github.com/halbritt/striatum/issues/9>`).
- [x] Labels enumerated: L5 (`bug, security, rfc-0050`).
- [x] Origin reference: L11–L12 (links back to dogfood-056 gemini Finding 1).
- [x] Attack vector explained with concrete payload: L14–L26.
- [x] Impact statement crisp: L30–L32 ("Remote command execution on the local runner").
- [x] Mitigations enumerated AND prioritized: L34–L40 ("Recommended landing: (1) + (2) for V1, (3) for V1.5").
- [x] Relation to RFC scoped: L42–L46 (V2 surface).
- [ ] **Title block renders as a heading.** L2–L7 are indented four spaces; in Markdown that becomes a `<pre>` block, hiding the H1 and metadata from rendered views.

### GH #10 (Override modal DOM trust) — `docs/issues/10/SPEC.md`

- [x] Issue source link: L4.
- [x] Labels enumerated: L5.
- [x] Origin reference: L11 (dogfood-056 Finding 2).
- [x] Attack vector ties to specific file: L13–L15 (`src/striatum/web/static/override_verdict.js`).
- [x] Impact statement: L17–L19.
- [x] Mitigation guidance with both client-side and server-side options: L21–L23.
- [x] Bundling guidance: L25 ("Bundle with #9 in v1.48.x security-hardening pass").
- [ ] **Title block renders as a heading.** L2–L7 indented four spaces (same defect as #9).

### GH #11 (Recovery panel dry-run trust) — `docs/issues/11/SPEC.md`

- [x] Issue source link: L4.
- [x] Labels enumerated: L5.
- [x] Origin reference: L11 (dogfood-056 Finding 3).
- [x] Attack vector with combine-with-#9 framing: L13–L17.
- [x] Mitigation options enumerated with explicit regression-test ask: L19–L23.
- [x] Bundling guidance: L25 ("Defense-in-depth against #9; bundle with v1.48.x").
- [ ] **Title block renders as a heading.** L2–L7 indented four spaces (same defect as #9 and #10).

## Findings

### Finding 1: SPEC title blocks render as preformatted code, not headings

- **Severity:** Medium
- **Files:** `docs/issues/9/SPEC.md` L2–L7, `docs/issues/10/SPEC.md` L2–L7,
  `docs/issues/11/SPEC.md` L2–L7.
- **Symptom:** Each SPEC begins with a blank line followed by six lines
  prefixed by four spaces. Per CommonMark, a four-space indent on a line
  following a blank line produces an indented code block. The `#` is taken
  literally rather than as an ATX heading, and the "Source:", "Labels:",
  and "Captured here verbatim..." metadata is monospaced.
- **First-time-operator impact:** Opening any of these files in GitHub's
  rendered view, the dashboard's Markdown surface, or an IDE preview shows
  a preformatted blob at the top rather than the issue title, GH link, and
  labels. The operator must scroll past or view raw source to learn what
  the document is about. Discoverability of the most important affordance
  (title + GH link + label set) is degraded.
- **Remediation:** Dedent L2–L7 to column 0 in all three SPECs. The
  separator `---` on L9 and all `##` headings below already sit at column 0
  and render correctly; only the leading metadata block needs the fix.
- **Verification:** After the fix, `head -10 docs/issues/N/SPEC.md` should
  show `# GH #N -- ...` at column 0 and the metadata lines should not be
  preceded by leading whitespace.

### Finding 2: Cross-references between issues are unlinked text

- **Severity:** Low
- **Files:** `docs/issues/10/SPEC.md` L25, `docs/issues/11/SPEC.md` L17, L25.
- **Symptom:** #10 and #11 refer to "#9" in prose ("Bundle with #9",
  "Combined with #9", "Defense-in-depth against #9") but provide no
  hyperlink or relative path. A first-time operator opening #10 first has
  no jump target to #9's spec — they must either guess the path
  (`../9/SPEC.md`) or go back to GitHub.
- **Remediation:** Replace bare "#9" mentions with either the GH URL or
  `[GH #9](../9/SPEC.md)` so the link is clickable from rendered Markdown.
  Optional but consistent with the captured-verbatim-for-offline posture
  noted in each spec's preamble.

### Finding 3: No explicit "Definition of done" / acceptance criteria block

- **Severity:** Low
- **Files:** all three SPECs.
- **Symptom:** #9's L40 ("Recommended landing: (1) + (2) for V1, (3) for
  V1.5 if needed") is the closest thing to a Definition of Done, but it is
  embedded in the Mitigations section. #10 and #11 do not state which
  mitigations are required to close the issue. An implementer claiming the
  fix packet has to infer scope from "Recommended landing" prose.
- **Remediation:** Add a short `## Definition of Done` section to each
  spec naming the mitigations that must land for the issue to close, vs.
  those deferred to a later milestone. (Optional — implementer can derive
  from the existing prose; this is a discoverability nicety.)

### Finding 4: Verbatim-capture preamble offers no path to additional context

- **Severity:** Low
- **Files:** all three SPECs L4–L7.
- **Symptom:** The preamble states the SPEC is "Captured here verbatim so
  the runner's `context.docs` is self-contained and reviewers do not need
  GitHub API access mid-run." Good intent. But there is no pointer to the
  dogfood-056 source review (other than the inline mention later in each
  body), no link to RFC 0050, and no mention that #9/#10/#11 are bundled.
  A first-time operator reading #10 in isolation has to mine the body to
  discover the parent RFC.
- **Remediation:** Add a one-line "Related:" or "See also:" entry to the
  preamble that names RFC 0050, the dogfood-056 review path, and the
  bundle siblings. Lightweight change; consistent with the
  self-contained-context intent already stated.

## Verification Assessment

This is an `ergonomics_dx`, `document_only` review — no code, tests, or
runtime artifacts were examined. The body content of each SPEC accurately
mirrors the upstream gemini finding (verified against
`docs/dogfood/056/review/build/gemini/REVIEW.md` Findings 1–3), so the
information fidelity is intact; the issues raised here are surface-level
Markdown rendering and discoverability concerns, not content
inaccuracies. The build phase succeeded at producing implementer-ready
specs; it stumbled on rendered-surface polish.

## Verdict

**Accept with findings.** The three SPECs are content-complete and
sufficient to drive the v1.48.x security-hardening implementation. Finding 1
(indented title block) is a real first-time-operator papercut that should
be cleaned up before the docs are linked from operator-facing surfaces, but
it does not block fix-phase work because the implementer can read raw
source. Findings 2–4 are discretionary polish.
