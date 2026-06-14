# Apply — RFC 0126 P0

Address every accepted review finding from the prior review, then finalize.
Keep the change minimal and strictly within P0 scope and your write_scope.

Produce `docs/campaigns/rfc-0126/artifacts/SUMMARY.md` recording:

1. The final set of edits (files + the migration path actually taken, with the
   verdicts-table ownership evidence).
2. How each review finding was resolved.
3. The test + build results (`make -C go build` and the P0 pgtest output).
4. A one-line statement of what remains for P1–P3 (the separate runs).

Do NOT merge to main — leave the change on the run's feature branch for operator
review and integration.
