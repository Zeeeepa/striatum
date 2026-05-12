# Claude Code Design Prompt

Produce `docs/dogfood/035/design/claude_code/DESIGN.md`.

Design an implementation plan for RFC 0032: cross-repository workflows + MCP mutation capabilities. Sit on top of dogfood-034's daemon RPC + supervision + sealed-apply foundation; do not redesign that scaffold.

Focus on:

**Trust boundaries:** operator, daemon process, MCP client (potentially prompt-injected), supervised lane process, and cross-repo coordinator. Per RFC 0031 §Threat Model the scope is over-eager AI + operator-mistake footguns; malicious-local-root is out of scope. Documentation must reflect this exactly.

**Capability authorization for mutation tools:**
- the existing capability vocabulary from RFC 0030 (`read`/`write`/`review`/`claim`/`apply`/`admin`/`recovery`) is already in the daemon DB; this RFC wires it into MCP `tools/call` + filters `tools/list`
- capability scope semantics: `repo_id`-scoped tokens cannot invoke write-paths against a different `repo_id`
- token lifecycle: issuance (admin-only), expiry, revocation, rotation; short-lived tokens for mutation
- audit row appended for every mutating `tools/call` request: client_id, repo_id when known, method, params hash, decision (allowed/denied), denial_reason from documented vocabulary, transport, audit chain link

**Default-deny gating:**
- unknown MCP methods return the standard unknown-method error
- methods registered without a declared capability requirement fail-closed (refused as `capability_missing`)
- the daemon never bypasses capability gating when an MCP client claims a "trusted" identity
- no global `--allow-mutations` flag in V2 — capability tokens are the only access path

**Prompt-injection mitigation:**
- tokens are the operator-controlled gate; a prompt-injected MCP cannot escalate beyond its token's capabilities
- short-lived tokens (e.g. `daemon.token.create --expires-in 1h`) for mutation; documented as the recommended posture
- operator UX for revoking a leaked token + the audit chain showing the attack timeline

**Cross-repo run lifecycle:**
- `run prepare` for a cross-repo workflow: daemon-mediated atomic write across daemon DB + per-repo SQLite, two-phase commit semantics inside the daemon
- daemon crash mid-prepare: the daemon-DB row is `preparing`; startup reconciliation completes or rolls back
- `run start` for a cross-repo run: enforces all participating repos still registered and accessible
- `run summary` aggregates across per-repo state stores
- `run cancel` cascades through all participating repos with daemon-coordinated transaction
- one participating repo unregistered mid-run: pause the run with a human checkpoint; refuse to advance until re-registration or cancellation

**Daemon-DB + repo-local-DB coordination:**
- `cross_repo_runs` table in daemon DB (id, workflow_id, repositories, state, started_at, completed_at)
- `cross_repo_run_id` recorded in each participating repo's `runs` row
- transaction ordering: daemon-DB write first, then per-repo SQLite writes inside the same daemon transaction; rollback on partial failure
- no cross-machine semantics in V2; cross-repo = cross-locally-registered-repo only

**Concrete touch points in `src/striatum/`:**
- `workflow.py` (validator for `repositories` block + cross-repo edges/cycles)
- `daemon_rpc/registry.py` (route map for cross-repo + MCP mutation methods)
- `daemon_rpc/server.py` (cross-repo run lifecycle handlers)
- `mcp.py` (capability-gated `tools/call`, per-token `tools/list` filter)
- `cli/mutations.py` if any CLI verbs need extension
- `daemon_pg/migrations.py` and `daemon_pg/sql/0003_cross_repo.sql` for the substrate addition
- existing `daemon_apply/`, `daemon_supervisor/`, `daemon_pg/audit.py` are leveraged, not redesigned

**Multi-repo / cross-repo END-TO-END integration tests are EXPLICITLY DEFERRED** to a follow-up RFC (`docs/TODO.md` Open item 19). Your design should specify the unit-level + mock-based coverage strategy without authoring a multi-repo daemon harness.

State what cannot be claimed even after this dogfood lands:
- cross-machine multi-tenant semantics (deferred to a separate hosted-mode RFC, never)
- malicious-local-root resistance (RFC 0031 threat model)
- atomic file-system mutations across two repos (workflow authors' responsibility; daemon coordinates ordering + verdicts, not file-system atomicity)

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim.

- Plain Markdown line, NO bold (`**`), NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.
- Correct: `author: designer-claude-opus-001`
- Wrong: `**Author:** ...`, `Author: ...`, etc.

The `handoff` kind does not require YAML front matter. Schema-bearing artifacts later in this dogfood do.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.
