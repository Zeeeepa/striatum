---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/plans/residual-deferred-closure-2026-05-23.md"]
---

# Residual Deferred Closure Summary
author: residual-deferred-coordinator-codex-gpt-5-001

## Outcome

All requested subagent tracks were scaffolded and driven through Striatum
workflow artifacts. The old residual/deferred list is no longer an ambiguous
backlog: each item is now either closed for current scope, explicitly
out-of-core/optional, or named as work that requires a new bounded RFC before
implementation.

## Closed For Current Scope

- TODO 62 / RFC 0069: PostgreSQL-only daemon-global surfaces are closed for
  current scope; future registry-probe regressions are guardrail failures.
- TODO 63 / RFC 0070: primitive daemon methods are the supported production
  path; removed composites stay out unless a future decision reintroduces
  them.
- TODO 2: process-adapter constraint enforcement is closed; enforced
  network/filesystem isolation needs a future sandbox adapter RFC.
- Artifact schemas/redaction: no missing current schema; session
  close/non-fresh reason prose now redacts as free text.
- RFC 0040 V1.6: PostgreSQL artifact summaries now expose recorded byline
  evidence via `author.line` and `author.actual_author_line`.
- TODO 16: current generic-language drift was corrected and the guardrail was
  widened; this remains a standing hygiene check, not a blocked backlog item.

## Explicit No-Action Or Out-Of-Core

- RFC 0049 remains shelved under D106.
- RFC 0054 Phase B guide-to-layout harvest is not warranted.
- RFC 0055 SVG polish is no-action until a concrete need appears.
- RFC 0056 workflow-file generation and artifact-root `.gitignore` policy are
  non-changes for the layout scaffold.
- TODO 59 external-consumer fetch/UI UX is outside Striatum core.
- TODO 60 hosted Git/PR provider actions are optional-plugin/out-of-core.
- RFC 0058 operator-tree init/rotation is optional future work.
- Engram-side ingester/MCP/retrieval tools are external to Striatum.

## Needs New Bounded RFC Before Implementation

- RFC 0052 Phase A committee deliberation implementation.
- RFC 0053 schema/runtime rename for `human_checkpoint` /
  `escalation_checkpoint` and `waiting_human`.
- RFC 0074 Phase B narrow `implementation_panel` generator slice is ready to
  schedule; UI selector/cost UX should follow separately.
- Cross-Repo Live Scheduler V1 for production prepare/start fan-out,
  dependency scheduling, cycle routing, session scope, recovery, and operator
  surfaces.
- Sealed apply/signing if `apply.reviewed_patch` is ever reintroduced.
- Windows daemon support.
- Local multi-operator tenancy.

## Still Active Outside This Closure

D125 default-live auto-finalize evidence, RFC 0050/RFC 0075 CLI retirement,
TODO 52 service splitting, TODO 53 escalation payload/schema hardening, and
TODO 61 legacy SQLite cleanup remain active product work. They are not
deferred closure leftovers.
