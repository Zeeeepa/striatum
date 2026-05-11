# Synthesize Design Prompt

Produce `docs/dogfood/034/DESIGN_SYNTHESIS.md`. The file must start with a `striatum.synthesis.v1` front matter block (JSON-encoded values; quote strings; JSON arrays for lists). Example shape:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/034/design/codex/DESIGN.md", "docs/dogfood/034/design/claude_code/DESIGN.md", "docs/dogfood/034/design/gemini/DESIGN.md"]
---
```

The byline appears AFTER the front matter block, as a plain Markdown line `author: <slug>` (lowercase, no bold/italics/heading/quotes).

Read all three design artifacts and synthesize ONE paired implementation plan for RFC 0030 + RFC 0031. The synthesis must explicitly choose, not enumerate.

Required sections:

- **Accepted Implementation Scope** — map each RFC 0030 §Acceptance Criteria bullet AND each RFC 0031 §Acceptance Criteria bullet 1:1 to a concrete code-and-test plan, with one named owner per bullet (which `src/striatum/` module, which test file).
- **Deferred Scope** — cross-repo workflows + MCP mutation capability expansion (RFC 0032), Python → Go core port (D084), bundled / Dockerized PG distribution (RFC 0033 follow-up), retirement of direct repo-local CLI mode (separate future RFC), service-manager installer (separate future RFC). Each line says why deferred and where it lands.
- **RPC Envelope and Framing Decision** — concrete envelope shape (`schema_version`, `request_id`, `method`, `params`, `capability_token`, `deadline_ms`); JSON framing for V2.0; how the protocol survives a future Go core (D084).
- **Transport Decision** — Unix-domain socket default + loopback HTTP opt-in + MCP over socket sub-path; per-platform socket discovery paths.
- **Version Handshake Rules** — `daemon.hello` / `daemon.welcome` flow; refuse on envelope mismatch (exit code 10); no silent downgrade to direct mode on version-skew error.
- **Capability Vocabulary and Method Registry** — concrete `read`/`write`/`review`/`claim`/`apply`/`admin` vocabulary + the method → required-capability mapping. `apply` is new in V2 for sealed mode.
- **Audit + Request Log Mapping** — how each mutating RPC call appends to the audit chain on the RFC 0033 substrate; canonical params hash; denial reasons.
- **Supervisor Ownership Migration** — daemon DB `process_supervisors` table + repo-local `process_supervisor_pointers` table; `supervise.*` RPC methods; reattach across daemon restart via pid + pid_start_time.
- **Sealed-Apply Boundary** — `apply.reviewed_patch` RPC flow, refuse-on-mismatch exit codes, daemon-owned worktree, apply receipt format.
- **Signing Key Custody** — OS keyring per platform, `0600` runtime fallback, key rotation RPC, daemon refuses sealed-mode without a loadable key. AI-guardrail framing (RFC 0031 threat model) not cryptographic non-repudiation.
- **Workflow Schema Additions** — `require_daemon: true`, `apply_gate: true`, `sealed_patch_provider: refuse` (debugging aid).
- **Routing Plan** — which CLI verbs route through daemon by default in V2, which still allow `--no-daemon`, which are admin-only (no fallback).
- **MCP Surface Update** — daemon MCP tools/list filters by capability; capability tokens are the only access path; no global `--allow-mutations` flag.
- **Test Matrix** — adversarial cases for each refuse path (capability missing, version mismatch, base-tree drift, digest mismatch, verdict-against-wrong-digest, supervisor reattach with pid_start_time mismatch, token revocation race).
- **Documentation Deltas** — SPEC / MCP / UBIQUITOUS_LANGUAGE / CLI_REFERENCE / HOW_TO_HUMAN / RFC 0030 status / RFC 0031 status / CHANGELOG.
- **Staging Plan** — what lands in this dogfood vs deferred to a future dogfood. Avoid overclaim of RFC 0032 (cross-repo) or sealed apply behavior that depends on later work.
- **Human-Decision Questions** — any open questions the implementer cannot resolve from the synthesis alone; map back to RFC 0030 / RFC 0031 §Open Questions where applicable.

If the three designs disagree, pick one path and explain the tradeoff. If a guarantee is advisory, label it advisory. Per RFC 0031 threat model, sealed apply is an AI-guardrail, not cryptographic non-repudiation; documentation must reflect that exactly.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim into the artifact title block AFTER the front matter and a blank line.

- The byline must be a plain Markdown line with NO bold (`**`), NO italics, NO heading prefix (`#`), NO quotes around the value, NO trailing punctuation.
- The line must start with lowercase `author:` exactly.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.
