# RFC 0076 Audit Remediation Gap Fix Handoff
author: remediator-codex-001
status: complete
date: 2026-05-22

## Summary

Applied one bounded documentation gap fix from the verification reports:
`docs/USING_STRIATUM.md` now has a recovery triage table mapping visible
operator states to inspection commands and recovery actions.

No source changes were needed. The source verification report closes REM-001,
REM-002, and REM-009 against current code and focused tests. Current docs
already contained the other requested RFC 0076 remediation material for RFC
0050 status, RFC 0076 acceptance, RFC 0077 routing, private project memory,
tmux watching, adopt workflow guidance, and Postgres.app/no-sudo setup notes.

## Changed Paths

- `docs/USING_STRIATUM.md`
- `docs/operator/artifacts/rfc-0076-audit-remediation/build/HANDOFF.md`

## Validation

Ran:

```bash
striatum workflow validate --allow-same-model-pairing docs/operator/workflows/rfc-0076-audit-remediation/workflow.json
rg -n "\| Visible state \||tmux attach -t <session-name>|Postgres.app|private project memory|suggested starter workflow|RFC 0077|first runnable operator workflow|native Go daemon" docs/USING_STRIATUM.md docs/HOW_TO_HUMAN.md docs/POSTGRES_TRANSITION.md docs/CONTEXT_HYGIENE.md docs/UBIQUITOUS_LANGUAGE.md docs/operator/BRIEF.md docs/ROADMAP.md docs/rfcs/README.md src/striatum/day_zero.py
(cd go && go test ./pkg/mutations ./pkg/mcp)
pytest tests/test_day_zero.py
pytest tests/daemon_pg/handlers/workflow_loop/test_claim_next.py
```

Results:

- Workflow validation passed for `rfc-0076-audit-remediation`.
- REM doc evidence search found the expected current strings.
- Go focused tests passed.
- `tests/test_day_zero.py`: 12 passed.
- `tests/daemon_pg/handlers/workflow_loop/test_claim_next.py`: 5 passed.

## Deferred Follow-Up

No additional gap fix is needed in this job.

The catalog follow-up artifact classifies generator/catalog promotion and role
pack work as deferred to RFC 0074 Phase A, and classifies a dedicated audit
finding schema plus operator UI issue queue as no action until more reuse
evidence exists.
