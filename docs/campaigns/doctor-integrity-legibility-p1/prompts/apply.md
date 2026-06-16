# Apply — Doctor integrity legibility P1

The review accepted (possibly with non-blocking findings). First address any
**required** revisions the review raised. Keep code changes minimal — the
implementation already landed in the draft phase.

Produce `docs/campaigns/doctor-integrity-legibility-p1/artifacts/SUMMARY.md` as a
`synthesis` artifact with valid `striatum.synthesis.v1` front matter. It must
record:

- What landed: the edited/new files (`go/pkg/reads/doctor_artifact_anchor.go`,
  `go/pkg/reads/doctor_acknowledged_loss.go`, tests, `go/pkg/reads/doctor.go` if
  touched), the three rules (history-awareness / superseded / acknowledged_loss),
  the new warning codes, and the decision-log D-number.
- The before/after expectation: doctor's 42 `problems` should split into 14 clean
  (Rule A), 12 `artifact_superseded_on_default_branch` warnings (Rule B), and 16
  `artifact_acknowledged_loss` warnings (Rule C) once the operator commits the
  curated baseline — taking `problems` to 0 and `ok` to `true`.
- The ack-file contract: path `docs/operator/doctor-acknowledged-loss.json`,
  schema `striatum.doctor.acknowledged_loss.v1`, id+sha-bound downgrade,
  safe-degrade when absent.
- Any accepted-review findings deferred to follow-ups.
- Operator follow-ups (explicitly, in order): (1) `make install` +
  `systemctl --user restart striatumd` — this is daemon code, inert until the
  running image is replaced; verify `/proc/<MainPID>/exe` is not `(deleted)`. (2)
  Curate + commit the 16-entry `docs/operator/doctor-acknowledged-loss.json`
  baseline. (3) Re-run `striatum doctor` and confirm `ok=true` with 0 problems and
  the warning channel carrying the full story. (4) CHANGELOG entry + close #300.

Do **NOT** merge to `main`. Do **NOT** author the live acknowledged-loss baseline
file — that is operator-curated provenance.
