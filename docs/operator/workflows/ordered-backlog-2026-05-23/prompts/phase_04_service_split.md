# Phase 4: TODO 52 Service Split

Write `docs/operator/artifacts/ordered-backlog-2026-05-23/phase-04-service-split/REPORT.md`
with `author: service-split-codex-gpt-5-001`.

Task:
- Review `src/striatum/service.py` and `src/striatum/web/static_assets.py`.
- Execute or classify the next small daemon-first web service split. Prefer
  moving static-asset response orchestration behind the web helper boundary.
- Include tests that prove `/static/*` behavior, CSP, and traversal refusal.

Keep `service.py` as a thin route/handler wrapper.
