# Design synthesis: RFC 0018 V1 (review postures)

author: designer-codex-gpt-5.5-001
date: 2026-05-09

## Scope

V1 ships RFC 0018 steps 1+2. Step 3 (`verdicts.posture` column +
introspection surfaces) is **deferred** per the RFC's own
implementation path. The RFC's "build-completion gate" wording is
**re-cast** as a workflow-validation gate per the lifecycle
ambiguity surfaced in `research/POSTURE_SHAPE.md` § "Lifecycle
ambiguity in RFC 0018 step 2." Net behavior matches RFC intent;
enforcement boundary moves from runtime to authoring time.

## 1. Posture vocabulary (closed V1 set)

```python
ALLOWED_POSTURES: frozenset[str] = frozenset({
    "neutral",            # default; today's behavior
    "devils_advocate",
    "security",
    "threat_model",
    "latency_performance",
    "ergonomics_dx",
    "accessibility",
    "compliance_license",
    "supply_chain",
})
```

Off-list postures are accepted via the `custom:<name>` grammar
where `<name>` is a non-empty string. Bare `"custom:"` is
rejected; `"custom:"` followed by all-whitespace is rejected.
There is no normalisation — `"custom:Foo"` and `"custom:foo"` are
distinct postures.

## 2. `POSTURE_INSTRUCTIONS` table

One deterministic sentence per first-class posture, appended to
`review_policy.instruction` after the existing access/context
sentences:

| Posture | Appended sentence |
| --- | --- |
| `neutral` | (no sentence appended; today's behavior) |
| `devils_advocate` | "This is a devil's-advocate review. Argue against the artifact's claims; verdict acceptance means the claims survived your strongest counterarguments." |
| `security` | "This is a security-focused review. Read the artifact looking for security weaknesses; verdict acceptance means you actively looked and found nothing actionable." |
| `threat_model` | "This is a threat-modeling review. Enumerate the trust boundaries and attack surfaces the artifact introduces; verdict acceptance means each is acknowledged or mitigated." |
| `latency_performance` | "This is a latency / performance review. Evaluate the artifact's runtime and resource cost; verdict acceptance means no acceptance-blocking regression was found." |
| `ergonomics_dx` | "This is a developer-ergonomics review. Evaluate the artifact's surface from a first-time-user perspective; verdict acceptance means the affordances are discoverable and consistent." |
| `accessibility` | "This is an accessibility review. Evaluate the artifact against accessibility expectations; verdict acceptance means the affordances meet the declared accessibility bar." |
| `compliance_license` | "This is a compliance / license review. Evaluate the artifact for license, attribution, or compliance issues; verdict acceptance means none are unresolved." |
| `supply_chain` | "This is a supply-chain review. Evaluate the artifact's external dependencies and their provenance; verdict acceptance means each is justified and pinned." |

`custom:<name>` postures get **no** auto-appended sentence — the
workflow author owns the prompt body for off-list postures.

## 3. Step 1: validator (`_validate_review_posture`)

Inserted into `_validate_review_job_fields` (or as a sibling
helper called from `_validate_jobs`):

```python
def _validate_review_posture(job: JsonObject, *, job_id: str) -> None:
    if "review_posture" not in job:
        return
    if job.get("type") != "review":
        raise WorkflowError(
            f"non-review job {job_id!r} cannot declare review_posture"
        )
    posture = job.get("review_posture")
    if not isinstance(posture, str) or not posture:
        raise WorkflowError(
            f"review job {job_id!r} review_posture must be a non-empty string"
        )
    if posture in ALLOWED_POSTURES:
        return
    if not posture.startswith("custom:"):
        raise WorkflowError(
            f"review job {job_id!r} has unknown review_posture {posture!r}; "
            f"allowed: {sorted(ALLOWED_POSTURES)} or custom:<name>"
        )
    custom_name = posture[len("custom:"):]
    if not custom_name.strip():
        raise WorkflowError(
            f"review job {job_id!r} review_posture {posture!r} has empty custom name"
        )
```

All rejections raise `WorkflowError` (exit code 8).

## 4. Step 1: packet exposure

`_build_review_policy` extends to:

```python
def _build_review_policy(workflow, *, workflow_job_id):
    # ... existing job_def lookup ...
    has_access = "reviewer_access_scope" in job_def
    has_context = "reviewer_context_policy" in job_def
    has_posture = "review_posture" in job_def
    if not (has_access or has_context or has_posture):
        return None

    access = job_def.get("reviewer_access_scope") if has_access else "document_only"
    context = job_def.get("reviewer_context_policy") if has_context else "cross_round"
    posture = job_def.get("review_posture") if has_posture else "neutral"

    instruction = (
        _REVIEWER_ACCESS_INSTRUCTIONS[access]
        + _REVIEWER_CONTEXT_INSTRUCTIONS[context]
        + POSTURE_INSTRUCTIONS.get(posture, "")
    )
    block = {
        "access_scope": access,
        "context_policy": context,
        "instruction": instruction,
    }
    if has_posture:
        block["posture"] = posture
    return block
```

The block now triggers for any of the three field declarations.
`posture` is included in the block only when explicitly declared
(omission is byte-identical to today's output for a workflow that
declares only `reviewer_access_scope`).

## 5. Step 2: `required_review_postures` validator

New helper `_validate_required_review_postures`:

```python
def _validate_required_review_postures(job: JsonObject, *, job_id: str) -> None:
    if "required_review_postures" not in job:
        return
    if job.get("type") != "build":
        raise WorkflowError(
            f"non-build job {job_id!r} cannot declare required_review_postures"
        )
    postures = job.get("required_review_postures")
    if not isinstance(postures, list) or not postures:
        raise WorkflowError(
            f"build job {job_id!r} required_review_postures must be a non-empty list"
        )
    for entry in postures:
        if not isinstance(entry, str) or not entry:
            raise WorkflowError(
                f"build job {job_id!r} required_review_postures entries must be non-empty strings"
            )
        if entry in ALLOWED_POSTURES:
            continue
        if not entry.startswith("custom:") or not entry[len("custom:"):].strip():
            raise WorkflowError(
                f"build job {job_id!r} required_review_postures contains invalid entry {entry!r}; "
                f"allowed: {sorted(ALLOWED_POSTURES)} or custom:<name>"
            )
```

## 6. Step 2: workflow-validation gate (re-cast from runtime gate)

After all per-job validators run, walk the edge graph for each
build job that declares `required_review_postures`:

```python
def _validate_required_postures_reachable(workflow: JsonObject) -> None:
    """Each build's required_review_postures must be satisfiable.

    For every build job B with required_review_postures = [P1, P2, …],
    each Pi must be the review_posture of at least one review job R
    such that there is a directed edge path from B to R (B is upstream
    of R) OR from R to B (R is upstream of B). The workflow author
    chose to gate B on these reviews; the runner refuses to start a
    workflow that cannot satisfy the gate.
    """
```

Acceptable patterns:

- **Pre-build review** (dogfood-016 shape): `review_design`
  upstream of `implement`. Satisfies a build's
  `required_review_postures` because the review is reachable
  from the build via the *reverse* edge graph.
- **Post-build review**: `review_build` downstream of
  `implement`. Satisfies because the review is reachable via
  the forward edge graph.

The validator walks both directions (forward edges from the
build, and reverse edges into the build) to find candidate
review jobs, then checks each `required_review_postures` entry is
covered.

Rejection: `WorkflowError` (exit code 8) with message naming the
build, the missing posture, and the available posture set across
reachable reviews.

This runs at `striatum workflow validate` AND at every workflow
load (so `run prepare` also catches it before a run starts). No
runtime gate added — today's edge-verdict mechanism plus the
existing run-completion semantics already enforce that a run
cannot terminate while reviews are unsatisfied.

## 7. Test plan (`tests/test_review_postures.py`)

```
1.  test_review_posture_rejected_on_non_review_job
2.  test_review_posture_rejects_unknown_value
3.  test_review_posture_rejects_empty_string
4.  test_review_posture_rejects_bare_custom_prefix
5.  test_review_posture_rejects_whitespace_only_custom_name
6.  test_review_posture_accepts_each_first_class_value
7.  test_review_posture_accepts_custom_named_value
8.  test_packet_exposes_posture_when_declared
9.  test_packet_omits_posture_when_undeclared
10. test_packet_instruction_appends_posture_sentence_for_first_class
11. test_packet_instruction_unchanged_for_neutral_posture
12. test_packet_instruction_unchanged_for_custom_posture
13. test_required_review_postures_rejected_on_non_build_job
14. test_required_review_postures_rejects_empty_list
15. test_required_review_postures_rejects_unknown_entry
16. test_required_review_postures_rejects_non_list
17. test_required_review_postures_accepts_first_class_entries
18. test_required_review_postures_accepts_custom_entries
19. test_workflow_validates_when_required_posture_is_reachable_via_forward_edge
20. test_workflow_validates_when_required_posture_is_reachable_via_reverse_edge
21. test_workflow_rejects_unreachable_required_posture
22. test_workflow_rejects_when_review_with_required_posture_exists_but_disconnected
23. test_zero_regression_for_posture_omitting_workflow_packet
24. test_zero_regression_for_posture_omitting_workflow_validator
```

## 8. Out of scope (V1)

- `verdicts.posture` column (RFC 0018 step 3, deferred).
- `striatum status` `verdicts_by_posture` block.
- `run summary` per-posture grouping.
- `evidence export` posture surfacing.
- `run graph --format json` posture on review nodes.
- Dashboard verdicts panel posture chips.
- Web UI posture chips.
- Doctor check `build_missing_required_posture_review` (subsumed
  by the workflow-validation gate above).

These all hinge on the `verdicts.posture` column landing first.
V1.5 picks them up after V1 is in operator hands.

## 9. Documentation surface

V1 also updates:

- `docs/SPEC.md` § "Reviewer Policy" — adds the posture
  vocabulary and the validator gate.
- `docs/UBIQUITOUS_LANGUAGE.md` — entries for `review posture`
  and `required review postures`.
- `docs/DECISION_LOG.md` — D069 row for accepting RFC 0018 V1.
- `docs/TODO.md` — F16 row, marked done.
- `docs/rfcs/0018-focused-adversarial-review-postures.md` —
  status `accepted (V1; step 3 deferred)`.
- `docs/rfcs/README.md` — index updated.
- `CHANGELOG.md` — `## 1.7.0 — 2026-05-09` section.
- `pyproject.toml` and `src/striatum/__init__.py` — bump to
  `1.7.0`.

## 10. Confirmation: zero regression

A workflow that omits `review_posture` and
`required_review_postures` produces:

- Identical `WorkflowError` set (no new rejections fire).
- Identical work-packet bytes (the `posture` key is absent;
  `instruction` is unchanged).
- Identical verdict-recording behavior (no schema change).
- Identical run lifecycle (no new gate fires).

Net: V1 is purely additive for posture-omitting workflows.
