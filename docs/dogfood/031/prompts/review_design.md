# Review Design Prompt

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter.

Review `docs/dogfood/031/DESIGN_SYNTHESIS.md` under your assigned posture. Use only an accepting verdict if the plan is implementation-ready for that posture.

Attack at least these risks:

- scope creep beyond the RFC 0028 §Acceptance Criteria bullets and the phased migration steps 1–6;
- daemon authority overclaim: capability defaults, MCP mutation gating, apply authority, signing implications;
- migration risk for existing `.striatum/state.sqlite3` runs and direct CLI mode fallback during the phased migration;
- tenancy ambiguity: repository tenant vs operator tenant vs client tenant boundaries on a single-user laptop and on a shared workstation;
- registry storage choice: split-brain risk between central registry and per-repo state, fresh-clone audit story, repository deletion semantics;
- cross-platform containment and packaging gaps, especially macOS launchd, Linux systemd user units, and Windows service shape;
- supervised process re-attach correctness after daemon crash or upgrade, including PTY vs pipe and lane attestation continuity;
- audit log integrity: tamper resistance, token redaction, retention, and the boundary against transcript capture;
- MCP threat surface: prompt-injected client requesting mutation, capability token theft, version skew with mutation tools;
- false provenance claims: any text in the synthesis that implies RFC 0026 attestation or RFC 0027 sealed guarantees the V1 daemon does not actually provide.

For `needs_revision`, list the minimum concrete changes needed before implementation may proceed. For `accept_with_findings`, make sure the findings are non-blocking and explicitly say so.
