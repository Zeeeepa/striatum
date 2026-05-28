---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "reject"
severity: "high"
tags: ["build-review", "threat-model", "tmux", "interrogation"]
---

# RFC 0089 Phase 1 Build Threat-Model Review

author: reviewer-codex-gpt-5.5-xhigh-001

Verdict: reject.

The review could not accept the build on a threat-model basis because the
required live builder interrogation produced no answer before verdict. The task
requires build review to interrogate the live reviewed session before verdict and
to publish a curated `INTERROGATION_CHAT.md` projection with question/answer
turns and IDs only (`TASK.md:67-74`). I opened interrogation
`intg_fbc8795f8d00015abf1bd8ae651999d0` against the active codex builder
session and asked for the implementation's tmux identity fields, delivery and
recovery gates, and tests. After repeated curated `interrogation.show` polls, it
still contained only the question turn. That leaves the main trust-boundary
claims unverified by the preserved builder context.

## Trust Boundaries Reviewed

- Daemon-owned workflow state versus operator-local tmux/PTY diagnostics. RFC
  0089 keeps pane text and PTY logs out of workflow state, provenance, verdict
  input, byline input, and exports (`0089:37-49`, `0089:169-177`; `0088:127-133`).
- Real lane process identity versus transient observer attach clients. The
  required slice is to stop treating `tmux attach-session` as the supervised
  lane identity (`TASK.md:38-53`; `0089:72-87`, `0089:103-121`).
- Tmux session/pane liveness versus packet delivery and recovery authority. RFC
  0089 requires `has-session`, pane existence, `pane_dead`, pane pid, and
  start-token comparison, with structured failure classes surfaced without pane
  text (`TASK.md:44-49`; `0089:123-145`).
- Supervisor metadata versus new workflow authority. The tmux session, pane id,
  pane pid, start token, and attach command are metadata, while liveness is a
  derived read-model value (`0089:216-224`).

## Blocking Risk

The build must demonstrate that killing or replacing the tmux session/pane
transitions the supervisor/read projection to a structured unhealthy state
before any further packet delivery (`0089:181-199`). Without a builder answer
confirming where delivery reconciliation and recovery sweep gate on the tmux
probe, this remains the critical attack surface: a stale or replaced pane could
continue receiving work, retain lane attestation incorrectly, or publish under a
trusted byline after the real lane identity has changed.

## Required Evidence Before Accept

- Curated builder answer identifying the persisted/read-model tmux fields and
  confirming attach command metadata cannot become the supervised process
  identity.
- Verification that packet delivery, `supervise.status`, doctor/status/dashboard
  details, recovery sweep, and `supervise.stop` all use the tmux probe for
  tmux-backed lanes.
- Tests covering attach-client exit, missing session, missing pane,
  `pane_dead`, pid/start-token mismatch, and D028 transcript non-leakage, as
  required by the task (`TASK.md:76-86`) and RFC acceptance criteria
  (`0089:181-199`).
