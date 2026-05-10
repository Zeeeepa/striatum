# Implement Prompt

Implementation is blocked until `review_design_devils`, `review_design_security`, and `review_design_threat` all have accepting verdicts. Do not start implementation from the raw RFCs alone.

After the gate opens, implement only the accepted scope in `docs/dogfood/030/DESIGN_SYNTHESIS.md` and the resolved design review findings. Stay inside the workflow write scope.

Expected behavior:

- update runner code, migrations, validators, CLI/API surfaces, status/doctor/evidence/run-summary/web surfaces, docs, and tests only as required by the accepted plan;
- preserve existing advisory workflows unless the accepted plan explicitly changes behavior and documents compatibility;
- add adversarial tests for false bylines, unattested sessions, digest binding, apply eligibility, path scope, and receipt/provenance claims as applicable to the accepted phase;
- update `docs/SPEC.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `docs/DECISION_LOG.md`, `docs/TODO.md`, RFC docs, `README.md`, `CHANGELOG.md`, and `docs/rfcs/README.md` when behavior or status changes;
- do not claim sealed source-write containment unless the implementation mechanically denies operator writes to protected paths on the supported platform.

Produce `docs/dogfood/030/BUILD_HANDOFF.md` summarizing changes, tests run, compatibility notes, deferred scope, and any human decisions still required.
