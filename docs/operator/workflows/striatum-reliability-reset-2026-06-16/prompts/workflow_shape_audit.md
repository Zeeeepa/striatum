# Workflow Shape Audit

Audit whether workflow shape complexity has outrun Striatum's reliability
envelope.

Focus on `divergent_ideation`, generated fan-out/fan-in, support-tier claims,
unattended fixtures, generated write scopes, shape governance, and any defects
that escaped the graduation gate. Treat `divergent_ideation` as the incident
that exposed the system, not as the only thing to inspect.

Write `docs/operator/artifacts/striatum-reliability-reset-2026-06-16/workflow_shapes/REVIEW.md`.
Include the exact author line from your work packet.

Required sections:

- Should `divergent_ideation` remain supported, be demoted, or be frozen?
- What its reliability fixture did and did not prove.
- Fan-in/fan-out and write-scope risks.
- Shape catalog support-tier honesty.
- Minimum graduation gate for any future shape.
- Delete/demote/freeze recommendations.

