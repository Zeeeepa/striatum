# Claude Code Design Prompt

Produce `docs/dogfood/037/design/claude_code/DESIGN.md`.

Design an implementation plan for RFC 0035 emphasizing the trust boundaries the e2e tests will exercise. The harness's value comes from exercising RFC 0032 V2's threat surfaces end-to-end at the test level; the unit + mock coverage that already shipped in dogfood-035 verified the contracts, but the harness verifies the integrated daemon-mediated flows.

Focus on:

**Threat surfaces the e2e tests exercise:**

- **Capability scope mismatch**: token scoped to repo A used against repo B → daemon refuses with `capability_missing` + audit row with documented denial vocabulary. The test asserts both the refusal AND the audit row.
- **Default-deny on unknown methods**: MCP `tools/call name="unknown.method"` → standard unknown-method error + audit row with `denial_reason=method_unknown`.
- **Audit chain integrity across allow/deny paths**: every mutating call produces an audit row; the hash chain links from row N to N+1; the test asserts chain continuity across a sequence of mixed allow/deny calls.
- **Daemon crash mid-prepare**: SIGKILL between daemon-DB write and per-repo SQLite writes → daemon restart's startup reconciliation observes `preparing` row + missing per-repo rows → rolls back daemon row to `aborted`. The test asserts the reconciliation state machine.
- **Daemon crash mid-start**: SIGKILL after prepare, before per-repo transitions complete → reconciliation completes or fails with a structured error; no orphans.
- **One participating repo unreachable mid-run**: simulate by chmod'ing the repo's `.striatum/` to 000 → daemon pauses the run with a human checkpoint; both daemon DB + reachable repos record the checkpoint. The test asserts the checkpoint exists.
- **Per-repo write-scope enforcement**: job targeting repo B publishing an artifact whose path resolves into repo A → publish-artifact refuses with `write_scope_violation`; the validator catches the same case at submit time. Tests cover both validator-time and runtime refusal.

**Per-test residue concerns:**

- Audit chain rows must be cleared cleanly between tests so chain assertions don't leak across test functions.
- Cross-repo run rows must be cleared.
- Capability tokens must be cleared.
- Per-repo SQLite state (runs, jobs, leases, queue_messages, etc.) must be cleared, but the schema-version row stays.
- Ephemeral PG database dropped on stop; scratch directories removed; Unix socket deleted.

The per-test reset path uses TRUNCATE rather than DROP+CREATE to amortize the schema-version metadata cost. The harness exposes `reset_daemon_db()` and explicit `register_all()` for the per-function escape hatch.

**Capability token issuance + lifecycle helpers:**

- `harness.issue_token(capability, repo_id=None, expires_in=3600)` issues a capability token through the daemon admin path. The test then uses the token via the MCP client helper.
- `harness.revoke_token(token)` invokes the documented revocation path.
- `harness.expire_token(token)` (test-helper-only) shifts the daemon's clock or directly updates the expiry row to force expiry for tests; documented as test-only.

**Adversarial test cases (must appear in the design):**

- Hostile chat/MCP client requesting `tools/list` with elevated args → returns the token's capability-filtered set, not the full set; test asserts the filtering.
- Prompt-injected MCP client claiming "trusted" identity via header manipulation → daemon ignores the identity claim and authorizes based on token capability.
- Replay an expired token after issuing a fresh one with the same scope → daemon refuses the expired one with `token_expired` even though the operator has the same scope active.
- Cross-repo token leak (token scoped to repo A used to read repo B's state) → refused with `capability_missing`; audit row records scope mismatch.
- Operator-confirmation gate bypass (the harness exposes a way to test this even without the chat UI present: simulate the operator-confirmation field missing on a write call) → server refuses; audit row records the missing confirmation.
- Audit chain tamper attempt via daemon API (role-enforced append-only refuses) → test asserts the daemon's audit-append code path refuses.

**State the harness does NOT introduce a parallel production-code path:**

- Same daemon binary
- Same SQLite + Postgres migrations
- Same RPC envelope (RFC 0030)
- Same capability vocabulary
- Same audit chain helper (RFC 0032 V2)
- Same MCP `tools/call` + `tools/list` code path

Anything the harness wraps for test ergonomics (token issuance helper, MCP client helper, audit-row inspection helper) is a thin wrapper over the production code paths, NOT a re-implementation.

**State what cannot be claimed even after the harness lands:**

- Cross-machine multi-tenant testing — out of scope per D083 (single-user single-machine).
- Windows daemon testing — out of scope per RFC 0030 V2.
- Malicious-local-root resistance — RFC 0031 threat model is the AI-guardrail framing.
- Performance/load testing — separate effort.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:` exactly. Correct: `author: designer-claude-opus-001`.

The `handoff` kind does not require YAML front matter.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.
