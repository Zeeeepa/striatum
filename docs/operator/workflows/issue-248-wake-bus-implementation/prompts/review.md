Review the RFC 0120 Phase 2 implementation from the draft job.

Hard boundary:

- This review is only for #248 / RFC 0120 Phase 2 wake delivery.
- Treat any edits outside the declared #248 write scope as a blocking finding, including `docs/operator/BRIEF.md`, `docs/operator/workflows/issue-252-*`, `ORIGINAL_REQUEST.md`, or `STRIATUM_GIT_HYGIENE_GEMINI_2026-06-10.md`.
- Do not create, modify, or scaffold workflows for any other issue, including lane-auth, issue #250, or issue #252 follow-up work.

Read the required context docs and the produced diff. Write only the declared review artifact; do not modify source, docs, contracts, or other artifacts.

Focus on:

- Correctness: wake hints are notify-only and never become authoritative workflow state.
- Liveness: lost/missing notifications fall back to bounded interval polling and cannot wedge `run drive`.
- Privacy: wake payloads exclude prompts, artifacts, PTY output, transcripts, tokens, raw user content, and provider output.
- Ordering: wake hints for enqueue/requeue/conversation/interrogation transitions are emitted only after durable state is observable.
- `run drive`: idle waits use wake hints when supported, wake early for reconcile, and preserve existing Phase 1 idle-exit behavior.
- Contracts/docs: any new RPC is present in `contracts/daemon_methods.json`, generated `go/pkg/rpc/registry_methods.go`, generated `docs/reference/daemon-method-tables.md`, and curated references.
- Tests: the implementation covers the required behavior without brittle implementation-detail assertions or immediate fake wait loops.
- Product boundary: no daemon-side lane spawn, auto-spawn scheduler, hosted queue, telemetry, external persistence, or scheduler principal.

Use a code-review stance. If the implementation should not land, submit a `needs_revision` verdict with concrete file/line findings and a minimal fix path. If it is ready, submit an accepting verdict and note any residual #212/non-goal follow-up.

Publish `docs/operator/artifacts/issue-248-wake-bus-implementation/review/REVIEW.md` with:

- author line `author: reviewer-codex-gpt-5-001`
- verdict
- findings ordered by severity
- verification gaps, if any
