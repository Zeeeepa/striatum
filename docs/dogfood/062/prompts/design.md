# Design task — RFC 0046 V1.7 lane-attestation gap closure

Produce `docs/dogfood/062/design/<lane>/DESIGN.md` (front-matter
`handoff.v1` — author line `author: designer-<lane>-<model>-001`)
answering the seven design questions in
[`roles/designer.md`](../roles/designer.md). One concrete answer per
question. Threat model is load-bearing — `--allow-no-process-execution`
operator-override path must remain functional.

**Required inputs:** see `roles/designer.md` § "Must read first".

**Deliverable sections:**

1. **Gate location** — file:line + function signature.
2. **SQL query** — exact parameterized query proving the session
   has a valid process_executions row.
3. **By-session vs by-artifact** — locked decision with threat-model
   justification.
4. **Error code + operator hint** — concrete strings.
5. **Operator-override compatibility** — explicit `--allow-no-process-execution`
   path preservation.
6. **Dashboard surface** — process_execution_count?
7. **Acceptance tests** — 4 named functions with fixture inputs.

**Write scope:** `docs/dogfood/062/design/<lane>/DESIGN.md`.
