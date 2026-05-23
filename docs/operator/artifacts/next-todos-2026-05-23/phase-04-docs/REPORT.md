---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/ROADMAP.md", "docs/operator/artifacts/next-todos-2026-05-23/phase-03-service-split/REPORT.md", "docs/operator/artifacts/next-todos-2026-05-23/phase-03-escalation/REPORT.md"]
---

# Phase 4 Docs Follow-Through
author: operator [self-declared: codex-driver]

## Result

Updated `docs/TODO.md` and `docs/ROADMAP.md` to match the phase-3 source
changes:

- TODO 52 / roadmap Phase 4 now says `web/doctor.py` owns doctor page
  rendering and response/error mapping, with `service.py` keeping only the
  stable route wrapper.
- TODO 53 / roadmap Phase 5 now says the typed `striatumd.escalation_inbox`
  table landed in both Python and Go migrations, and the remaining work is
  blocker payload hardening plus a future direct create/update method only if
  product scope requires it.

D125, RFC 0075, and CLI retirement gates remain explicitly not claimed as
complete.

## Validation

- `rg -n 'typed escalation table|dedicated escalation table|table itself is still missing|schema hardening: whether|web/doctor.py.*keeps template rendering|doctor.py.*problem grouping while \`service.py\` keeps template' docs/TODO.md docs/ROADMAP.md docs/rfcs/0062-real-escalation-inbox.md`
- `git diff --check`
