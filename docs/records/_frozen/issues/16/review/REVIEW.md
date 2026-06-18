---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
tags: ["ergonomics_dx", "gh-16", "operator-prompt"]
---

author: reviewer-unknown-model-001

# GH #16 — Operator Initialization Prompt — Ergonomics-DX Verify Review

## Verdict

`accept`. Every bullet in the issue's "Required shape", "Required behavior",
"Required first-action sequence", and "Definition of done" sections is closed
at a concrete location in the new prompt or in the `prompts/README.md`
edits. A fresh AI operator session pasted this prompt has a fillable
parameter block, an explicit required-reading list scoped to current
canonical docs, an enumerated first-action sequence, and named failure
exit codes — without any pointer to historical dogfood prompts. The
boundary prompt is preserved as a focused guardrail and the initialization
prompt links to it for role-adjacent work.

## Posture

Ergonomics-DX, fresh context. Acceptance means a first-time operator
discovers the prompt's affordances and can drive a Striatum run from a
cold paste without reading historical dogfood prompts first.

## Inputs Reviewed

- `docs/issues/16/SPEC.md` (issue body, captured verbatim)
- `prompts/OPERATOR_INITIALIZATION_PROMPT.md` (new)
- `prompts/README.md` (edits)
- `docs/SPEC.md#artifact-front-matter-schemas` (only to confirm
  `striatum.finding.v1` shape for this review's own front matter)

## Bullet-By-Bullet Closure

### Required shape — fill-in block items

| Spec bullet | Closed at |
| --- | --- |
| Striatum repo path | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:18` |
| Striatum version / command path | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:19-21` |
| Target repository path | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:22` |
| Workflow path | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:23` |
| Intended branch / branch-confirmation policy | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:24-25` |
| Existing run id if resuming, else prepare/start | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:26` |
| Daemon/Postgres state | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:27-30` |
| Direct mode allowed for this run | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:31` |
| Required docs to read first | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:32-35` |
| Expected artifact root | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:36` |
| Operator report path and update cadence | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:37-39` |
| MCP/chat vs CLI control surface | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:40` |
| Native sub-agents for read-only audits | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:41-42` |
| Blockers / open GH issues / deferred work | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:43-44` |
| Commit/push policy | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:45-46` |

All fourteen shape bullets are closed.

### Required behavior

| Spec bullet | Closed at |
| --- | --- |
| Read project instructions and canonical docs before acting | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:62-84` (Required Reading) and `:123` (action 1) |
| Check `git status --short --branch` before state-changing work | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:88-89` |
| Confirm Striatum command path/version | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:90-91` |
| Inspect current run/workflow state when resuming | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:97-99` |
| Validate workflow before preparing or starting | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:100` |
| Prepare/start/register/claim/ack/publish/complete only via Striatum CLI or approved MCP/chat tools | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:101-103` |
| Keep role work in role sessions; operator must not author role artifacts | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:54-59`, `:106-107` |
| Use `status` / `why` / `doctor` / `dashboard` / `run summary` / documented recovery for failures | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:108-109`, `:143-149` |
| Update operator report incrementally, especially before compaction or handoff | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:110-112`, `:155-157` |
| Preserve unrelated user changes; never edit `.striatum/` directly | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:89` (preserve), `:113` (never edit) |
| Stop for explicit human decision only at human checkpoints or when prompt-required | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:116-117`, `:151-153` |

All eleven behavior bullets are closed.

### Required first-action sequence

| Step | Closed at |
| --- | --- |
| 1. Load project instructions and listed canonical docs | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:123` |
| 2. Check repository state and Striatum version | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:124-125` |
| 3. Inspect daemon/Postgres or direct-mode status | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:126-127` |
| 4. Validate the workflow | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:128-129` |
| 5. Inspect existing run or prepare new and capture run id | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:130-131` |
| 6. Start the new run or resume execution | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:132-133` |
| 7. Create/update operator report at the filled path | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:134-136` |
| 8. Continue driving until complete or blocked | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:137-140` |

All eight first-action steps are present in order.

### Definition of done

| Spec bullet | Closed at |
| --- | --- |
| `prompts/OPERATOR_INITIALIZATION_PROMPT.md` exists and marked `Status: reusable` | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:3` |
| Complete initialization prompt, not a short boundary warning | Fill-In Block, Mission, Required Reading, Operating Rules, First Action Sequence, Failure And Recovery sections at `prompts/OPERATOR_INITIALIZATION_PROMPT.md:16-157` |
| `prompts/README.md` lists it separately from `OPERATOR_BOUNDARY_PROMPT.md` and explains when to use each | `prompts/README.md:23-32` |
| Old boundary prompt kept as focused guardrail or refactored so init prompt reuses/points to it | Init prompt points to boundary prompt at `prompts/OPERATOR_INITIALIZATION_PROMPT.md:54`, `:81`; README keeps boundary prompt with focused use-case at `prompts/README.md:28-32` |
| Generic; no RFC0026/RFC0027 or Engram paths | No occurrences of "RFC0026", "RFC0027", or "Engram" anywhere in the new prompt; only RFC 0043 and RFC 0048 referenced, both for the current daemon/Postgres transition |
| Reflects current daemon/Postgres transition, including RFC0048 caveat | `prompts/OPERATOR_INITIALIZATION_PROMPT.md:92-96` names RFC 0043 V1 status and the `STRIATUM_DAEMON_REQUIRED=0` / `STRIATUM_TEST_HARNESS=1` test-harness escape scheduled for removal in RFC 0048 phase C |
| Fresh operator can use it to start/resume without historical dogfood prompts | Self-contained fill-in + required-reading + first-action sequence; `prompts/OPERATOR_INITIALIZATION_PROMPT.md:83-84` explicitly says not to read historical dogfood prompts unless the fill-in block or human puts them in scope |

All seven Definition-of-done bullets are closed.

## Ergonomics Walkthrough — Fresh Session Test

Imagining a fresh AI session pasted this prompt as its only initialization:

- The fill-in block at `prompts/OPERATOR_INITIALIZATION_PROMPT.md:16-46`
  enumerates exactly what the human must supply before pasting. The order
  mirrors what the operator needs first (repo paths and command) and ends
  with policy items (sub-agents, commits) that affect later phases.
- Required Reading at `:62-84` enumerates current canonical docs by
  repo-relative path and gives a short reason for each. `:83-84` closes
  the obvious failure mode of an AI session pulling in historical
  dogfood material because the doc tree contains it.
- Operating Rules at `:86-119` and First Action Sequence at `:121-140`
  are consistent: each behavioral rule has a corresponding action step,
  and both lists name the same Striatum verbs (`status`, `why`,
  `dashboard`, `run summary`, `doctor`).
- Failure And Recovery at `:142-157` names stable exit codes (5 for
  lease/state, 6 for artifact validation) so a fresh session can map a
  CLI failure to a documented path without guessing.
- Daemon/Postgres status at `:92-96` is the most volatile section of
  the prompt by content type; it is correctly hedged to the current
  state (RFC 0043 V1 landed, default not yet flipped) and points the
  reader at the test-harness escape's RFC 0048 phase-C removal so the
  prompt does not silently age.

A fresh session pasted this prompt would be able to begin without
external context beyond the human's filled block and the named canonical
docs. The ergonomics affordances are discoverable and consistent. The
prompt does not require operator intuition to bridge gaps.

## Non-Blocking Observations

These are informational; they do not lower the verdict.

- The Required Reading section at `prompts/OPERATOR_INITIALIZATION_PROMPT.md:62-84`
  duplicates the fill-in block's docs list at `:32-35`. The duplication
  is intentional and helpful (the fill-in block is the human's checklist;
  Required Reading is the operator's), but it does mean future doc-set
  changes need to be applied in both places.
- The fill-in block bullet "Daemon/Postgres state" at `:27-30` and
  "Direct mode allowed for this run" at `:31` are slightly redundant
  when the state is already "direct mode against repo-local runner
  state". Not worth tightening; the redundancy is a clarity affordance
  for the human filling the block.

## Scope Notes

This review was conducted under `review_policy.context_policy: fresh`
and `access_scope: document_only`. The only file consulted beyond the
three target documents was `docs/SPEC.md#artifact-front-matter-schemas`
(to confirm the `striatum.finding.v1` front-matter shape for this
artifact). No prior thread state from earlier rounds was relied upon.
