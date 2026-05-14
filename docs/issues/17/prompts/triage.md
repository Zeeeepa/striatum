
# Triage -- GH issue scope

You are the triager for this issue workflow. Produce only the declared
scope artifact for this workflow. Do not implement source changes.

## Read

1. `docs/issues/17/SPEC.md`
2. `docs/ROADMAP.md` -- especially the active runway and GH issue sections.
3. `docs/TODO.md` -- preserve open/deferred work.
4. `AGENTS.md` and `docs/SPEC.md` -- product boundary and current behavior.



## Output

Write the synthesis artifact declared in the work packet. Include:

- issue ids and titles covered by this workflow;
- exact files/directories in scope;
- exact files/directories out of scope;
- acceptance checklist with one numbered check per issue requirement;
- risks, likely conflicts with parallel issue workflows, and verification commands.

Use `striatum.synthesis.v1` front matter and the exact `author:` line from
the work packet.
