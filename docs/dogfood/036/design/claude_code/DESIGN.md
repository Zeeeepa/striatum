# RFC 0034 Workflow Generator: Implementation Design

author: designer-claude-opus-001

## 1. Posture and Scope

This design implements RFC 0034 V1 — generator core, CLI surface, and local
API surface — through the **ergonomics_dx** lens: the reviewer will judge
whether a first-time operator can discover, choose, preview, and save a
working `workflow.json` without scraping prose or memorizing a JSON schema.

V1 ships **CLI + Python API + local API only**. The web `/workflows/new`
chooser flow (RFC 0034 §9), the chat-assisted scaffolding tool (RFC 0034
§10), the target-repo local catalog extension (RFC 0034 §6 V1.5), and any
automatic repository inspection are **explicitly deferred** to a follow-up
dogfood. They are named in §13 so the synthesis can carry the deferral
forward without re-litigation.

The trust/safety boundary stays light per the posture: JSON-only generator
output, refuse-to-overwrite on writes, validator gate, no hosted
marketplace, no telemetry, no remote template fetch. Hardening (signed
templates, supply-chain provenance, multi-operator concurrency) is out of
scope.

## 2. First-Time-Operator Storyboard

This is the path V1 is judged against. Every section below maps back to a
step in this story.

```
                      striatum workflow --help
                                │
                                ▼
              striatum workflow templates list
                                │   (sees six shapes; picks one)
                                ▼
              striatum workflow templates show code_change
                                │   (sees recommended lane sets; picks one)
                                ▼
   striatum workflow generate <path> --shape code_change \
       --lane-set author_reviewer --artifact-root striatum/my-change \
       --dry-run --json
                                │   (sees envelope; reads warnings)
                                ▼
              (drops --dry-run; runs the same command)
                                │
                                ▼
        striatum workflow validate <path>/workflow.json
                                │
                                ▼
        striatum workflow graph <path>/workflow.json
                                │
                                ▼
                          (commits files)
```

Five named choices, each surfaced by an obvious next command, each
producing a structured envelope an AI client can also consume. The
operator never needs to open `docs/SPEC.md`.

## 3. Generator Core (`src/striatum/workflow_generator/`)

A new package because the surface is large enough to warrant separation
from `workflow.py` (which already owns validation, planning, and graph
data) but small enough to keep flat.

```
src/striatum/workflow_generator/
  __init__.py            # public: generate_workflow, exceptions, spec types
  spec.py                # WorkflowGenerationSpec dataclass + JSON adapters
  envelope.py            # GeneratedWorkflow dataclass + JSON adapter
  errors.py              # GeneratorError, field_path-bearing subclasses
  catalog.py             # load catalog.json package data; lookups by id
  shapes/                # one module per shape compiler
    __init__.py          # registry: shape_id -> compile fn
    minimal.py
    review.py
    code_change.py
    human_checkpoint.py
    evidence_backed.py
    multi_review_synthesis.py
    custom.py            # plan compiler; closed block vocabulary
  lane_sets/             # one module per lane-set compiler
    __init__.py          # registry: lane_set_id -> compile fn
    local.py
    single_agent.py
    author_reviewer.py
    multi_review.py
    custom.py
  modifiers.py           # supervised | worktree_isolated | constrained |
                         # harness_profiled, layered onto a base lane set
  scaffold.py            # role/prompt stub authoring (no I/O)
  validate.py            # post-compile pass that calls workflow.validate_workflow
```

### 3.1 Public API

```python
from striatum.workflow_generator import (
    WorkflowGenerationSpec,
    GeneratedWorkflow,
    GeneratorError,
    generate_workflow,
)

def generate_workflow(spec: WorkflowGenerationSpec) -> GeneratedWorkflow:
    """Compile a generation spec to a validated workflow tree.

    Pure: no filesystem writes, no network. Caller chooses to write the
    returned files or to discard them (`--dry-run`).
    """
```

`WorkflowGenerationSpec` is a frozen dataclass that round-trips JSON via
`spec.to_json()` / `WorkflowGenerationSpec.from_json(obj)`. Fields match
the RFC §2 example verbatim; unknown keys are rejected with a
`field_path`-bearing error rather than silently dropped. This is the
contract the CLI, local API, and chat tool all use.

`GeneratedWorkflow` is the result envelope (RFC §2): `workflow` (the
compiled `workflow.json` dict), `files` (list of `{path, content}`),
`metadata` (shape, lane_set, graph preview), `warnings` (non-fatal
notes), and a `validation` block populated by `validate.py`. Same shape
returned to CLI, Python callers, and HTTP clients.

### 3.2 Compilation pipeline

The pipeline is a single forward pass with five named stages. Each stage
either advances or raises a `GeneratorError` with a `field_path`. The
stages are deliberately small so the failure message points the operator
at the exact spec field to fix.

```
spec
  │
  ▼  stage 1: parse & schema-check spec (errors include field_path)
  ▼  stage 2: resolve shape  -> shape compile fn (errors: spec.shape)
  ▼  stage 3: resolve lane_set + modifiers (errors: spec.lane_set / .lane_modifiers[i])
  ▼  stage 4: shape.compile(spec, lane_plan) -> workflow dict + scaffold files
  ▼  stage 5: validate_workflow(workflow) (errors: bubble up workflow-side field_path)
  │
  ▼
GeneratedWorkflow{workflow, files, metadata, warnings, validation}
```

Stages 2-4 may emit `warnings` instead of erroring when a combination is
sub-optimal but coherent: e.g. `worktree_isolated` on a review-only shape
is a warning ("no repo-write lanes; modifier is a no-op"); `supervised`
without a lane command is a stage-3 error ("supervised requires
`lanes.<lane>.command`"). The warnings list propagates into the envelope
so the CLI and UI can print them without the operator scraping stderr.

### 3.3 Custom shape

`shape: custom` reads the constrained plan document from RFC §5 and runs
through the same five-stage pipeline. The plan compiler in
`shapes/custom.py` owns:

- Closed block vocabulary: `draft | review | synthesis | implementation
  | test | human_checkpoint | support_ledger | evidence_audit |
  final_review`. Unknown kinds are stage-1 errors with
  `field_path=plan.blocks[i].kind`.
- Cycle bounding: every cycle must declare `max_iterations` and the
  edge's source must be a `review` block.
- Lane-binding gate: every block id in `job_lane_bindings` must exist in
  the plan and resolve to a lane in the compiled lane plan.
- Path-safety gate: any `write_scope.allowed_paths` derived from block
  configuration must be repo-relative and outside `.striatum/`.

This gives operators an escape hatch without making "custom" synonymous
with "free-hand JSON the validator hasn't seen."

### 3.4 Catalog package data

`src/striatum/workflow_templates/catalog.json` is the **only** catalog
source in V1. Layout matches RFC §6: one entry per shape, one entry per
lane set. The CLI, local API read endpoints, and tests all load the same
file via `importlib.resources.files("striatum.workflow_templates")`.

Discoverability requires that `display_name`, `summary`, and
`recommended_for` are **specific and actionable** (RFC reviewer test).
The catalog ships values like:

```json
{
  "template_id": "code_change",
  "kind": "shape",
  "display_name": "Code change with bounded revision",
  "summary": "Draft, review, revise at most once if needed, then apply.",
  "recommended_for": [
    "small code edit that needs an explicit review gate",
    "docs change where one revision round is expected",
    "focused bug fix with one author and one reviewer"
  ],
  "default_lane_sets": ["author_reviewer", "single_agent"],
  "required_options": ["workflow_id", "artifact_root"],
  "graph_preview": {"nodes": [...], "edges": [...]}
}
```

Tests in `tests/test_workflow_templates_catalog.py` assert that every
`recommended_for` entry is at least one full clause, no boilerplate like
"general code changes" survives, and every shape declares at least one
compatible lane set. This is enforced by the test suite so future catalog
edits cannot drift back into vague metadata.

### 3.5 Testing

- `tests/test_workflow_generator_shapes.py` — every shape × every
  compatible base lane set validates. ~24 combinations; the test is a
  parametrize matrix.
- `tests/test_workflow_generator_modifiers.py` — each modifier layers
  correctly on its compatible bases, and each incompatible combo raises a
  `GeneratorError` with the expected `field_path`.
- `tests/test_workflow_generator_custom.py` — closed block vocabulary,
  unbounded cycle refusal, unknown block kind, invalid lane bindings,
  unsafe paths.
- `tests/test_workflow_generator_envelope.py` — JSON round-trip for spec
  and envelope; unknown spec key rejected.
- `tests/test_workflow_templates_catalog.py` — catalog content guards
  (above).

Every passing test ends in a `validate_workflow(envelope.workflow)`
assertion. Generator success without validator success is a bug and the
test suite refuses to encode it.

## 4. CLI Surface (`src/striatum/cli/workflow.py`)

A new module (the file does not exist today; today
`workflow_init` lives at `src/striatum/cli/workflow_init.py` and
`workflow validate/plan/graph` are wired in `dispatch.py`). Creating
`cli/workflow.py` collects the verb cluster in one place. `dispatch.py`
delegates the `workflow` subcommand to a router in this new module;
`workflow init` remains in `workflow_init.py` and the router forwards to
it so `init`'s behavior is unchanged on the import surface.

### 4.1 Verb additions

```
striatum workflow templates list [--kind shape|lane_set] [--json]
striatum workflow templates show <template_id> [--json]
striatum workflow generate <path>
    --shape <shape>
    --lane-set <lane_set>
    --artifact-root <repo-relative-path>
    [--lane-modifier <m>]...
    [--plan <plan-path>]
    [--workflow-id <id>]
    [--name <name>]
    [--scaffold-root <repo-relative-path>]
    [--branch-suggestion <name>]
    [--option key=value]...
    [--dry-run]
    [--force]
    [--json]
```

Required vs optional is named in the **help text and in argparse
required-set**. The verb refuses to run without `--shape`, `--lane-set`,
and `--artifact-root`; the error message is `field_path`-bearing so an AI
client can repair the call:

```
$ striatum workflow generate workflows/my-change --shape code_change
error: missing required spec field
  field_path: spec.artifact_root
  hint: pass --artifact-root <repo-relative-path>; see
        `striatum workflow templates show code_change` for examples.
```

### 4.2 Defaults that protect first-time operators

- **`--dry-run` is the documented first pass.** `templates show` ends
  with the exact `generate --dry-run` invocation the operator can paste.
  The non-dry-run write is one keystroke away (drop the flag) but never
  the first thing demonstrated.
- **`--force` is reserved.** V1 refuses to overwrite an existing path
  (parity with current `workflow init`); the verb prints a non-fatal
  `--force` mention only when it detects a conflict, so the operator
  finds it the moment they need it instead of seeing it on every run.
- **`workflow init --style minimal|review|code-change` still works.**
  Its dispatch is rewired to construct a `WorkflowGenerationSpec` with
  `lane_set: "local"` and the current single-lane placeholder, then call
  `generate_workflow`. The CLI output shape (the dict printed by
  `workflow_init`) is preserved: `{status, path, workflow_path, style,
  files}` — same keys, same human-readable lines. Existing tests in
  `tests/` that assert on this shape continue to pass without changes.
- **`--option key=value` carries nested keys** via dotted paths
  (`--option options.review_postures=devils_advocate`). Repeated flags
  merge into a dict; conflicting keys are an error with `field_path`.

### 4.3 Help text rules

Every new verb's help has three sentences and exactly one example.
Sample for `workflow generate`:

```
Generate a workflow tree from a shape, a lane set, and an artifact root.
Validates the result before returning. Use --dry-run on the first pass
to preview JSON and files without writing.

Example:
  striatum workflow generate workflows/my-change --shape code_change \
      --lane-set author_reviewer --artifact-root striatum/my-change \
      --dry-run --json
```

`templates list` example is the natural "obvious starting point" the
prompt asks for: it's the first thing `workflow --help` mentions in its
description block.

### 4.4 Error envelope

CLI errors emit the same dict shape on `--json`:

```json
{
  "ok": false,
  "error": {
    "code": 8,
    "message": "missing required spec field",
    "field_path": "spec.artifact_root",
    "hint": "pass --artifact-root <repo-relative-path>",
    "ref": "striatum workflow templates show code_change"
  }
}
```

Exit codes reuse the existing table from `docs/CLI_REFERENCE.md`:
generator errors map to **8** (workflow config rejected). Catalog
lookups that miss return **3** (missing target). Lease/state errors
cannot arise from this verb — it does not touch SQLite — which keeps the
mental model "generate is a pure compile step" intact for the operator.

### 4.5 Human surface mirrors JSON

The human prose printed without `--json` uses the same field labels as
the JSON envelope. Example:

```
$ striatum workflow generate ...
generated: workflows/my-change/workflow.json
files:
  workflows/my-change/workflow.json
  workflows/my-change/roles/author.md
  workflows/my-change/roles/reviewer.md
  workflows/my-change/prompts/draft.md
  workflows/my-change/prompts/review.md
  workflows/my-change/prompts/apply.md
warnings: (none)
validation: ok
```

`field_path` strings in human errors match the JSON exactly so an
operator pasting from the terminal into a chat tool sees the same
identifier the AI tool consumed.

## 5. Local API Surface

The generator must be reachable from AI / operator-surrogate clients
without screen-scraping the CLI. The local service exposes the same
generator through HTTP and returns the same envelope shape.

### 5.1 Routes

```
GET  /workflow-templates                  -> 200 envelope (read; safe)
GET  /workflow-templates/<template_id>    -> 200 envelope | 404
POST /workflows/generate/preview          -> 200 envelope (non-mutating)
POST /workflows/generate                  -> 200 envelope (mutation-gated)
```

`POST /workflows/generate` requires `confirm_write: true` in the request
body AND the service to have been started with `--allow-mutations`.
Either missing is a 4xx with `field_path: confirm_write` or
`field_path: server.allow_mutations` respectively. The preview endpoint
is **always available**, mutations gate or not — AI operators get a safe
read path that returns the exact envelope a CLI dry-run would produce.

### 5.2 Wiring into `service.py`

`service.py` already routes path prefixes by dispatching on
`parsed.path.startswith(...)` (see `_render_workflow_*` at lines
543-572 and the mutation guards near 1103, 1200, 1217). The new routes
follow the same pattern:

- Add four prefix branches in the GET handler for `/workflow-templates`
  (exact and `/<id>`) and in the POST handler for
  `/workflows/generate/preview` and `/workflows/generate`.
- Each handler is a thin adapter that loads JSON from the request body,
  constructs `WorkflowGenerationSpec.from_json(body)`, calls
  `generate_workflow(spec)`, and returns the envelope.
- Mutation-gated handlers reuse the existing `if not
  self.state.allow_mutations:` guard pattern that the workflow-edit and
  workflow-run-now handlers already use.

Route registration stays declarative-by-convention (no new framework or
router abstraction). Adding endpoints means adding two branches each in
the existing GET / POST dispatchers in `service.py`.

### 5.3 Response shape parity

```json
{
  "ok": true,
  "data": {
    "workflow": { ... },
    "files": [ {"path": "...", "content": "..."} ],
    "metadata": {"shape": "code_change", "lane_set": "author_reviewer", "graph": {}},
    "warnings": [],
    "validation": {"ok": true}
  }
}
```

The CLI's `--json` output is `{"ok": true, "data": <envelope>}`, the
same wrapper Striatum already uses (see `dispatch.py:100`). The HTTP
endpoint returns the same. A chat tool that pretty-prints the CLI dry-run
and a chat tool that pretty-prints the HTTP preview render the same
content — that's the **one generator, one envelope** invariant.

### 5.4 Error envelope

Errors reuse the existing service error envelope:

```json
{
  "ok": false,
  "error": {
    "code": 8,
    "message": "incompatible lane modifier for shape",
    "field_path": "spec.lane_modifiers[0]",
    "hint": "modifier 'supervised' requires lanes.<id>.command",
    "ref": "GET /workflow-templates/supervised"
  }
}
```

`field_path` is REQUIRED on every validation-style error. An AI client
should be able to:

1. Read `field_path`.
2. Mutate the spec at that path.
3. Re-call `/workflows/generate/preview`.
4. Loop until `ok: true`.

No prose scraping. This is the "structured field errors" criterion in the
RFC acceptance list and is enforced by tests:
`tests/test_workflow_generator_errors.py` asserts that every error
constructor sets `field_path` (or explicitly marks `field_path=None`
for the rare non-field error case, e.g. catalog file missing on disk).

## 6. Backwards Compatibility

`workflow init --style minimal|review|code-change` must keep working
exactly as it does today. The implementation path:

1. `workflow_init` (the function, not the verb) is rewritten as a thin
   shim that constructs a `WorkflowGenerationSpec`, calls
   `generate_workflow(spec)`, and writes `envelope.files` to disk.
2. The dict it returns (`{status, path, workflow_path, style, files}`)
   stays byte-identical for the three legacy styles, so callers that
   parse it (CLI dispatch, tests, supervised-init flows) see no change.
3. The starter workflow JSON produced for each legacy style matches the
   current `_starter_workflow` output exactly. A snapshot test —
   `tests/test_workflow_init_backcompat.py` — pins this: it loads
   today's output, runs the new generator-backed `workflow_init`, and
   asserts the JSON dicts are equal. If the snapshot needs to change,
   it's a deliberate decision recorded in `docs/DECISION_LOG.md`.

The user-visible surface (`workflow init --style review path/to/x`)
prints the same lines, exits with the same code, and creates the same
files. An operator who learned the old verb yesterday does not see a
surprise tomorrow.

## 7. Discoverability — How a First-Time Operator Finds the Generator

The reviewer's heuristic: "if I have never used Striatum, do I find
this in under five minutes?" The design has four pegs.

1. **`striatum workflow --help` surfaces the new verbs.** The help
   description leads with: "Generate a workflow tree from a shape and a
   lane set. Start with `workflow templates list`." That sentence
   appears in the parser's description for the `workflow` subcommand.
2. **`workflow templates list` is the obvious starting point.** No
   flags required. Default output is short prose: shape id, display
   name, one-line summary, "recommended for" hints. The
   `--json` variant returns the same data structurally.
3. **`workflow templates show <id>` ends with a runnable command.**
   The last line of `show`'s human output is the exact `generate
   --dry-run` invocation, with the recommended lane set pre-filled. The
   operator copies one line, pastes, and sees a real envelope.
4. **`docs/WORKFLOW_TYPES.md` adds a "Generator workflow" section.**
   The roadmap section already points at RFC 0034; this dogfood
   replaces that "roadmap to a chooser" stub with a "use the generator"
   section that shows the CLI flow above. `docs/CLI_REFERENCE.md` gains
   a `workflow templates` and `workflow generate` block in its core
   lifecycle list.

These are the four places a first-time operator looks for "what do I do
next?": `--help`, the templates verb, the show verb, and the docs. Each
one points forward to the next named step. No grep, no SPEC.md.

## 8. Avoiding Combinatorial Paralysis

Six shapes × five lane sets × four modifiers = 120 combinations on
paper. The design uses four mitigations so the operator never feels that
choice space.

1. **Shape and lane set are separate decisions.** RFC §1 calls this out;
   the CLI enforces it by making them separate required flags. The
   operator picks shape first (graph intent), then lane set (execution
   topology). The wizard pattern in plain CLI form.
2. **Default lane sets per shape.** Catalog metadata
   (`default_lane_sets`) lists the small set of lane sets that fit a
   given shape. `templates show <shape>` prints those and only those by
   default; `--all-lane-sets` is the escape hatch for the curious. The
   operator sees two or three sensible choices, not five.
3. **`recommended_for` is the per-template hint.** Each entry has two or
   three concrete clauses (e.g. "small code edit that needs an explicit
   review gate"). The operator reads three sentences and picks the one
   matching their task description.
4. **Modifiers are explicitly optional and additive.** First-time
   operators don't see modifiers at all in the recommended path —
   `templates show` lists them under a clearly-marked "Optional lane
   modifiers" section, with one sentence each. The default `generate`
   invocation has no `--lane-modifier`. Advanced operators discover them
   when they need them.

The first-time path is: pick from six shapes, pick from two or three
recommended lane sets, run with `--dry-run`. Three decisions, two of
which have curated short lists. Modifiers and `--plan` are for the second
visit.

## 9. Custom Shape Without Free-Hand JSON

`shape: custom` is the escape hatch. The design keeps it explicit:

- `--shape custom` requires `--plan <path>` (parser-enforced; missing it
  is a usage error with `field_path: spec.plan`).
- `--plan` accepts a JSON file matching `striatum.workflow_plan.v1` (RFC
  §5). The plan compiler in `shapes/custom.py` runs the closed-block
  vocabulary and the safety gates listed in §3.3.
- `templates list --kind shape` includes `custom` with a `summary` that
  warns: "Compose from known block kinds; see `templates show custom`
  for the closed block vocabulary." `templates show custom` lists the
  nine block kinds with one-line descriptions.
- The plan file format has its own schema entry in the catalog so a chat
  tool or AI client can produce a valid plan from structured input.

`custom` is intentionally less ergonomic than the named shapes — the
operator must author a small plan file — because "custom" should feel
like more work than "code_change", not the same amount. That preserves
the gravitational pull toward the named shapes for first-time users.

## 10. CLI Ergonomics — Field Names and Flag Discipline

Naming follows the RFC §2 spec keys verbatim so the CLI, JSON envelope,
catalog metadata, and error messages share one vocabulary.

| CLI flag                  | Spec field             | Required |
|---------------------------|------------------------|----------|
| `--shape`                 | `spec.shape`           | yes      |
| `--lane-set`              | `spec.lane_set`        | yes      |
| `--artifact-root`         | `spec.artifact_root`   | yes      |
| `--lane-modifier <m>`     | `spec.lane_modifiers[]`| no       |
| `--workflow-id`           | `spec.workflow_id`     | no (auto from path tail) |
| `--name`                  | `spec.name`            | no (auto from id) |
| `--scaffold-root`         | `spec.scaffold_root`   | no (defaults to `<path>`) |
| `--artifact-root`         | `spec.artifact_root`   | yes      |
| `--branch-suggestion`     | `spec.branch.suggested_name` | no |
| `--plan`                  | `spec.plan`            | required iff `--shape custom` |
| `--option key=value`      | `spec.options.<key>`   | no       |
| `--dry-run`               | (CLI-only)             | no       |
| `--force`                 | (CLI-only; reserved)   | no       |
| `--json`                  | (CLI-only)             | no       |

The rule "every required flag has a `field_path`-bearing error if
missing" is enforced by `tests/test_workflow_generator_cli.py`. The rule
"every flag listed here maps to exactly one spec key" is enforced by a
test that parses an empty `WorkflowGenerationSpec`, applies each flag,
and asserts the resulting dict matches.

## 11. Local API for AI Clients — Worked Example

A chat client that wants to scaffold a code-change workflow follows the
same loop the human operator does, but via HTTP.

```
1. GET /workflow-templates?kind=shape
   -> 200 list of shapes with display_name, summary, recommended_for.
   AI picks `code_change`.

2. GET /workflow-templates/code_change
   -> 200 catalog entry incl. default_lane_sets and required_options.
   AI picks `author_reviewer`.

3. POST /workflows/generate/preview
   {"spec": {
     "schema_version": "striatum.workflow_generator.v1",
     "shape": "code_change",
     "lane_set": "author_reviewer",
     "artifact_root": "striatum/my-change",
     "workflow_id": "my-change",
     ...
   }}
   -> 200 envelope; AI shows the file list and graph to the operator.

4. (Operator approves in chat UI; AI sets confirm_write: true.)

5. POST /workflows/generate
   {"spec": {...same...}, "confirm_write": true}
   -> 200 envelope incl. validation.ok = true; files written to disk.

6. (On error at any step:)
   -> 4xx envelope with field_path; AI mutates spec at that path and
      re-calls /workflows/generate/preview.
```

Step 6 is the load-bearing one for AI ergonomics. Without `field_path`
the AI scrapes prose; with it, the AI loops on structured signal until
the spec validates. Tests in
`tests/test_service_workflow_generate.py` cover the preview→error→fix
loop with deterministic specs.

## 12. Trust / Safety Boundary (Light, per Posture)

Per the ergonomics_dx posture, safety is **scoped to "don't surprise the
operator"**, not "block all classes of attack":

- **JSON-only output.** Generator never returns or writes anything that
  isn't a `workflow.json` dict or a stub Markdown file. No shell scripts,
  no binary blobs.
- **Validator gate.** Every successful envelope has run through
  `validate_workflow`. Generator-success-without-validator-success is
  treated as a bug in the test suite.
- **Refuse-to-overwrite.** Non-dry-run writes refuse if any target file
  exists. `--force` is documented but not implemented in V1 (the verb
  prints the flag name in the conflict error message; the parser does
  not accept it yet so the error is a usage error). Implementing
  `--force` is a separate future RFC.
- **Path safety.** All paths in `files` are repo-relative; the writer
  refuses paths that contain `..`, start with `/`, or land under
  `.striatum/`. Same gates `workflow validate` already enforces.
- **No mutations on the preview endpoint.** `POST /workflows/generate/
  preview` does not need `--allow-mutations` and does not write. AI
  clients can preview freely.
- **Mutation gate on the write endpoint.** `POST /workflows/generate`
  requires both `--allow-mutations` on the server and `confirm_write:
  true` in the body. This is the same double-gate the existing
  `/workflows/edit/<path>` POST uses.
- **No telemetry, no hosted marketplace, no remote template fetch.** The
  catalog is local package data and the only catalog read path is
  `importlib.resources`. There is no HTTP client in the generator.

Out of scope for V1: per-template signing, supply-chain provenance for
plan files, multi-operator concurrency, and adversarial spec fuzzing.
Those belong in a hardening RFC if first-contact dogfooding reveals a
need.

## 13. Explicitly Deferred

The synthesis should carry these deferrals forward without
re-litigation. Each is named here so the design's V1 scope is
unambiguous.

1. **Web `/workflows/new` chooser UI (RFC 0034 §9).** The visual flow
   that lets an operator click through shape → lane set → required
   fields → preview graph → review JSON → save. Deferred to a follow-up
   dogfood. The local API endpoints this design ships are the contract
   the UI will eventually call.
2. **Chat-assisted scaffolding tool (RFC 0034 §10).** A closed chat tool
   that wraps the local API. Deferred. The error-envelope `field_path`
   contract this design ships is the contract that tool will eventually
   consume.
3. **Target-repo local catalog extensions (RFC 0034 §6 V1.5).** Allowing
   a target repo to drop extra catalog entries under
   `<repo>/.striatum/workflow-templates/`. Deferred. The catalog loader
   is structured so a second loader can layer over the package-data
   loader without changing the public API.
4. **Automatic repository inspection / suggested shapes.** The GitHub
   Actions "we noticed your repo has a Dockerfile" pattern. Deferred and
   left for a hardening pass. V1 keeps selection entirely explicit.
5. **`--force` overwrite flag.** Reserved name; not implemented in V1.
6. **Durable `workflow.generator.json` source plan alongside generated
   `workflow.json`** (RFC §11 open question). Deferred. Generated
   workflow JSON is the only durable source in V1.

## 14. Implementation Order (Six Bounded Landings)

Each landing is its own PR-sized change, validates with `make lint && make
typecheck && make test`, and ends in a green test suite. The order
matches RFC §11 with a CLI-before-API split chosen for ergonomics
review: get the CLI right first, then layer the HTTP surface that mirrors
it.

1. **Generator core + catalog.** Package, spec types, envelope,
   shape/lane-set compilers, custom plan compiler, catalog package data,
   unit tests. No CLI or HTTP yet. (~3 days.)
2. **CLI `workflow templates list/show`.** Catalog read verbs, JSON +
   human output, help text, tests. (~1 day.)
3. **CLI `workflow generate`.** Dry-run path, write path with
   refuse-to-overwrite, error envelope with `field_path`, tests. (~2
   days.)
4. **`workflow init` rewire.** Replace the body of `workflow_init` with
   a shim over `generate_workflow`; preserve the existing return dict
   shape; add the back-compat snapshot test. (~1 day.)
5. **Local API endpoints.** Add the four routes to `service.py`, reuse
   the existing mutation guard pattern, JSON envelope tests covering
   preview, write, errors, and the `confirm_write` / `--allow-mutations`
   gates. (~2 days.)
6. **Documentation.** `WORKFLOW_TYPES.md` "Generator workflow" section,
   `CLI_REFERENCE.md` block, `WRITING_WORKFLOWS.md` note at the top
   pointing first-time authors at `workflow templates list`,
   `SPEC.md` value-object entries, `UBIQUITOUS_LANGUAGE.md` terms,
   `DECISION_LOG.md` entry. (~1 day.)

Total: ~10 working days; each step independently shippable.

## 15. Acceptance Mapping (RFC 0034)

Every acceptance criterion in RFC §"Acceptance Criteria" maps to this
design:

| Criterion | Where addressed |
|---|---|
| `generate_workflow` exists as library API with unit tests | §3.1, §3.5 |
| Every built-in shape × every compatible base lane set validates | §3.5 (shape matrix test) |
| Incompatible combos return field-specific errors | §3.2 (stage-3 errors), §10 (test) |
| `shape: custom` compiles from closed vocabulary only | §3.3, §9 |
| `workflow templates list/show --json` exposes catalog metadata | §4.1, §7 |
| Local service read endpoints expose the same metadata | §5.1 |
| `workflow generate --dry-run --json` returns envelope; writes nothing | §4.1, §4.2 |
| `workflow generate <path>` writes and validates | §4.1, §4.2 |
| `POST /workflows/generate/preview` returns the same envelope | §5.1, §5.3 |
| `POST /workflows/generate` mutation-gated, `confirm_write: true`, structured errors | §5.1, §5.4 |
| `workflow init --style` remains backwards-compatible | §6 |
| Web `/workflows/new` chooser | **Deferred — §13.1** |
| Docs updates across WORKFLOW_TYPES / WRITING_WORKFLOWS / CLI_REFERENCE / SPEC / UBIQUITOUS_LANGUAGE | §14 step 6 |

## 16. Open Questions Carried Forward

These are RFC §"Open Questions" entries this design does **not**
resolve; the synthesis can decide them when the second dogfood opens.

- Durable `workflow.generator.json` source plan alongside generated
  `workflow.json` — V1 says no (§13.6), but the cost to add it later is
  low.
- Whether lane commands should be required for `single_agent` /
  `author_reviewer` or allowed with placeholder commands plus loud
  warnings. V1 uses placeholder commands with **stage-3 warnings**
  (`warnings[].field_path: spec.lanes.<id>.command`) so the workflow
  validates but the operator sees the next step. Reviewer judgment may
  flip this.
- Inference of available lane profiles from installed skill or plugin
  bundles. V1 keeps lane commands explicit operator input.
- Target-repo local catalog extensions — deferred (§13.3).
- Web-chooser repository inspection for suggested shapes — deferred
  (§13.4).

The design is ready to implement in V1 without resolving these; each
becomes a small follow-up decision after the first round of CLI
dogfooding.
