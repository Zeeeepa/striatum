---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0013 step 7 Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-09
Run: dogfood-013 / RFC 0013 step 7 (web UI mutation buttons)
Decision: `accepted_with_follow_up` (V1_ACCEPTANCE; autonomous)
Version: `1.4.0`

## Files Changed

- **`src/striatum/service.py`**: `_handle_health` now returns
  `allow_mutations: bool` so the SPA can hide mutation buttons
  when the gate is off.
- **`src/striatum/web/static/app.js`** (~150 lines added):
  - `allowMutationsCache` module-level cache.
  - `mutationsAllowed()` / `invokeMutation(argv)` /
    `confirmModal({...})` / `showResultBanner(host, result)` /
    `runMutation({host, argv, ...})` helpers.
  - `renderRunDetail` adds a "Record decision" button.
  - `renderJobDetail` adds:
    - Per-blocker "Continue" / "Cancel job" buttons (open
      blockers).
    - "Record verdict" button (review jobs in `running`
      state).
    - "Requeue stale review" button (review-only jobs in
      `stale_lease` state).
  - All click handlers attached via `addEventListener`; no
    inline handlers (CSP-clean).
- **`src/striatum/web/static/app.css`** (~80 lines added):
  modal overlay + dialog + actions + argv preview block,
  mutation buttons (neutral + destructive variants), result
  banners (success + error).
- **`tests/test_web_ui.py`** (5 new cases, 13 total):
  - `test_health_includes_allow_mutations_flag_off`
  - `test_health_includes_allow_mutations_flag_on`
  - `test_invoke_mutation_refused_without_allow_mutations`
    (HTTP 405 envelope with "allow-mutations" message)
  - `test_spa_app_js_has_mutation_button_handlers`
    (string-grep guard for `/v1/invoke`, `/v1/health`,
    `mutationsAllowed`, `decision`, `record`, `verdict`,
    `checkpoint`, `resolve`, `requeue-stale`)
  - `test_spa_app_js_no_external_urls` (CSP / D020 guard)
- **`docs/SPEC.md`** § "Local Web UI" updated to V1+step 7;
  enumerates all five mutation buttons + their argv mapping.
- **`docs/rfcs/0013-local-web-ui.md`** status →
  `accepted (V1+step 7)`.
- **`docs/rfcs/README.md`** index reflects `accepted (V1+step 7)`
  + D065 reference.
- **`docs/DECISION_LOG.md`** D065.
- **`docs/TODO.md`** F13.
- **`pyproject.toml`** + **`src/striatum/__init__.py`** 1.3.0
  → 1.4.0.
- **`CHANGELOG.md`** 1.4.0 section.

## Verification

- `make lint` clean.
- `make typecheck` clean (51 source files).
- `make test` — 285 passed (280 baseline + 5 new web UI tests).
- Live service restarted at v1.4.0 with `--allow-mutations`:
  `curl http://127.0.0.1:8088/v1/health` returns
  `allow_mutations: true`. Tailscale bridge at
  `http://proximal:8088/` mirrors it.
- SPA load against `http://127.0.0.1:8088/` renders the new
  buttons in the right views.

## Notes For The Reviewer

- **Mutation gate refusal returns HTTP 405**, not exit 8. The
  service's `_dispatch_post` was already wrapping the gate
  refusal in HTTP 405 with a clear "command requires
  --allow-mutations" message. The SPA renders this inline as
  an error banner. Test
  `test_invoke_mutation_refused_without_allow_mutations`
  asserts both the status and the message substring.
- **No CSRF for V1**. The runner is loopback by design (D020);
  cross-origin attacks aren't possible against the loopback
  service. The tailscale bridge changes that — V1 explicitly
  defers CSRF to a future RFC. Documented in DECISION_LOG D065
  revisit triggers.
- **Verdict button collects session/lease via prompt()**. Not
  great UX, but the why-endpoint payload doesn't yet expose
  the active lease for a job. A future commit could surface
  `current_session_id` / `current_lease_id` in the job-detail
  read endpoint and the buttons would auto-populate. Out of
  scope for step 7.
- **Decision-record path validation is server-side**. The
  publisher refuses paths inside `.striatum/` and outside the
  repo with exit code 6; the SPA's prompt() collects the path
  but doesn't pre-validate. The error envelope renders
  inline if the path is rejected.
