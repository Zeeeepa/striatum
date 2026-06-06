# Synthesis: finalize RFC 0114 incorporating accepted review

You are the **author** again, finalizing the RFC.

## Read first

- Your draft (`.../artifacts/RFC_DRAFT.md`).
- The review finding (`.../artifacts/review/REVIEW.md`).
- `docs/dogfoods/rfc0114-read-scope/CONTEXT.md` (the source facts).

## What to produce

Write the FINAL, publishable RFC at the declared artifact path
(`docs/dogfoods/rfc0114-read-scope/artifacts/RFC_FINAL.md`).

- Incorporate every accepted review finding. If the verdict was
  `needs_revision`, fix exactly what the reviewer flagged. If
  `accept_with_findings`, fold in the non-blocking improvements.
- This is the body that the operator will place at
  `docs/rfcs/0114-<slug>.md` on the review branch. Write it as a complete,
  standalone RFC in the house style of `docs/rfcs/0113-*.md`:
  title (`# RFC 0114: ...`), `Status: proposed`, `Date:`, `author:` byline
  matching your packet, `Context:` line, then Problem, Goals, Non-Goals,
  Proposal (scope+ordering, the ownership-constraint resolution, the owner
  bundle 0006 plan, daemon read-handler dual-path changes, guard tests, doctor
  posture transition), Acceptance, Rollout + verification (owner-applied
  out-of-band, NOT executed here), and Open questions / revisit triggers.
- Keep it design-only. No `go/` change, no live owner-bundle apply, no daemon
  restart happened in this run.

Publish the artifact, then complete the job.
