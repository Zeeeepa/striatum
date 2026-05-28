# Final implementation pass - RFC 0089

Read:

- `docs/operator/workflows/rfc-0089-tmux-final-build-review/TASK.md`
- `docs/rfcs/0089-tmux-backed-lane-monitoring.md`
- `docs/operator/workflows/rfc-0089-tmux-helper-redesign-rerun2/artifacts/DESIGN_SYNTHESIS.md`
- `docs/operator/workflows/rfc-0089-tmux-helper-redesign-rerun2/artifacts/review/build/codex/REVIEW.md`
- `docs/operator/workflows/rfc-0089-tmux-helper-redesign-rerun2/artifacts/review/build/agy/REVIEW.md`
- `docs/operator/workflows/rfc-0089-tmux-helper-redesign-rerun2/artifacts/review/build/claude_code/REVIEW.md`

Inspect the current working tree. The expected final behavior is:

- session registration defaults to workflow lane capabilities when no explicit
  capability list is supplied;
- a helper-owned attach-bridge exit with a live tmux pane keeps pane liveness
  attached/attested but marks delivery liveness degraded;
- `supervise.send` refuses delivery while the supervisor is delivery-degraded;
- status/dashboard/read projections surface delivery liveness separately from
  tmux pane liveness;
- no raw tmux pane text, PTY log bytes, or transcripts enter daemon state or
  durable artifacts.

If the current implementation already satisfies this, do not churn code. If a
gap remains, fix it with focused tests.

Run:

```bash
cd go && gofmt -l . && go vet ./... && go test ./...
```

Write `docs/operator/workflows/rfc-0089-tmux-final-build-review/artifacts/build/HANDOFF.md`
with:

- files changed in this final pass;
- final behavior landed;
- exact verification commands/results;
- remaining non-blocking findings, if any;
- the live session id reviewers should interrogate if it is not obvious from
  `list.sessions`.

Stay live for build-review interrogation after publishing and completing.
