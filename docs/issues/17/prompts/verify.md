
# Verify Issue Workflow

Fresh-context review. Posture: `compliance_license`.

## Read

- `docs/issues/17/SPEC.md`
- upstream scope/design artifacts and implementer handoff for this workflow
- changed files and tests named by the handoff
- `docs/ROADMAP.md` and `docs/TODO.md` only to preserve scoped/deferred work

## Output

Write the finding artifact declared in the work packet. Use
`striatum.finding.v1` front matter and include:

- final verdict (`accept`, `accept_with_findings`, `needs_revision`, or `reject`);
- issue-by-issue acceptance checklist with file:line evidence;
- test/verification assessment;
- findings with severity and exact remediation when any gap remains.
