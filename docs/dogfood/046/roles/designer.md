# Designer Role (Dogfood 046)

Three fresh-design lanes (codex, claude, gemini) produce independent
perspectives on RFC 0044 V1 Striatum-side corpus export. Synthesis
picks one path. Cite the existing code that your design changes — do
not propose green-field shapes.

Required citations (read these before designing):

- `docs/rfcs/0044-engram-phase-1-implementation-spec.md` — §3
  (Striatum Export Bundle) and §8 (Augmentation-Not-Dependency).
- `docs/rfcs/0041-engram-memory-layer-for-striatum-operators.md` —
  operator motivation.
- `src/striatum/cli/parser.py` and `src/striatum/cli/dispatch.py` —
  the verb registration + dispatch surface; mirror neighboring verbs
  (`run summary --json`, `recovery stale-leases`).
- `src/striatum/cli/run_summary.py` — the canonical run-summary reader
  RFC 0044 §3 mandates for run summaries.
- `~/git/engram/migrations/` and `~/git/engram/src/engram/` — cite
  Engram's `source_kind` enum precedent and the expected ingest row
  shape. **Do NOT modify Engram**; this is read-only awareness.

Address: CLI verb wiring, `src/striatum/corpus/` module layout,
per-`sub_kind` enumeration sources, redaction policy with explicit
denylist, JSONL emission shape locked to RFC 0044 §3, manifest fields,
augmentation-not-dependency boundary, and a tests plan ending in one
integration test against a real recent dogfood with replay-stable
hashes.

**Augmentation-not-dependency is non-negotiable** — no Engram client
import anywhere under `src/striatum/`. No `memory.*` capability on
the daemon RPC registry. Striatum must run identically with Engram
absent. Name the regression guard.

Out of scope: Engram ingester, `engram-mcp-stdio`, `engram.search`,
`engram.fetch_reference`, `striatum operator memory check`, the
`striatum-engram` skill, doc updates beyond build/HANDOFF.
