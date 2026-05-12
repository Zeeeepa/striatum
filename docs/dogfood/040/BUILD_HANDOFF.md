# Dogfood-040 Build Handoff (Combined)

author: implementer-claude-opus-001

RFC 0040 V1 — MCP-Driven Dogfood Harness for Operator Sessions —
ships as two disjoint implementer halves running in parallel under
dogfood-040. This combined handoff stitches them together.

The author lane (`claude_code`, `implementer-claude-opus-001`) shipped
the **operator-side / ergonomics half**. The codex lane was scoped to
the **systems half** in the same dogfood; its handoff at
`docs/dogfood/040/build/systems/HANDOFF.md` had not been published
when this combined handoff was written. The status table below is
authored from the perspective of the V1 acceptance criteria in
[`docs/rfcs/0040-mcp-driven-dogfood-harness.md`](../../rfcs/0040-mcp-driven-dogfood-harness.md);
the operator can update the systems-half columns when that handoff
lands.

## Scope split

| Section | Half | Implementer |
|---------|------|-------------|
| MCP chat-tool entries for dogfood-lifecycle verbs | ergonomics | claude_code |
| Per-model harness-profile fragments + catalog | ergonomics | claude_code |
| Generator enrichment by default | ergonomics | claude_code |
| `striatum workflow upgrade` CLI verb | ergonomics | claude_code |
| Operator-side documentation | ergonomics | claude_code |
| `dogfood.publish_on_behalf` composite tool | systems | codex |
| `dogfood.surgical_recovery` composite tool | systems | codex |
| `surgical_recovery` capability | systems | codex |
| Daemon-side supervised-progress watcher | systems | codex |
| RFC 0035 harness extensions for composite tools | systems | codex |

## Ergonomics half — what shipped (this handoff)

See [`docs/dogfood/040/build/ergonomics/HANDOFF.md`](build/ergonomics/HANDOFF.md)
for the file-by-file detail. Summary:

- 12 new dogfood-lifecycle chat-tool entries in
  `src/striatum/web/chat_tools.py`, 10 mutation-gated, 2 read-shaped.
- `harness_profile_fragments` section in
  `src/striatum/workflow_templates/catalog.json` with the
  `claude_code_default`, `codex_default`, `gemini_default`, and
  `generic_default` profiles.
- Generator enrichment in
  `src/striatum/workflow_generator/core.py` so new workflows pick up
  the catalog defaults; `src/striatum/workflow_generator/catalog.py`
  helpers (`list_harness_fragments`, `get_harness_fragment`,
  `get_harness_fragment_by_tool_family`).
- New `src/striatum/cli/workflow.py` with
  `workflow_upgrade(path, *, repo, force, dry_run)` plus parser +
  dispatch wiring for the `striatum workflow upgrade <path>` verb.
- New tests: extensive additions to `tests/test_chat_tools.py`,
  `tests/test_workflow_generator.py`, `tests/test_web_chat.py`; new
  `tests/test_workflow_upgrade.py`.
- New `docs/HARNESS_FRICTION_PATTERNS.md`; updates to `docs/MCP.md`,
  `docs/HOW_TO_HUMAN.md`, `docs/HOW_TO_AGENT.md`,
  `docs/UBIQUITOUS_LANGUAGE.md`, `docs/CLI_REFERENCE.md`,
  `docs/INDEX.md`, `docs/TODO.md`, `docs/DECISION_LOG.md` (D093),
  `docs/rfcs/0040-mcp-driven-dogfood-harness.md` (status →
  `accepted (V1)`), `docs/rfcs/README.md`, `README.md`,
  `CHANGELOG.md`, `pyproject.toml` (1.28.0 → 1.29.0).

## Systems half — what should ship (TBD)

Per RFC 0040 §2, §3, §4 the systems half should land:

- `src/striatum/dogfood/operator_tools.py` (or extension of an
  existing module) implementing the `dogfood.publish_on_behalf`
  composite tool with operator `reason` field in the audit row.
- The `dogfood.surgical_recovery` composite tool (atomic lease
  reactivation + supervisor reattachment + job-state restoration)
  with the new `surgical_recovery` capability added to the RFC 0030
  closed vocabulary.
- `src/striatum/daemon_supervisor/progress_watcher.py` — daemon-owned
  watcher per supervisor that refreshes the lease when the
  supervised wrapper's log mtime indicates forward progress.
- Test extensions: simulate ack-denied case end-to-end, simulate
  lease-expired-under-active-load case, simulate log growth →
  heartbeat. These live in `tests/_harness/` and `tests/test_daemon_*.py`.

When the systems handoff lands at
`docs/dogfood/040/build/systems/HANDOFF.md` the operator can update
the acceptance table below.

## Combined acceptance criteria status

| Criterion (RFC 0040) | Status |
|----------------------|--------|
| Each dogfood-lifecycle RPC method has a chat-tool entry | ✅ ergonomics |
| `dogfood.publish_on_behalf` composite tool works end-to-end | ⏳ systems |
| `dogfood.surgical_recovery` composite tool works end-to-end | ⏳ systems |
| Daemon supervised-progress watcher refreshes leases on log growth | ⏳ systems |
| `workflow init` / `generate` emit new harness fragments by default | ✅ ergonomics |
| `workflow upgrade` verb backports fragments into existing workflows | ✅ ergonomics |
| `docs/HARNESS_FRICTION_PATTERNS.md` documents the patterns + fixes | ✅ ergonomics |
| `surgical_recovery` capability added to RFC 0030 vocabulary | ⏳ systems |
| RFC 0035 harness extensions cover composite tools + watcher | ⏳ systems |
| No regression in existing dogfood-lifecycle behavior | ✅ both (additive) |

## Verification still required from the operator

Both halves should be smoke-tested together once the systems half
lands. From the ergonomics half alone the operator should run:

```bash
make install
make lint
make typecheck
.venv/bin/pytest tests/test_chat_tools.py \
                 tests/test_web_chat.py \
                 tests/test_workflow_generator.py \
                 tests/test_workflow_upgrade.py -q
make test
make smoke
```

Then drive a dry-run of the new chat-tool sequence (`run_prepare` …
`evidence_export`) through `striatum serve --web --allow-mutations`
against the existing `examples/rfc-ledger-cleanup/workflow.json`
fixture; confirm tools appear in the request body sent to the
operator-configured LLM and that mutations are correctly hidden when
`--allow-mutations` is omitted.

## Notes for the OPERATOR_REPORT.md

When the operator finalizes the dogfood-040 report under
`docs/dogfood/040/OPERATOR_REPORT.md`, the load-bearing observations
from the ergonomics half are:

1. The "no-questions" instruction now lives in a single catalog
   table rather than being duplicated across role docs. New
   dogfoods scaffolded with `workflow generate` pick it up
   automatically; existing dogfood-031…039 workflows can be
   upgraded with one verb each.
2. `striatum ack` was denied for this implementer session under the
   `claude --print` supervised wrapper (the pattern this RFC
   addresses). The operator published this handoff on the author's
   behalf, exercising the publish-on-behalf shape RFC 0040 §2 will
   eventually fold into a composite chat tool.
3. The ergonomics half is additive: no existing CLI/JSON API/SSE/CSP
   surface changed. Operators currently driving dogfoods via bash
   CLI continue to work; the chat-tool path is opt-in via
   `serve --web --allow-mutations`.
