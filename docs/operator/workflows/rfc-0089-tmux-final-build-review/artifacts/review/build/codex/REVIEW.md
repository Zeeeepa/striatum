---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["threat-model", "rfc-0089", "tmux-liveness", "delivery-liveness"]
---

# Final build review codex threat model
author: reviewer-codex-gpt-5.5-xhigh-001
date: 2026-05-28
verdict: accept_with_findings

## Summary

The final RFC 0089 implementation addresses the blocking threat-model finding
from the rerun2 review: tmux pane liveness and lane byline attestation are no
longer treated as proof that the supervisor delivery path is healthy.
Helper-owned attach bridge exit and missing FIFO-reader cases now persist
explicit delivery degradation, and `supervise.send` refuses delivery before
writing or recording `supervisor.packet_delivered`.

No blocking threat-model findings remain. I am recording
`accept_with_findings` for one low-severity follow-through item: Phase 2 and
operator-facing consumers must keep `delivery_liveness` visible anywhere they
display `liveness=alive` or `lane_attestation=attested`, so the corrected DTO
boundary does not regress into a UI-level false-health signal.

## Verification

Required command run:

```bash
cd go && go test ./pkg/supervisor ./pkg/mutations ./pkg/reads
```

Result: passed.

I also inspected the implementation and focused tests around:

- `go/pkg/supervisor/helper.go`
- `go/pkg/mutations/supervision.go`
- `go/pkg/mutations/supervision_control.go`
- `go/pkg/reads/supervision.go`
- `go/pkg/supervisor/tmux_liveness.go`
- `go/pkg/mutations/supervision_test.go`
- `go/pkg/mutations/supervision_control_test.go`
- `go/pkg/reads/supervision_test.go`
- `go/pkg/supervisor/tmux_liveness_test.go`

## Interrogation

- Interrogation id: `intg_51838f106c8baf41787656cbb1921248`
- Target session: `sess_ac823cc767750f0857cd4a9fa2ced765`
- Rounds asked: 2
- Rounds answered: 2
- Stop reason: sufficient threat-model coverage after delivery-health and
  stale-pid/byline-attestation questions; interrogation closed normally.
- Curated log: `INTERROGATION_CHAT.md`

## Threat Boundaries Reviewed

- **Delivery vs. pane liveness:** `attach_client_exited` records
  `delivery_liveness: {class: degraded, healthy: false,
  reason: attach_client_exited}` while leaving a live pane attached. Missing
  FIFO readers record the same shape with `reason: stdin_reader_missing`.
  `supervise.send` checks that metadata before pane/PID liveness and refuses
  degraded delivery without recording delivery success.
- **Byline/provenance:** tmux-backed sessions only remain attested when the
  pane process identity is live and start-token verified. Literal, missing, or
  unavailable start tokens produce operational `tmux_ok` liveness but downgrade
  attestation to `start_token_unverified`.
- **Pane text authority:** liveness probing uses `has-session` and tmux
  identity fields. The guard tests and metadata sanitizers keep raw pane text,
  stdout/stderr, and transcript-like fields out of daemon events/read
  projections.
- **PID reuse and stale pane spoofing:** numeric `pane_start_time` tokens are
  compared when available, with Linux `/proc` fallback. Non-Linux or older tmux
  cases without a numeric token remain operationally live but unattested.
- **Stop/recovery safety:** tmux-backed stop uses `tmux kill-session` first and
  does not signal `attach_client_pid`. Any direct pane/helper PID cleanup
  fallback is gated by matching start tokens and records skip reasons on
  missing, unavailable, or mismatched tokens.

## Finding

### Low: preserve delivery-health visibility in downstream operator surfaces

The corrected core behavior is in place: read DTOs project
`delivery_liveness` separately from `tmux.liveness`, and mutation code refuses
degraded delivery. That is sufficient for Phase 1 acceptance.

The remaining risk is downstream presentation drift. Any compact dashboard,
web UI, or future status consumer that renders only `liveness=alive` and
`lane_attestation=attested` while omitting `delivery_liveness` can recreate
the old false-health operator impression at the presentation layer. Phase 2
should keep delivery health prominent wherever pane health or attestation is
shown.

This finding is non-blocking because the daemon state boundary and delivery
mutation guard are now correct.
