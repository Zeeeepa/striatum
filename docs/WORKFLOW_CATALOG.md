# Workflow Template Catalog

Generated from the bundled Striatum workflow template catalog.

## Workflow Shapes

### Code change with bounded revision (`code_change`)

Draft, review, revise at most once if needed, then apply.

- Recommended for: small code or docs edits that need an explicit review gate
- Default lane sets: `author_reviewer`, `single_agent`
- Required options: `workflow_id`, `artifact_root`

```mermaid
flowchart TD
  n0["Draft"]
  n1["Review"]
  n2["Apply"]
  n0 --> n1
  n1 --> n2
  n1 -.->|needs_revision| n0
```

### Custom safe block plan (`custom`)

Compose a workflow from known block kinds without raw workflow JSON.

- Recommended for: advanced operators who need a graph not covered by a built-in shape
- Default lane sets: `custom`
- Required options: `plan`, `workflow_id`, `artifact_root`

No fixed graph preview.

### Evidence-backed artifact (`evidence_backed`)

Produce claims with a support ledger and audit review.

- Recommended for: artifacts whose claims need explicit evidence checking
- Default lane sets: `author_reviewer`
- Required options: `workflow_id`, `artifact_root`

```mermaid
flowchart TD
  n0["Draft"]
  n1["Support ledger"]
  n2["Evidence audit"]
  n3["Final review"]
  n0 --> n1
  n1 --> n2
  n2 --> n3
```

### Human checkpoint (`human_checkpoint`)

Require owner judgment before downstream work proceeds.

- Recommended for: runs that need an explicit operator decision gate
- Default lane sets: `author_reviewer`, `single_agent`
- Required options: `workflow_id`, `artifact_root`

```mermaid
flowchart TD
  n0["Analysis"]
  n1["Checkpoint"]
  n2["Apply"]
  n0 --> n1
  n1 --> n2
```

### Minimal bounded job (`minimal`)

One bounded job for a small report or starter artifact.

- Recommended for: small reports; narrow inspections; first drafts
- Default lane sets: `local`, `single_agent`
- Required options: `workflow_id`, `artifact_root`

```mermaid
flowchart TD
  n0["Draft"]
```

### Multi-phase workflow (`multi_phase`)

Run phase-scoped parallel tracks behind explicit synthesis gates.

- Recommended for: large work split into ordered design, build, review, or release phases
- Default lane sets: `author_reviewer`, `multi_review`, `single_agent`
- Required options: `workflow_id`, `artifact_root`, `phases`

```mermaid
flowchart TD
  n0["Phase 1 track"]
  n1["Phase 1 synthesis"]
  n2["Phase 2 track"]
  n3["Phase 2 synthesis"]
  n0 --> n1
  n1 --> n2
  n2 --> n3
```

### Multi-review synthesis (`multi_review_synthesis`)

Collect several independent reviews before a final recommendation.

- Recommended for: productive disagreement; RFC or proposal review
- Default lane sets: `multi_review`
- Required options: `workflow_id`, `artifact_root`

```mermaid
flowchart TD
  n0["Review 1"]
  n1["Review 2"]
  n2["Synthesis"]
  n3["Final review"]
  n0 --> n2
  n1 --> n2
  n2 --> n3
```

### Review and synthesis (`review`)

Draft, fresh review, then final synthesis.

- Recommended for: proposal review; bug triage; documentation review
- Default lane sets: `author_reviewer`, `single_agent`, `local`
- Required options: `workflow_id`, `artifact_root`

```mermaid
flowchart TD
  n0["Draft"]
  n1["Review"]
  n2["Synthesis"]
  n0 --> n1
  n1 --> n2
```

## Lane Sets

### Separate author and reviewer (`author_reviewer`)

Authoring jobs and review jobs bind to separate lanes.

- Recommended for: independent review for a code or docs change
- Required options: `lanes.author.command`, `lanes.reviewer.command`

### Custom lane bindings (`custom`)

Advanced lane topology with every binding declared.

- Recommended for: custom plans with explicit job-to-lane bindings
- Required options: `lanes`, `plan.job_lane_bindings`

### Local fixture lane (`local`)

Fixture/operator-by-hand starter lane that validates without a real model command.

- Recommended for: tests; operator-by-hand runs; starter scaffolds

### Multiple reviewers (`multi_review`)

One author lane plus several reviewer lanes.

- Recommended for: productive disagreement through multiple review postures
- Required options: `lanes.author.command`

### Single agent (`single_agent`)

One real agent session handles the whole workflow.

- Recommended for: small low-risk work; early adoption
- Required options: `lanes.agent.command`
