---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0040", "mcp-harness", "build"]
---

author: reviewer-claude-opus-002

# RFC 0040 V1 Build Review — Ergonomics Half (Threat Model)

## Scope and Method

Threat-model review of the RFC 0040 V1 ergonomics-side surfaces per RFC
0031 §Threat Model: over-eager AI agents through documented interfaces +
operator-mistake footguns; malicious local-root is out of scope.

Files inspected:

- `src/striatum/web/chat_tools.py`
- `src/striatum/cli/workflow.py`
- `src/striatum/cli/parser.py` (workflow upgrade subparser)
- `src/striatum/cli/dispatch.py` (workflow upgrade dispatch)
- `src/striatum/workflow_generator/catalog.py`
- `src/striatum/workflow_generator/core.py` (`_enrich_harness_profile_body`)
- `src/striatum/workflow_templates/catalog.json`
- `src/striatum/daemon_rpc/registry.py`
- `docs/MCP.md`, `docs/HOW_TO_HUMAN.md`, `docs/HOW_TO_AGENT.md`
- `docs/HARNESS_FRICTION_PATTERNS.md`
- `tests/test_chat_tools.py`, `tests/test_workflow_upgrade.py`
- `docs/dogfood/040/DESIGN_SYNTHESIS.md` (as the source of truth for the
  fragment text and the V1 surface contract)

Tests were not re-executed; review is read-only.

## Trust Boundaries Enumerated

1. **Chat-tool dispatch boundary** (`execute_tool` in `chat_tools.py`):
   model-supplied tool name + args are validated against a closed set;
   unknown tools refuse with an error string and never reach the dispatcher.
2. **Mutation gate boundary** (`tool_names()` / `tool_schemas()` + the
   re-check inside `_tool_dogfood_lifecycle`): without
   `--allow-mutations` the ten state-mutating dogfood tools are hidden
   from `tools/list` *and* refused at execute time.
3. **Argument validation boundary** (`_require_str`, `_optional_str`,
   verdict-enum check, lease-seconds range check): a structured error
   envelope is returned before `striatum.api.invoke` is called.
4. **CLI invocation boundary** (`striatum.api.invoke` in the chat tool's
   thin shell): write-scope, capability, and front-matter validation
   happen in the canonical CLI verbs the chat tool wraps. The chat surface
   does not duplicate those checks.
5. **Workflow upgrade write boundary** (`workflow_upgrade` in
   `cli/workflow.py`): refuse-on-conflict + refuse-on-running-workflow +
   dry-run guard the rewrite. `--force` is the explicit operator escape.
6. **Catalog-fragment enrichment boundary**
   (`_enrich_harness_profile_body` in generator core): catalog text is
   applied only when the caller's `native_delegation.instruction` is
   absent/empty; explicit overrides are preserved.
7. **Capability registry** (`daemon_rpc/registry.py`): adds
   `surgical_recovery` as a closed-set capability; admin-bound by the
   composite tool route (systems half).

Each boundary is acknowledged below with mitigations or findings.

## Verdict

`accept_with_findings`. The ergonomics-half surfaces (chat-tool registry,
generator catalog, `workflow upgrade`, docs) are coherent and the
threat-model surfaces are guarded by sensible defaults. The findings are
non-blocking gaps against the design synthesis plus a small set of
documentation-honesty corrections.

## Findings

### F1. Gemini fragment is missing the strategy-then-exit guard (medium)

**Location.** `src/striatum/workflow_templates/catalog.json` →
`harness_profile_fragments[2].native_delegation_instruction` (the
`gemini_default` entry).

**Evidence.** The shipped gemini fragment reuses the codex-style intro:

> "Use native delegation aggressively for parallelizable work. … If a
> step is ambiguous, choose the most-conservative default …"

The DESIGN_SYNTHESIS (lines 124–128) specified instead:

> "Use local sub-agents only for bounded independent work. This is a
> one-shot supervised invocation: **write the artifact in this single
> call; do not surface strategy and exit without producing the file.** …"

The synthesis text was the gemini-specific anti-pattern guard for the
dogfood-036 strategy-then-exit failure mode (Pattern 1 in
`docs/HARNESS_FRICTION_PATTERNS.md`). The current fragment retains the
front-matter completeness callout (Pattern 4) but drops the
"write the artifact in this single call; do not surface strategy and
exit" clause that the synthesis put specifically in front of gemini.

**Threat-model impact.** Operator-side footgun, not adversary-side: a
fresh dogfood run that generates a gemini lane via `workflow generate`
inherits a fragment that does *not* contain the explicit guard against
the failure mode the RFC was scoped to eliminate. The omission is silent;
the operator only discovers it when gemini repeats the same
"strategy artifact then exit" pattern that the RFC was supposed to close.

**Recommendation.** Restore the synthesis-prescribed intro for
`gemini_default.native_delegation_instruction`: replace the leading
"Use native delegation aggressively …" sentence with
"Use local sub-agents only for bounded independent work. This is a
one-shot supervised invocation: write the artifact in this single call;
do not surface strategy and exit without producing the file." Keep the
front-matter completeness callout that follows. No additional verb
or schema change is required.

### F2. Codex fragment is missing the focused-pytest guidance (medium)

**Location.** `src/striatum/workflow_templates/catalog.json` →
`harness_profile_fragments[1].native_delegation_instruction` (the
`codex_default` entry).

**Evidence.** The DESIGN_SYNTHESIS (lines 112–116) closes the codex
fragment with:

> "For long-running test runs, prefer focused pytest invocations before
> the wider `make test` to avoid lease expiry beyond ~30 minutes."

That sentence is absent from the shipped fragment. The friction-patterns
doc (Pattern 3) frames lease-expiry-under-active-load as a codex-specific
shape (codex mid-`make test`) and identifies the fix as two-part:
operator-side composite tool + daemon-side progress watcher. The
designer added a *third* mitigation aimed at the supervised model
itself — prefer focused pytest first — which never made it into the
shipped fragment.

**Threat-model impact.** Operator-mistake footgun: until the systems-half
progress watcher (RFC 0040 §4) lands and ships, the codex fragment is the
only place where the model is told to avoid the exact shape that
triggers the lease-expiry race. Pattern 3 of `HARNESS_FRICTION_PATTERNS.md`
describes the fix as "fixes that landed in v1.29.0" — but the
model-side mitigation the designer wrote did not land.

**Recommendation.** Append the synthesis-prescribed sentence to
`codex_default.native_delegation_instruction`: "For long-running test
runs, prefer focused pytest invocations before the wider `make test` to
avoid lease expiry beyond ~30 minutes." Treat this as a sibling fix to
the daemon-side progress watcher.

### F3. Documentation overstates per-capability gating for chat tools (medium)

**Location.** `docs/MCP.md` lines 134–155 (the dogfood-lifecycle tools
table) and `docs/dogfood/040/DESIGN_SYNTHESIS.md` lines 41–58 (the
"MCP Chat-Tool Surface" table).

**Evidence.** Both tables include a "Required capability" column listing
`write` / `claim` / `review` / `read` per tool. In the current V1
implementation, `chat_tools.execute_tool` consults only the single
`allow_mutations` boolean (the `--allow-mutations` service flag); there
is no per-tool capability token check on the local chat surface. The
table's footnote (`MCP.md` line 150) does disclose this:

> "the local web chat surface is owner-only and reuses the mutation gate
> instead of token capabilities …"

but a reader scanning only the table will conclude that calling
`verdict` from a `write`-only chat session is refused, which is not what
the code does. The capabilities listed are the **daemon RPC** required
capabilities the tool *would* require when invoked through the daemon
MCP surface; the local web chat surface bypasses them entirely.

**Threat-model impact.** Operator-mistake footgun: an operator who reads
the table and assumes per-capability gating may grant `--allow-mutations`
believing they are scoping access. They are not — they are granting all
ten mutating tools at once, including `verdict` and `complete`. The
footnote is correct but easy to miss.

**Recommendation.** Tighten the MCP.md table heading from
"Required capability" to "Daemon-RPC required capability (local chat
gates on `--allow-mutations` only)", and lift the footnote into a
single-line preamble above the table. Same change in the design
synthesis table is a nice-to-have but not strictly required since the
synthesis is historical context.

### F4. Composite tools advertised in docs are not yet on the chat surface (low)

**Location.** Boundary between
`src/striatum/daemon_rpc/registry.py` (which registers
`dogfood.publish_on_behalf` and `dogfood.surgical_recovery`),
`src/striatum/dogfood/operator_tools.py` (systems-half implementation),
and `src/striatum/web/chat_tools.py` (ergonomics-half surface).

**Evidence.** `chat_tools.py` exposes only the twelve lifecycle
primitives (the `DOGFOOD_LIFECYCLE_TOOL_NAMES` frozenset). The two
composite tools that are the whole point of the publish-on-behalf and
surgical-recovery friction reductions (RFC 0040 §2 / §3) are reachable
only through the daemon MCP route (`tools/call` against
`dogfood.publish_on_behalf` / `dogfood.surgical_recovery`), not through
the `striatum serve --web --allow-mutations` chat tool registry.

The DESIGN_SYNTHESIS does say the chat surface should ship "per-RPC
lifecycle tools plus the two composite tools" (line 39). The
HOW_TO_HUMAN walkthrough (lines 157–183) and MCP.md (lines 156–164)
both correctly describe the V1 state as "thin lifecycle tools today,
composite tools land in the daemon-side systems half" — so the doc is
honest. But the friction-reduction promise of the RFC ("single composite
tool call instead of hand-chaining ack + publish + verdict + complete")
is not yet realized from the **chat** surface in V1. The HOW_TO_HUMAN
example sequence (lines 167–178) shows the operator chaining the four
primitives by hand, which is exactly the friction the composite was
introduced to remove.

**Threat-model impact.** Not a security issue. This is a
documentation-vs-implementation honesty gap for the friction reduction:
the local chat-tool path stays at the primitive-chaining shape. The
operator's MCP session can still reach the composites via the daemon
MCP transport.

**Recommendation.** Either (a) add `dogfood.publish_on_behalf` and
`dogfood.surgical_recovery` entries to `chat_tools.py` that route
through `striatum.api.invoke` against the composite CLI verbs (if the
systems half exposed them) or against the daemon RPC method directly,
or (b) extend the HOW_TO_HUMAN "Drive a dogfood through the MCP chat
surface" section to explicitly note that composite calls go through the
daemon MCP route, with a one-line redirect to the daemon MCP example.
Option (b) is the lighter fix and matches the
ergonomics-vs-systems split in this dogfood.

### F5. `workflow upgrade` running-workflow guard silently degrades on a locked or missing daemon DB (low)

**Location.** `src/striatum/cli/workflow.py` lines 199–239
(`_running_runs_for_workflow`).

**Evidence.** The guard catches `sqlite3.Error` twice and returns `[]`
in both cases (line 217–218 and line 236–237). A locked DB (concurrent
`striatum run prepare` mid-transaction), a missing DB, or a corrupted
DB all produce the same "no running runs found" result, which then
lets the upgrade proceed and rewrite the workflow.json. Per the
design synthesis (line 134), "It refuses on a workflow with an
active/running prepared snapshot unless `--force` is supplied" — the
intent is fail-closed; the implementation is fail-open on DB error.

The path-shape match between `workflow_snapshots.source_path` and the
target uses string equality against either the absolute resolved path
or the repo-relative path (lines 204–214). A run prepared with a
symlinked workflow path, or a workflow file later renamed/moved, will
not match. This is an extra footgun overlapping the DB-error fail-open.

**Threat-model impact.** Operator-mistake footgun: an operator running
`workflow upgrade` while another striatum process holds a write lock
on `state.sqlite3` will not get the running-workflow refusal they
expect; the upgrade succeeds. Pattern 3 of the friction-patterns doc
specifically calls out concurrent state mutation under load.

**Recommendation.** Either propagate the `sqlite3.Error` as a
`WorkflowError` so the caller sees `refused_db_error` (or similar) and
can decide whether to retry, or wait-with-busy-timeout for the lock to
clear before reporting empty. A two-line addition; no schema change.
Separately, document that the running-workflow guard matches on
`workflow_snapshots.source_path` and recommend running `workflow
upgrade` from the same path shape used at `run prepare` time.

### F6. `workflow upgrade` rewrites the whole JSON, not just the target field (low)

**Location.** `src/striatum/cli/workflow.py` lines 185–190.

**Evidence.** The verb produces the rewritten workflow with
`json.dumps(updated_workflow, indent=2, sort_keys=False) + "\n"`. This
rewrites the entire file with the generator's serialization shape:

- Trailing whitespace is normalized.
- Field ordering follows the loaded Python dict insertion order, not
  whatever ordering the operator chose when hand-authoring the file.
- Any non-standard formatting (extra spaces, JSON5-style trailing
  commas if a tolerant loader was used) is lost.

The design synthesis (line 134) explicitly bounded the verb: "It must
not rewrite jobs, lanes, edges, cycles, write scopes, roles, artifact
paths, or unrelated formatting beyond deterministic JSON output." The
"deterministic JSON output" caveat covers this, so it is in-spec — but
the diff a reviewer sees in the post-upgrade git diff can be much
larger than just the harness-profile delta, which makes the audit
trail noisy.

**Threat-model impact.** Operator-review footgun: a reviewer of the
upgrade commit may miss a malicious or unintended change buried in a
large reformatting diff. Out-of-scope per the threat model (no
malicious local root), but worth recording.

**Recommendation.** Optional. Either (a) document the rewrite shape in
the `workflow upgrade` `--help` / dispatcher description so reviewers
know to expect reformatting, or (b) load + emit with a stable
"surgical" JSON writer that keeps original ordering and only mutates
the `native_delegation` keys. (b) is over-engineered for V1; (a) is
the right scope.

### F7. `workflow upgrade` silently fills `native_delegation.mode` when absent (informational)

**Location.** `src/striatum/cli/workflow.py` lines 134–146.

**Evidence.** When `native_delegation.mode` is absent on a profile, the
verb adds it from the catalog default (e.g., `"preferred"` for claude,
`"encouraged"` for codex/gemini) without prompting and without
treating it as a conflict. The change row records `old_value: null` and
`new_value: <catalog_mode>`. There is no `forced: true` flag because no
conflict exists.

This is in line with the synthesis intent ("modifies only
`harness_profiles.*.native_delegation.instruction`" — note the
omission of `.mode`). Strict reading of the synthesis would say the
upgrade should *not* fill `.mode`; pragmatic reading says the catalog
mode is required for the profile to be useful and the operator likely
just omitted it.

**Threat-model impact.** None. Logged as informational because the
verb's scope is broader than what the synthesis prescribes.

**Recommendation.** Either narrow the verb to `native_delegation.instruction`
only and document the `.mode` omission as deliberate, or amend the
synthesis text in passing to acknowledge the `.mode` fill-in. Pick one
for documentation consistency.

### F8. Adversarial-case test coverage gaps (low)

**Location.** `tests/test_chat_tools.py`, `tests/test_workflow_upgrade.py`.

**Evidence.** Coverage gaps relative to the synthesis-listed adversarial
checklist (lines 24–35 of `review_build.md`):

- No test for `workflow upgrade` against a workflow with multiple
  profile tool families (codex + claude + gemini in one file). The
  happy-path test uses claude_code only.
- No test for `workflow upgrade` when the daemon SQLite is locked or
  missing (relates to F5).
- No test for `workflow upgrade` re-applied after `--force` (idempotency
  of the catalog-matching path).
- No test for the chat-tool argument-injection shape: the dogfood
  lifecycle tools accept arbitrary strings for `path`, `summary`,
  `rationale`, `reason`, etc. The publisher / CLI side is the
  authoritative validator, but a smoke test asserting the chat surface
  doesn't strip or transform the values before passing them through
  would catch silent normalization regressions.
- The `tool_names_match_schemas` test confirms `ANTHROPIC_TOOLS`,
  `OPENAI_TOOLS`, and `TOOL_NAMES` align on names but does not assert
  that the per-tool `description` text discloses the underlying CLI
  verb (the doc says each tool is "a thin shell over X" — a regression
  that silently rewords the description could break that contract).

**Threat-model impact.** Coverage gap only; no demonstrated regression.

**Recommendation.** Add three small unit tests: a multi-family
`workflow upgrade` happy path, a `workflow upgrade` with a locked DB
(simulate via `monkeypatch.setattr` on `sqlite3.connect`), and an
argument-passthrough test asserting `_dogfood_argv` builds the expected
argv vector verbatim for `publish_artifact` with a tricky path
(containing spaces, dots, parens). Each is <20 lines.

## Non-Findings (Acknowledged but Acceptable)

- **Chat-tool argument size**: `complete --summary`, `verdict
  --rationale`, and `publish_artifact --path` accept arbitrary-length
  strings on the chat surface. The CLI verbs are the authoritative
  validators (publisher refuses path escapes with exit code 6; the
  audit chain's `audit_class = "metadata"` excludes prose). Defense at
  the boundary is correct; no chat-level cap is needed for V1.
- **Mutation-gate granularity**: the binary `--allow-mutations` flag
  exposes all ten mutating tools at once. Granular per-capability
  gating is the daemon MCP path's job; the local chat surface is
  documented as owner-only with a binary gate (`MCP.md` line 150).
  Acceptable for V1; revisit when the daemon MCP path is the primary
  operator surface.
- **Composite-tool absence from chat surface**: see F4. Acknowledged
  as an ergonomics-vs-systems split decision in this dogfood, not a
  threat-model gap.
- **Catalog fragment text as plain-text instruction**: a hostile
  workflow author could put a prompt-injection payload in
  `harness_profiles.*.native_delegation.instruction`. The threat model
  excludes malicious local-root and the workflow validator does not
  policy-check instruction text; acceptable per RFC 0031 scope.
- **`workflow upgrade` is a CLI-only verb**: not exposed as a chat
  tool. This is the right scope decision — an over-eager AI cannot
  rewrite harness profiles silently. Good.

## Summary

The ergonomics-half ships the V1 surfaces the RFC promised: twelve
lifecycle chat tools with a mutation gate; per-model catalog fragments
with generator enrichment; `striatum workflow upgrade` with
refuse-on-conflict + refuse-on-running-workflow + dry-run; and
documentation that names the friction patterns it closes.

Three issues are worth addressing before V1 closes its first dogfood
cycle: the gemini fragment is missing its synthesis-prescribed
strategy-then-exit guard (F1), the codex fragment is missing its
focused-pytest guidance (F2), and the MCP.md table presentation
overstates per-capability gating for the local chat surface (F3). The
remaining findings (F4–F8) are non-blocking ergonomics and coverage
cleanups.

No new attack surface is introduced by the ergonomics half; the threat
boundaries are correctly delegated to the CLI verb layer or to the
binary mutation gate, with the per-capability token boundary deferred
to the daemon MCP route.
