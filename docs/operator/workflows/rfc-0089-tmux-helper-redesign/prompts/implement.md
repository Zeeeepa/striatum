# Implementation - RFC 0089 Phase 1

Read `TASK.md`, RFC 0089, the design synthesis, and the design review. Implement
the synthesized Phase 1 only: replace attach-as-liveness with tmux session/pane
liveness.

Expected implementation shape:

- record tmux session/window/pane/pane-pid metadata at launch;
- make `tmux attach-session` metadata only;
- add a probe for session/pane liveness and structured failure classes;
- route supervisor status, delivery reconciliation, doctor/status/dashboard
  details, recovery sweep, and stop behavior through the probe;
- preserve non-tmux fallback behavior and D028 transcript boundaries.

Add focused tests before or with the implementation. Run:

```bash
cd go && gofmt -l . && go vet ./... && go test ./...
```

Write `docs/operator/workflows/rfc-0089-tmux-helper-redesign/artifacts/build/HANDOFF.md`
with the files changed, what landed, what remains blocked, exact verification
commands/results, and whether universal tmux monitoring is now a
configuration/default change. Stay live for build-review interrogation.
