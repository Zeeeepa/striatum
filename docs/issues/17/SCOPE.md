---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/17/SPEC.md", "docs/ROADMAP.md", "docs/TODO.md", "docs/DECISION_LOG.md", "docs/SPEC.md", "docs/INDEX.md", "AGENTS.md", "docs/rfcs/0041-engram-memory-layer-for-striatum-operators.md", "docs/rfcs/0044-engram-phase-1-implementation-spec.md"]
---

author: triager-unknown-model-001

# GH #17 -- Scope

Bound scope for GH #17, "Update Striatum document consistency for Engram
memory integration." The implementation job is documentation-only plus an RFC
scaffold. It must make the Striatum-side story coherent: Engram consumes
Striatum corpus exports as optional local augmentation; Striatum does not
depend on Engram at runtime.

## Issues Covered

- GH #17 -- Update Striatum document consistency for Engram memory
  integration.

Related but not closed by this workflow:

- GH #15 -- Clarify PostgreSQL transition guidance. This workflow may add
  narrow wording where memory/export documentation touches state substrate or
  provenance, but the full README/SPEC/GETTING_STARTED/HOW_TO_HUMAN
  PostgreSQL transition sweep remains GH #15.
- Engram-side RFC 0044/0045 follow-up in `~/git/engram/`. This workflow must
  not add Engram ingester, MCP server, retrieval tools, or `memory.*`
  capabilities to Striatum.

## Files and Directories in Scope

The implementer should update or create these files only as needed:

- `docs/rfcs/0057-corpus-contract-v2.md` -- create a proposed RFC scaffold
  for Striatum Corpus Contract V2. It must define the contract questions named
  in `docs/ROADMAP.md` section 5.7: manifest shape, source kinds, metadata,
  stable item IDs, content hashes, instance/repository identity,
  privacy/redaction metadata, incremental-export watermarks, validation rules,
  backward compatibility, multi-corpus naming, and augmentation-boundary
  enforcement.
- `docs/rfcs/README.md` -- add RFC 0057 to the index as proposed.
- `docs/SPEC.md` -- add or update the Striatum-side corpus export and
  augmentation boundary wording. The SPEC should say Engram is an optional
  local consumer of exported Striatum corpora, not a live state store,
  message bus, required daemon, hosted service, or Striatum runtime
  dependency.
- `README.md` -- if needed, add a short operator-facing pointer to corpus
  export / optional Engram ingestion without implying cloud persistence or
  required external service availability.
- `docs/GETTING_STARTED.md` and `docs/HOW_TO_HUMAN.md` -- if needed, add
  operator initialization guidance for exporting a Striatum corpus for
  Engram ingestion. Keep this as an optional post-run or maintenance step.
- `docs/HOW_TO_AGENT.md` -- if needed, clarify that workflow agents should
  rely on packet context and canonical docs; any Engram-backed retrieval is
  optional augmentation supplied by the operator or a future explicit
  policy, not a requirement for ack/publish/complete.
- `docs/CLI_REFERENCE.md` -- if needed, ensure `striatum corpus export` is
  discoverable and framed as a read/export surface.
- `docs/MCP.md` -- if needed, clarify that current Striatum MCP/chat tools do
  not expose Engram `memory.*` capabilities; any Engram MCP server is
  separate and optional.
- `docs/ROADMAP.md` -- small consistency edits are allowed, especially to
  keep section 5.7 and the GH #17 row aligned after RFC 0057 is scaffolded.
- `docs/TODO.md` -- small status/cross-reference edits are allowed, but GH
  #17 should remain open unless the full issue is demonstrably closed by this
  workflow.
- `docs/INDEX.md` -- update only if new or changed docs need index coverage.
- `docs/issues/17/build/HANDOFF.md` -- required implementer handoff artifact.
- `docs/issues/17/review/REVIEW.md` -- required verifier artifact.

## Files and Directories Out of Scope

The implementer must not touch:

- `src/`, `tests/`, `go/`, or package metadata. No product code or test
  change is required by this triage scope.
- `.striatum/` or any runner state substrate by hand.
- `docs/dogfood/` historical artifacts.
- `docs/issues/15/` and GH #15-specific artifacts, except by reading them for
  awareness if present.
- `docs/DECISION_LOG.md`, unless the implementer makes a real product or
  architecture decision beyond a proposed RFC scaffold. A proposed RFC 0057
  entry alone does not require a D-row.
- `docs/ENGRAM_INCUBATION_CONTEXT.md`,
  `examples/rfc-0014-operational-artifact-home/`, and older P00x prompts,
  unless only adding an explicit historical/deferred label. They remain
  historical reference fixtures.
- Any Engram repository path, code, schema, ingester, MCP server, retrieval
  implementation, or `memory.*` capability.
- Cloud persistence, hosted service, telemetry, transcript capture, automatic
  Engram push, or always-running Engram daemon requirements.

## Acceptance Checklist

The verify job should cite each numbered check with file:line evidence.

1. GH17-1: Operator-facing docs present one coherent path: Striatum can export
   a redacted corpus bundle, and Engram may ingest it as an optional local
   consumer.
2. GH17-2: Docs describe what Striatum exports for Engram ingestion at the
   current V1 level: the existing `striatum corpus export --since <ref> --out
   <dir>` surface, JSONL bundle, `manifest.json`, redaction posture, and
   durable provenance/source categories.
3. GH17-3: A proposed RFC 0057 scaffold exists for Corpus Contract V2 and
   names the V2 decisions from `docs/ROADMAP.md` section 5.7, including
   multi-corpus identity and incremental-export watermarks.
4. GH17-4: Docs clearly say Striatum must run when Engram is unavailable.
   No state transition, workflow command, recovery path, run lifecycle, or
   corpus export documentation may require an Engram import, Engram daemon, or
   Engram MCP server.
5. GH17-5: Docs avoid implying cloud persistence, telemetry, hosted retrieval,
   transcript capture, or required external services for memory integration.
6. GH17-6: Docs preserve the Striatum/Engram boundary: Striatum owns export
   format and redaction discipline; Engram owns ingestion, indexing,
   retrieval, `memory.*` capabilities, and any personal-memory isolation.
7. GH17-7: PostgreSQL/daemon wording introduced by this workflow is narrow and
   non-conflicting. It may refer to current runner state/provenance only where
   memory/export docs require it, and it must not claim to close GH #15's full
   transition-guidance sweep.
8. GH17-8: Stale or conflicting Engram guidance in current product docs is
   either updated, explicitly marked historical/deferred, or left untouched
   only when it lives in historical fixture material.
9. GH17-9: The implementation handoff lists every changed file, every
   verification command run, verification not run, and residual risk.

## Risks and Parallel-Workflow Conflicts

- GH #15 overlap: many docs still contain older SQLite/PostgreSQL transition
  wording. This workflow should not broaden into a full state-substrate doc
  rewrite. Keep any state-substrate language local to corpus export,
  provenance, and the augmentation boundary.
- RFC numbering conflict: if another parallel workflow creates RFC 0057 first,
  the implementer must use the next free RFC number and update this scope in
  the handoff.
- Broad search output includes historical dogfood and incubation files with
  Engram-specific language. Do not rewrite historical provenance merely to
  remove the word "Engram"; current product docs and new RFC scaffolding are
  the target.
- Runtime-boundary regression risk: new prose must not suggest that workflow
  agents call Engram during `ack`, `publish-artifact`, `verdict`, `complete`,
  recovery, or run start. Any retrieval/injection policy belongs in RFC 0057
  as a future explicit opt-in decision.
- External dependency risk: `docs/ROADMAP.md` cites an Engram-side roadmap at
  `~/git/engram/STRIATUM_MEMORY_ROADMAP.md`. Do not hard-require that local
  path in reusable Striatum docs; use it only as roadmap context.

## Verification Commands

The implementer should run at least:

```bash
rg -n "Engram|engram|memory\\.\\*|corpus export|ingest-striatum|runtime dependency|optional" README.md docs/*.md docs/rfcs/*.md prompts/*.md
python3 -m pytest tests/test_doc_links.py tests/test_artifact_schemas.py
```

If the implementation only changes Markdown and the local environment lacks
test dependencies, the handoff must say so and include manual link/path
checks instead. Do not run broad source tests unless implementation expands
into code, which this scope does not require.
