---
schema_version: "striatum.harness_improvement_proposal.v1"
artifact_kind: "harness_improvement_proposal"
target: "documentation"
expected_benefit: "Stop telling reviewers to write to a path the runner immediately rejects. Today the reviewer role definition says 'file a harness_improvement_proposal under docs/dogfood/001/findings/', but the review job's write_scope only allows docs/dogfood/001/review/. The publisher correctly refuses with exit 6 ('artifact path is outside the job write scope'); the role doc creates a contradiction the operator only finds at publish time."
risk: "Either the role doc has to change (cheap) or the write_scope has to broaden (riskier — pulls in cross-job write surface). Picking the wrong fix is the only real risk."
rollback: "Revert the doc edit or the scope change."
---

# HARNESS-004 — Reviewer role doc contradicts the review job's write scope

Status: proposed
Run: dogfood-001
Reporter: reviewer (operator-driven; see HARNESS-003 for byline caveat)
Surface: documentation

## Observed friction

`docs/dogfood/001/roles/reviewer.md` line 26 instructs the reviewer:

> If you hit runner friction during review, file a
> `harness_improvement_proposal` under `docs/dogfood/001/findings/`.

The corresponding review job in `docs/dogfood/001/workflow.json` (job
`review_change`) declares:

```json
"write_scope": {
  "mode": "review_only_artifact",
  "repo_write": false,
  "allowed_paths": ["docs/dogfood/001/review/"],
  "forbidden_paths": [".striatum/"]
}
```

So when I tried to publish `docs/dogfood/001/findings/HARNESS-003.md`
from the reviewer session, the runner refused (correctly):

```
striatum publish-artifact --session-id ... --kind harness_improvement_proposal \
  --logical-name harness_003 --path docs/dogfood/001/findings/HARNESS-003.md ...
=> exit 6: artifact path is outside the job write scope
```

The role doc and the workflow disagree about where reviewer-authored
artifacts go.

## Supporting runner evidence

- run_id: `run_a04880660517480a95438fcc0368d2e0`
- job_id: `job_run_a04880660517480a95438fcc0368d2e0_review_change`
- relevant exit code: 6 ("artifact path is outside the job write scope")

## Proposed change

Pick one — both are reasonable:

1. **Doc fix (preferred for this dogfood lane).** Update
   `docs/dogfood/001/roles/reviewer.md` to say "file under
   `docs/dogfood/001/review/HARNESS-NNN.md`" so reviewer harness findings
   live alongside the review finding. Keep the author-side instruction
   ("file under `docs/dogfood/001/findings/`") as is. The write scopes
   already match this layout; only the doc lies.

2. **Workflow fix.** Add `docs/dogfood/001/findings/` to the review
   job's `write_scope.allowed_paths`. This is a wider write surface
   for the reviewer than `review_only_artifact` implies, which is why
   I prefer the doc fix unless the harness-proposal-from-review case
   is going to become routine.

3. **Generalization.** Introduce a workflow-level "harness scratch"
   path (e.g. `docs/dogfood/<id>/harness/`) that is implicitly added
   to every job's allowed_paths so any role can file a harness
   proposal without per-role scope edits. This is the structural fix
   if `harness_improvement_proposal` is meant to be a first-class
   cross-cutting artifact.

## Risk

- Doc fix: zero technical risk; only cosmetic at the cost of having
  reviewer harness proposals filed in a different directory than
  author harness proposals. Operationally fine.
- Workflow fix: pulls cross-job write surface into the reviewer scope;
  trivial here but suggests a pattern that erodes scope discipline.
- Generalization: schema change. Wider blast radius.

## Rollback

- Doc fix: revert the doc edit.
- Workflow fix: revert the JSON change.
- Generalization: needs an RFC; rollback would be reverting the schema
  bump.

## Notes

This is the third "operator hits a wall they could not have predicted
from the runbook" friction point in dogfood-001 (HARNESS-001 was the
supervised lane shape; HARNESS-002 was the editable-install pin;
HARNESS-003 was the independence breach). The pattern: the dogfood
scaffold *says* one thing, the runner *enforces* another, and the
operator only learns the truth at publish time.
