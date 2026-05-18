# GH-issue-driven workflows

Status: reusable workflow type
Date: 2026-05-14

Lightweight workflows for closing individual GitHub issues. Sister to
`docs/dogfood/` (which carries RFC-scoped multi-phase work).

## When to use this shape

Use `docs/issues/<N>/` when the work is bounded by a single GitHub
issue with a clear "Definition of done" or "Expected outcome" section.

Use `docs/dogfood/<N>/` when the work is a phase of an RFC, requires
synthesis from multiple inputs, or spans more than one deliverable
file/module that needs coordinated review.

| Property | `docs/issues/<N>/` | `docs/dogfood/<N>/` |
|---|---|---|
| Spec source | GH issue body | RFC + design synthesis |
| Jobs | 3 (triage → fix → verify) | 6 (synth → review_design → implement → 3-way build review) |
| Verifier count | 1 (or 3 for HIGH severity) | 3 (codex threat_model + claude ergonomics_dx + gemini adversarial) |
| Synth phase | No — issue body IS the spec | Yes — `DESIGN_SYNTHESIS.md` |
| Branch name | `striatum/gh-<N>-<slug>` | `striatum/dogfood-<N>-<slug>` |
| Typical duration | 1-3 hours | 4-12 hours |

## Job shape

1. **triage** — read the GH issue body + the linked code/docs/prompts;
   produce `SCOPE.md` naming the files to change + acceptance checks
   derived from the issue's "Definition of done" / "Expected outcome" /
   "Required behavior" sections. Stops scope creep early.
2. **fix** — implement the change. Spec is GH issue body + `SCOPE.md`.
   Output: `HANDOFF.md` citing each definition-of-done bullet closed.
3. **verify** — fresh-session reviewer reads only the GH issue body +
   the changed files; verdict on whether each definition-of-done bullet
   is actually closed. Output: `REVIEW.md` with verdict `accept` /
   `accept_with_findings` / `needs_revision`.

For HIGH severity issues (security, data loss, run-state corruption):
fan `verify` out to 3 lanes (codex threat_model + claude ergonomics_dx
+ gemini adversarial). Use the security hardening dogfood from
`docs/ROADMAP.md` §4.1 as the precedent.

## Lane selection guidance

| Issue type | Suggested triage | Suggested fix | Suggested verify |
|---|---|---|---|
| Documentation / prompt | claude_code | codex | claude_code (fresh) |
| Bug fix (Python) | codex | codex | claude_code |
| Bug fix (Go / shell / wrapper) | claude_code | claude_code | codex (fresh) |
| Web UI ergonomics | claude_code | codex | gemini + claude (2-way) |
| Security (HIGH) | codex | codex | 3-way (codex/claude/gemini) |
| Test gap | claude_code | codex | claude_code |

Avoid same-lane triage↔fix↔verify (codex/codex anti-pattern; D095-D098).

## Closure

After the run completes:

```bash
gh issue close <N> --comment "Closed by run_<id>; see docs/issues/<N>/HANDOFF.md"
```

Update `docs/issues/<N>/OPERATOR_REPORT.md` with the run id, verdicts,
and any deferred sub-findings. If sub-findings emerged, file them as
new GH issues rather than amending the closed one.

## Current

- [16/](16/) — Add complete operator initialization prompt
  ([gh#16](https://github.com/halbritt/striatum/issues/16))
- [22/](22/) — daemon migration path requires owner role but has no
  --admin-url ([gh#22](https://github.com/halbritt/striatum/issues/22))
- [23/](23/) — daemon status reads striatumd.pid but no code path
  writes it ([gh#23](https://github.com/halbritt/striatum/issues/23))
