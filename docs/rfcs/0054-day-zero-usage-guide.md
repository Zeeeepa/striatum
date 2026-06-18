# RFC 0054 — Day-zero "How to use Striatum" guide

**Status:** accepted (Phase A shipped)
**Scope:** V1.7 documentation; Phase A added `docs/USING_STRIATUM.md`
**Composes with:** RFC 0053 (human principal as escalation-only role +
AI operator as default driver), RFC 0055 (marketing-friendly README +
architecture graphics), RFC 0056 (consumer-repo directory-structure
opinions).

## Background

`docs/GETTING_STARTED.md` exists but was authored under the original
"pick a path — human operator or agent" framing. RFC 0053 collapsed
that bifurcation: the AI operator is the default driver and the human
is the principal who shows up only for escalations. Today's
GETTING_STARTED carries an RFC 0053 retrofit (commit
[7e21399](https://github.com/halbritt/striatum/commit/7e21399)), but
the doc is still organized as a feature tour with the day-zero
narrative buried inside.

A new arrival lands on the project and asks four questions in this
order:

1. *What is this?* (one-sentence definition + the operator/principal
   model)
2. *What do I need to install?* (runner + skill bundle + Postgres +
   daemon prerequisites)
3. *How do I start my first run?* (concrete commands, in order)
4. *What do I do as the principal?* (when the AI escalates)

`docs/GETTING_STARTED.md` currently answers (2) and (3) well, (1)
adequately after the retrofit, and (4) by cross-link only.
`docs/HOW_TO_HUMAN.md` answers (4) since [7e21399](https://github.com/halbritt/striatum/commit/7e21399). The
gap is a single top-down narrative that walks a new arrival through
all four in one read.

## Goals

- Produce a single day-zero guide that a new arrival can read end to
  end in ~10 minutes.
- Presume the RFC 0053 model: AI operator (default) + human principal
  (escalation-only).
- Lead with the mental model; cover prerequisites + first-run
  walkthrough + principal-on-call in that order.
- Cross-link the deep references (`HOW_TO_AGENT.md`,
  `HOW_TO_HUMAN.md`, `SPEC.md`, `CLI_REFERENCE.md`,
  `WORKFLOW_TYPES.md`) rather than duplicate them.

## Non-goals

- Replacing `HOW_TO_AGENT.md` or `HOW_TO_HUMAN.md`. Those are
  load-bearing role playbooks; the day-zero guide points at them.
- Becoming a tutorial-platform-style multi-page experience. One
  Markdown file, scannable headings, copy-pasteable commands.
- Embedding the full system architecture diagram. That belongs in the
  README (per RFC 0055); the day-zero guide links to it.

## Proposed shape

One new file at `docs/USING_STRIATUM.md` (or rewrite of
`docs/GETTING_STARTED.md` — see Open question 1). Section outline:

1. **What is Striatum?** — one paragraph definition; the operator +
   principal model in two short bullets.
2. **The two roles** — AI operator (default driver) and human
   principal (escalation-only). Each gets a 3-4 sentence
   description and a cross-link.
3. **Prerequisites** — Python 3.11+, Postgres (per RFC 0043),
   running daemon, an agent CLI (Claude Code / Codex / Gemini /
   generic).
4. **Day-zero setup** — install the runner, start the daemon,
   register the target repo, install the skill bundle for the
   chosen agent. Concrete commands in order.
5. **First run** — point the agent at a target repo and a
   workflow file; watch the dashboard. ~5 lines of commands.
6. **Your role as principal** — what to expect when an escalation
   surfaces; how to look at the inbox; cross-link to
   `HOW_TO_HUMAN.md` for the resolution playbook.
7. **Where to go next** — short curated list (the canonical role
   playbooks, the SPEC, the CLI reference, the WORKFLOW_TYPES
   guide, the README for architecture).

## Open questions

1. **Replace vs. complement.** Should this guide replace
   `docs/GETTING_STARTED.md` (which becomes a redirect), live
   alongside as `docs/USING_STRIATUM.md`, or BE the new
   `GETTING_STARTED.md` after a rewrite?
2. **Architecture surface.** A day-zero reader benefits from a small
   architecture diagram early. Does the guide embed an inline
   simplified diagram, or link to the full one in the README (RFC
   0055)?
3. **Tone.** Tutorial-warm (you / let's / here's why) or
   reference-cool (matches `SPEC.md` style)? The audience is a
   first-time reader, so warmer probably wins, but consistency with
   the rest of the doc set matters.
4. **Length budget.** Under 300 lines? Under 500? The current
   `GETTING_STARTED.md` is 270; the goal is to keep it scannable.
5. **Concrete-command discipline.** Should every step show real
   pasteable shell, or rely on `striatum --help` for syntax detail?
   Real shell wins for confidence; cross-link wins for maintenance.

## Phasing

- **Phase 0 (this RFC):** scaffold accepted; outline above pinned;
  Open question 1 resolved (replace / complement / rewrite).
- **Phase A:** write the doc per the resolved outline. Single
  commit lands the file + cross-link updates in
  `docs/INDEX.md`, the RFC 0055 README, and any cross-references in
  the deep playbooks.
- **Phase B (optional):** harvest content into the
  `--with-ddd-layout` scaffold (RFC 0021) if any of it should land
  in a target repo's docs by default.

## Provenance

- 2026-05-14 operator session: project owner asked for "a day zero
  'how to use striatum' doc which presumes human principle [sic —
  principal] and AI operator."
- The shape is constrained by RFC 0053 / D103 (model: AI operator +
  human principal). The day-zero guide is the canonical entry-point
  expression of that model for new arrivals.
- Companion RFCs in the same wishlist: RFC 0055 (marketing README +
  architecture graphics), RFC 0056 (consumer-repo directory-structure
  opinions). All three were proposed together; this scaffolds the
  first.
