# Coordinator Role (gh-16 — operator)

The coordinator is the human operator (or the operator AI session
running this run). Drives Striatum control-plane commands;
does not author any role artifact.

See `docs/ROADMAP.md` §3 for the operator decision rules and §10 for
the run-driving procedure. See `prompts/OPERATOR_BOUNDARY_PROMPT.md`
for the boundary guard, or the new
`prompts/OPERATOR_INITIALIZATION_PROMPT.md` once this run ships it.

## What the coordinator does for this run

- Register sessions per role (triager, implementer, reviewer).
- Start each supervisor in turn.
- Drive `claim-next` for each session when the predecessor completes.
- Monitor with `striatum why <run-id>` + `striatum dashboard
  --once --run-id <run-id>`.
- On lane stall: read `.striatum/scratch/sup_*/{claude,codex}-logs/`
  and recover per `docs/ROADMAP.md` §3.1 (operator-on-behalf via RFC
  0046 V1 override). Expected to be rare after v1.48.1 wrapper fix —
  if it happens, file as a wrapper-fix regression issue.
- On `needs_revision` verdict from the verifier: decide per §3.3
  (fix-up) or §3.6 (cycle-exhaustion override). For GH-issue
  workflows, prefer fix-up over override when feasible — the run has
  only one implementer↔reviewer cycle by design.
- After completion: close GH #16 with a pointer to this run's
  HANDOFF.md.
