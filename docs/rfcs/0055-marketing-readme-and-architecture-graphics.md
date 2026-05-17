# RFC 0055 — Marketing-friendly README + system architecture graphics

**Status:** proposed (Phase A shipped)
**Scope:** V1.7 documentation; Phase A rewrote the top-level README
**Composes with:** RFC 0053 (operator/principal model — anchors the
diagram), RFC 0054 (day-zero usage guide — the README cross-links
into it), RFC 0056 (consumer-repo layout — README may reference the
recommendations).

## Background

The current top-level `README.md` is structured as a docs index and
quickstart. It opens with a brief mission line, then dives into
install / state-store reality / decision-record pointers / link list.
That serves existing operators but does not introduce Striatum to a
new arrival who wants to know, in 60 seconds:

- *What is this thing for?*
- *How is it structured?*
- *Why would I use it?*

The README is also the front page of the GitHub repo. A new visitor
forms an opinion in the time it takes to scroll the first screen.
Today's README does not show any diagrams; the system-architecture
mental model has to be reconstructed from `SPEC.md`,
`UBIQUITOUS_LANGUAGE.md`, and the RFC catalog.

## Goals

- Rewrite the top-level `README.md` to lead with vision and value,
  not docs index.
- Include at least one system-architecture diagram on the front
  page.
- Preserve the existing utility links (install, GETTING_STARTED,
  SPEC, RFCs) but move them out of the lead.
- Reflect the current substrate honestly (Postgres + mandatory
  daemon per RFC 0043; AI operator + human principal per RFC 0053).
- Keep the README scannable — front-page should fit in two
  scrolls.

## Non-goals

- Producing marketing copy that overpromises. Striatum is a local
  orchestrator; the README should be confident, not breathless.
- Adding hosted-service framing. Per RFC 0043 / D094 Striatum is
  local-daemon-first; no hosted SaaS positioning.
- Adding screenshots of UIs that may move. Architecture diagrams
  describe the *model*, not the *current pixel layout*.
- Replacing `docs/INDEX.md` or removing the docs-index function
  entirely — move it down the README, not out.

## Proposed shape

A rewritten `README.md` with this structure (illustrative; the
copy harden in Phase A):

1. **Headline + tagline** — one sentence: "Striatum is a local
   workflow runner for terminal-based AI coding agents." Plus a
   one-line tagline.
2. **What it does** — 3-5 bullets, value-oriented (e.g.
   "Coordinates multi-lane review cycles," "Records every decision
   with audit-chain provenance," "Runs locally — no hosted
   services").
3. **System architecture** — a diagram showing the AI operator(s),
   the daemon, the Postgres substrate, the target repository, and
   the human principal as an escalation receiver. Plus a paragraph
   explaining the picture.
4. **The two roles** — AI operator and human principal (per RFC
   0053), one line each, linking into the day-zero guide.
5. **Quick start** — one block of shell, with a "for full
   walkthrough see [USING_STRIATUM.md or GETTING_STARTED.md]" link.
6. **Why Striatum** — the problem space in 2-3 paragraphs. The
   reviewer-co-blindness pattern, the audit-chain need, the
   provider-portability stance. Cite the dogfood ledger as
   evidence the project eats its own food.
7. **Project status** — version, supported platforms, license,
   contribution path.
8. **Docs** — the curated link list (current README's
   strength), demoted to its own section near the bottom.

## Diagram format

Three candidates; the Phase 0 decision picks one (or all three):

- **Mermaid.** Renders natively on GitHub. Diagrams live in source
  (`.md`), reviewable in diffs, no binary blobs. Limited layout
  control. *(Recommended starting point.)*
- **ASCII.** Renders everywhere including terminals and reviews.
  Maintainable; no toolchain. Less polished visually.
- **SVG.** Highest design polish. Binary in the repo (or generated
  from a source like draw.io / d2 / excalidraw). Heavier
  contributor burden; can render at any size.

Mermaid wins on the contributor / diff / review axes; SVG wins on
polish. ASCII wins on render-everywhere. The RFC defaults to
Mermaid for Phase A with an SVG follow-up if the polish matters
enough.

## Open questions

1. **Tone.** Should the README read as developer-product (think
   `htop` or `ripgrep` README) or as infrastructure-product (think
   `kubernetes`, `nomad`)? Striatum sits between; pick a register.
2. **What lives at the top.** Does the headline + tagline + value
   bullets all fit before the fold? Cutting one is fine; bury
   anything past it.
3. **Architecture diagram audience.** Does the diagram aim at
   *adopter* (here's how the pieces fit so you can install) or
   *evaluator* (here's the system shape so you can compare to
   other tools)? Different abstractions for each.
4. **Maintenance burden.** Each diagram needs to be kept current
   as RFCs land. Should the README pin the diagram to a stable
   high-level shape (operator / daemon / substrate / repo) that
   rarely changes, with detailed diagrams living in
   per-RFC docs?
5. **Marketing-versus-honesty boundary.** The README should not
   overclaim. Where's the line on phrases like "audit-quality
   provenance," "portable across providers," etc.? Calibrated by
   what the dogfood ledger actually demonstrates.
6. **Animated screenshots / GIFs.** Some README cultures use a
   `gif` of the dashboard as the hero asset. In or out? Adds
   maintenance burden; commits binary churn.

## Phasing

- **Phase 0 (this RFC):** scaffold accepted; Open questions 1, 2,
  3, 6 resolved; diagram format chosen (recommend Mermaid).
- **Phase A:** rewrite `README.md` per the resolved outline. Single
  commit lands the rewrite + the Mermaid (or chosen-format)
  diagram inline. Link updates in `docs/INDEX.md` and any docs
  that cross-reference README sections by anchor.
- **Phase B (optional):** SVG architecture diagram as a polish
  pass once the shape is stable.

## Provenance

- 2026-05-14 operator session: project owner asked for "a more
  marketing friendly README on the front page with some system
  architecture graphics."
- Companion RFCs in the same wishlist: RFC 0054 (day-zero usage
  guide), RFC 0056 (consumer-repo directory-structure opinions).
  All three were proposed together; this scaffolds the second.
- Substrate / role honesty anchors: RFC 0043 (Postgres + mandatory
  daemon), RFC 0053 (AI operator + human principal).
