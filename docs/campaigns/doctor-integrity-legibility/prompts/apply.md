# Apply — Doctor integrity legibility (P0)

The review accepted (possibly with non-blocking findings). First address any
**required** revisions the review raised. Keep code changes minimal — the
implementation already landed in the draft phase.

Produce `docs/campaigns/doctor-integrity-legibility/artifacts/SUMMARY.md` as a
`synthesis` artifact with valid `striatum.synthesis.v1` front matter. It must record:

- What landed: the edited files (`go/pkg/reads/{worktree_refs.go,
  doctor_artifact_anchor.go,doctor.go}`), the warning taxonomy introduced, and the
  decision-log D-number.
- The before/after expectation: doctor's `problems` count should drop to only
  genuinely-unpreserved content; preserved-on-default-branch / terminal-run /
  legacy findings now appear as `warnings` (do not red `ok`).
- Any accepted-review findings deferred to follow-ups.
- Operator follow-ups: this is daemon code — it takes effect only after `make
  install` + `systemctl --user restart striatumd`; then re-run `striatum doctor`
  to confirm `ok` reflects only genuine loss, and the A-data reconciliation of the
  residual true-loss tail.

Do **NOT** merge to `main`.
