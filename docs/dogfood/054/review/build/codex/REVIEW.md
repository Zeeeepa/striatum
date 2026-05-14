---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "provenance", "ui"]
---

author: reviewer-unknown-model-001

# Build Review — Threat Model

Verdict: needs_revision

## Trust Boundaries Reviewed

- Runner state to web payload shaping: `service.py` converts SQLite rows into UI semantics for verdicts, bylines, lane attestation, expected artifacts, and lane evidence.
- Web payload to Jinja templates: server-rendered pages must not drop provenance labels or render model-looking authorship for unattested sessions.
- Runner state to terminal dashboard: `dashboard.py` must consume the same `next_actions`, attestation, byline, override, and evidence vocabulary as the web surface.
- React shared components: TypeScript chips must keep closed enums, especially muted-only lane evidence for V1.
- Operator override surface: any accepting operator override must remain visibly distinct from the original natural verdict and must expose the rationale inline.
- Transcript boundary: process-adapter evidence may expose diagnostic envelopes, not live terminal output, stdout/stderr, or model transcripts.

## Attack Surfaces

- A stale or unattested lane could be made to look like a model-authored artifact if a template derives byline text from role/lane instead of publish-time truth.
- A human override could be laundered into an ordinary accepting verdict if a page ignores provenance or only displays `verdict`.
- A future evidence correlation feature could overclaim provenance if V1 renders green evidence before path-specific correlation exists.
- A convenience UI could drift into transcript capture by rendering supervised stdout/stderr or verbose logs.
- Dashboard and web parity can fail if either surface formats its own `next_actions` or chip labels instead of consuming the shared runner vocabulary.

## Finding 1 — Posture Verdict Audit Page Still Launders Operator Overrides

Severity: high

`run_posture_verdicts.html` still renders verdicts with the old raw status-pill path and never imports or calls the new `VerdictChip` macro. The row prints only `v.verdict` at `src/striatum/web/templates/run_posture_verdicts.html:30`, then prints a generic `Rationale:` row at `src/striatum/web/templates/run_posture_verdicts.html:49`. There is no provenance column, no `operator-override` label, no lane-attestation chip, and no guarantee that an override rationale is tied beside the override pill.

That leaves an audit route where an operator override to `accept` or `accept_with_findings` is visually indistinguishable from a natural verdict. This violates the RFC 0050 provenance-honesty requirement that override verdicts must not silently substitute for the original natural verdict, and it contradicts the design's posture-page requirement for provenance and attestation columns.

The new shaping code does compute override provenance for job-detail verdict rows in `src/striatum/service.py:431` through `src/striatum/service.py:441`, and the Jinja macro can render override provenance plus rationale in `src/striatum/web/templates/_components.html:26` through `src/striatum/web/templates/_components.html:38`. The vulnerable page bypasses both. The regression test also misses this route: `tests/test_override_rationale_regression.py:68` only requests `/run/<run_id>/job/<review_job_id>`, then asserts strings in that job-detail HTML at `tests/test_override_rationale_regression.py:72`.

Recommended fix: route posture verdict rows through the same provenance shaper used by job detail, import `_components.html`, render `ui.verdict_chip(v.verdict, v.provenance, v.override_rationale)`, and add a regression that fetches `/run/<run_id>/posture/<posture>` after an override and asserts both `operator-override` and the override rationale are visible inline.

## Regression Checks

- Byline regression: no failing path found in the inspected V1 run/job surfaces. `service.py` returns muted/unattested chip data for missing sessions at `src/striatum/service.py:253`, and run detail renders `author: operator` for unattested sessions at `src/striatum/web/templates/run_detail.html:98`.
- Override rationale: failed on the posture audit page described above. Job-detail and dashboard routes render the rationale, but the posture route does not render override provenance at all.
- Lane evidence: no green V1 path found in the inspected service/dashboard/component callers. `service.py` hard-codes `not_yet_correlated` at `src/striatum/service.py:278`, `dashboard.py` defaults to `not_yet_correlated` at `src/striatum/dashboard.py:477`, and the React type only admits `not_yet_correlated` at `src/striatum/web/frontend/src/shared/types.ts:385`.
- Transcript capture: no new supervised stdout/stderr or live terminal-output panel found in the inspected V1 files. The dashboard process evidence renderer exposes diagnostic envelope fields at `src/striatum/dashboard.py:634` through `src/striatum/dashboard.py:663`.
- Dashboard/web vocabulary parity and V1.41 `next_actions`: the focused regression suite passed, including the checks for `inspect_packet_with_inbox`, `derive_expected_byline`, and `recovery_auto_publish`.

## Verification

Ran:

```bash
pytest tests/test_dashboard_web_parity.py tests/test_byline_regression.py tests/test_override_rationale_regression.py -q
```

Result: 3 passed.
